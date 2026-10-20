package gateway

import (
	"context"
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

// handleAccountSearch answers AccountSearchCommand (CS_10026, the server
// list's "query role" button): the client asks every server in turn for the
// commander of its account and labels it lv.N, or "No Data" when user_id or
// level is 0.
func (gateway *Gateway) handleAccountSearch(client *connection.Client, req *protobuf.CS_10026) error {
	commander, err := gateway.DB.GetCommanderByAccount(context.Background(), req.GetAccountId())
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return client.Send(&protobuf.SC_10027{UserId: proto.Uint32(0), Level: proto.Uint32(0)})
	case err != nil:
		return err
	}
	return client.Send(&protobuf.SC_10027{
		UserId: proto.Uint32(commander.ID),
		Level:  proto.Uint32(commander.Level),
	})
}

// handleGatewaySwitch answers ServerInterconnectionCommand (CS_10802): a
// client switching to the other platform's servers (the CN switch_platform
// button) asks for that platform's gateway. Belfast has one gateway for every
// platform, sharing its address with the game server: the reply sends the
// client back here.
//
//belfast:static
func (gateway *Gateway) handleGatewaySwitch(client *connection.Client, req *protobuf.CS_10802) error {
	if len(gateway.Servers) == 0 {
		return fmt.Errorf("CS_10802: no server address to hand out")
	}
	server := gateway.Servers[0]
	return client.Send(&protobuf.SC_10803{
		GatewayIp:   proto.String(server.Host),
		GatewayPort: proto.Uint32(server.Port),
		ProxyIp:     proto.String(server.Host),
		ProxyPort:   proto.Uint32(server.Port),
	})
}

// handleCheaterMark records CheaterMarkCommand's report (CS_10994): the
// client flags its own commander when a battle fails its verification, then
// logs out (code 7, "verification failed") unless the reason is CC_TYPE_99 /
// CC_TYPE_100. The reply only acknowledges it.
func (gateway *Gateway) handleCheaterMark(client *connection.Client, req *protobuf.CS_10994) error {
	if client.CommanderID == 0 {
		return fmt.Errorf("CS_10994 before server login")
	}
	err := gateway.DB.CreateCheaterMark(context.Background(), db.CreateCheaterMarkParams{
		CommanderID: client.CommanderID,
		AccountID:   client.AccountID,
		Reason:      req.GetType(),
		MarkedAt:    time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	logger.LogEvent("Gateway", "CheaterMark", fmt.Sprintf("commander %d flagged itself, reason %d", client.CommanderID, req.GetType()), logger.LOG_LEVEL_WARN)
	return client.Send(&protobuf.SC_10995{Result: proto.Uint32(0)})
}
