package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleWorldChat takes SendMsgCommand's world chat message (CS_50102, type
// 1, already filtered by the client's wordVer), which has no reply: it goes
// to every commander online in the sender's chat room (SC_11003.chat_room_id,
// CS_11401), the sender included since ChatProxy only shows what SC_50101
// brings. Shaped as captured: the sender's PLAYER_INFO_P50, type 1, the
// text. Each client hides its blacklisted senders itself (ChatProxy).
func (game *Game) handleWorldChat(client *connection.Client, req *protobuf.CS_50102) error {
	ctx := context.Background()
	if req.GetContent() == "" {
		return nil
	}
	sender, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	msg := &protobuf.SC_50101{
		Player:  playerInfoP50(sender),
		Type:    proto.Uint32(req.GetType()),
		Content: proto.String(req.GetContent()),
	}
	for _, id := range game.onlineCommanders() {
		reader, err := game.DB.GetCommander(ctx, id)
		if err != nil {
			return err
		}
		if reader.ChatRoomID == sender.ChatRoomID {
			game.deliver(client, id, msg)
		}
	}
	return nil
}

// handleReportPlayer answers InformCommand (CS_50111), reporting a player
// from their profile or a chat message (the client sends it from level 20):
// it is only logged, the reply is the result alone. Built from the Lua
// alone.
//
//belfast:static
func (game *Game) handleReportPlayer(client *connection.Client, req *protobuf.CS_50111) error {
	logger.WithFields("Chat",
		logger.FieldValue("commander", client.CommanderID),
		logger.FieldValue("reported", req.GetId()),
		logger.FieldValue("reason", req.GetInfo()),
		logger.FieldValue("content", req.GetContent()),
	).Info("player reported")
	return client.Send(&protobuf.SC_50112{Result: result(nil)})
}
