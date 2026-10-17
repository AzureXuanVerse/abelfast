package gateway

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// serverLoginRejected is any non-zero result the client doesn't special-case
// (serverlogincommand.lua): it shows the generic server login failure.
const serverLoginRejected = 1

// handleServerLogin is the game server side of the login: the client presents
// the ticket from SC_10021 (or the one from a previous SC_10023 when it
// reconnects) and gets a fresh one back.
//
// user_id 0 means the account has no commander on this server yet, the client
// then plays the prologue and goes to the create player scene (CS_10024).
func (gateway *Gateway) handleServerLogin(client *connection.Client, req *protobuf.CS_10022) error {
	account, ok := gateway.Account(req.GetServerTicket())
	if !ok || account != req.GetAccountId() {
		logger.LogEvent("Gateway", "ServerLogin", fmt.Sprintf("rejected account %d from %s: unknown ticket", req.GetAccountId(), client.IP), logger.LOG_LEVEL_WARN)
		return client.Send(&protobuf.SC_10023{
			Result:       proto.Uint32(serverLoginRejected),
			UserId:       proto.Uint32(0),
			ServerTicket: proto.String(""),
		})
	}
	ticket, err := gateway.issueTicket(account)
	if err != nil {
		return err
	}
	logger.LogEvent("Gateway", "ServerLogin", fmt.Sprintf("account %d on server %d from %s", account, req.GetServerid(), client.IP), logger.LOG_LEVEL_INFO)
	return client.Send(&protobuf.SC_10023{
		Result:       proto.Uint32(0),
		UserId:       proto.Uint32(0),
		ServerTicket: proto.String(ticket),
	})
}

// handleHeartbeat answers the client's ping (ConnectionMgr:resetHBTimer),
// sent on every connection; the client only times the reply, state is unused.
// Unanswered, it resends forever and eventually reconnects.
func (gateway *Gateway) handleHeartbeat(client *connection.Client, req *protobuf.CS_10100) error {
	return client.Send(&protobuf.SC_10101{State: proto.Uint32(0)})
}
