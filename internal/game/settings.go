package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// maxChatRoom is the highest world chat room ChangeChatRoomCommand allows.
const maxChatRoom = 99

// handleChangeChatRoom answers ChangeChatRoomCommand, moving the commander
// to another world chat room (1 to 99); it comes back in
// SC_11003.chat_room_id. Room 0 asks the server to pick one: ConnectionMgr
// sends it after every reconnect, and the commander stays in their room.
func (game *Game) handleChangeChatRoom(client *connection.Client, req *protobuf.CS_11401) error {
	ctx := context.Background()
	room := req.GetRoomId()
	if room > maxChatRoom {
		return client.Send(&protobuf.SC_11402{Result: result(errRefused), RoomId: proto.Uint32(room)})
	}
	if room == 0 {
		commander, err := game.DB.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		return client.Send(&protobuf.SC_11402{Result: result(nil), RoomId: proto.Uint32(commander.ChatRoomID)})
	}
	if err := game.DB.SetChatRoom(ctx, db.SetChatRoomParams{ChatRoomID: room, ID: client.CommanderID}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11402{Result: result(nil), RoomId: proto.Uint32(room)})
}

// handleClientReport takes CS_12299, sent by the C# side of the client after
// loading; there is no reply.
//
//belfast:static
func (game *Game) handleClientReport(client *connection.Client, req *protobuf.CS_12299) error {
	return nil
}
