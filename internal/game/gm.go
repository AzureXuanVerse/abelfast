package game

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleGMCommand answers SendCmdCommand, the GM console: world chat
// messages "$ <cmd> [args]" (NotificationMediator), and a few commands the
// client sends itself ("kick" when an NPC collection event ends, "world
// reset" + "kick" from WorldProxy.NetResetWorld). The client only prints
// msg; it reads "CMD:into" and "Result:ok" in it (luaex.lua's battle test),
// so replies are "CMD:<cmd> Result:<outcome>". Only "time" / "nowtime"
// (named in luaex.lua's reply handler) do anything here: the server clock.
// Everything else, the client's own commands included, is answered as
// unknown and changes nothing.
//
//belfast:static
func (game *Game) handleGMCommand(client *connection.Client, req *protobuf.CS_11100) error {
	cmd := req.GetCmd()
	switch cmd {
	case "time", "nowtime":
		now := time.Now()
		return client.Send(&protobuf.SC_11101{
			Result: result(nil),
			Msg:    proto.String(fmt.Sprintf("CMD:%s Result:ok %d %s", cmd, now.Unix(), now.In(gamedata.Zone()).Format(time.DateTime))),
		})
	}
	return client.Send(&protobuf.SC_11101{
		Result: result(errRefused),
		Msg:    proto.String(fmt.Sprintf("CMD:%s Result:unknown command", cmd)),
	})
}
