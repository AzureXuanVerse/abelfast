package gateway

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The inner login (LoginType.PLATFORM_INNER, sdkmgr.lua GetLoginType) is the
// client's own username / password panel, used instead of the SDK when the
// gateway is on 10.0.x.x or the device has server_config.txt. Its User has
// type 2, arg1 the username and arg2 the password (loginpanelview.lua).
const innerLoginType = 2

// Results of an inner login / registration, as the client handles them
// (LoginMediator GAME.USER_LOGIN_FAILED / GAME.USER_REGISTER_FAILED, the
// texts are error_message[n]):
const (
	// "Data Does Not Exist.": the login failure that switches to the
	// register panel.
	innerUnknownUser = 20
	// "Error Parameter.": back to the login panel.
	innerWrongPassword = 9
	// "Invalid Input.": the register failure, shown in a message box.
	innerRegisterRefused = 1
)

func passwordHash(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return sum[:]
}

// handleRegister answers UserRegisterCommand (CS_10001, the register panel):
// it creates the inner account, the client then logs into it with CS_10020.
// The panel only sends the username, password and mail_box are "".
func (gateway *Gateway) handleRegister(client *connection.Client, req *protobuf.CS_10001) error {
	username := req.GetAccount()
	if username == "" {
		return client.Send(&protobuf.SC_10002{Result: proto.Uint32(innerRegisterRefused)})
	}
	created, err := gateway.DB.CreateInnerAccount(context.Background(), db.CreateInnerAccountParams{
		Username:     username,
		PasswordHash: passwordHash(req.GetPassword()),
		MailBox:      req.GetMailBox(),
		AccountID:    accountID("inner\x00" + username),
		RegisteredAt: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	if created == 0 {
		logger.LogEvent("Gateway", "Register", fmt.Sprintf("username %q already taken (%s)", username, client.IP), logger.LOG_LEVEL_WARN)
		return client.Send(&protobuf.SC_10002{Result: proto.Uint32(innerRegisterRefused)})
	}
	logger.LogEvent("Gateway", "Register", fmt.Sprintf("inner account %q from %s", username, client.IP), logger.LOG_LEVEL_INFO)
	return client.Send(&protobuf.SC_10002{Result: proto.Uint32(0)})
}

// innerAccount checks an inner login (CS_10020 login_type 2): the account id
// of username, or the non-zero result to refuse it with.
func (gateway *Gateway) innerAccount(username, password string) (uint32, uint32, error) {
	account, err := gateway.DB.GetInnerAccount(context.Background(), username)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, innerUnknownUser, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if subtle.ConstantTimeCompare(account.PasswordHash, passwordHash(password)) != 1 {
		return 0, innerWrongPassword, nil
	}
	return account.AccountID, 0, nil
}
