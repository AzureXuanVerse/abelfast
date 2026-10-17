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

// handleSecondaryPassword answers FetchSecondaryPasswordCommand, which keeps
// a loading overlay up until it gets a reply: no secondary password set, no
// failed attempts.
func (game *Game) handleSecondaryPassword(client *connection.Client, req *protobuf.CS_11603) error {
	return client.Send(&protobuf.SC_11604{
		FailCount: proto.Uint32(0),
		FailCd:    proto.Uint32(0),
		Notice:    proto.String(""),
	})
}

// handleChangeChatRoom answers ChangeChatRoomCommand, moving the commander
// to another world chat room (1 to 99); it comes back in
// SC_11003.chat_room_id.
func (game *Game) handleChangeChatRoom(client *connection.Client, req *protobuf.CS_11401) error {
	room := req.GetRoomId()
	if room < 1 || room > maxChatRoom {
		return client.Send(&protobuf.SC_11402{Result: result(errRefused), RoomId: proto.Uint32(room)})
	}
	if err := game.DB.SetChatRoom(context.Background(), db.SetChatRoomParams{ChatRoomID: room, ID: client.CommanderID}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11402{Result: result(nil), RoomId: proto.Uint32(room)})
}

// handleClientReport takes CS_12299, sent by the C# side of the client after
// loading; there is no reply.
func (game *Game) handleClientReport(client *connection.Client, req *protobuf.CS_12299) error {
	return nil
}

// handleCommanderHome answers GetCommanderHomeCommand, the cat lodge: level
// 1 with its first slot open (op_flag 7) and empty (captured; there are no
// cat commanders yet).
func (game *Game) handleCommanderHome(client *connection.Client, req *protobuf.CS_25026) error {
	return client.Send(&protobuf.SC_25027{
		Level: proto.Uint32(1),
		Slots: []*protobuf.COMMANDERHOMESLOT{{
			Id:     proto.Uint32(1),
			OpFlag: proto.Uint32(7),
			Style:  proto.Uint32(1),
		}},
	})
}

// handleWorldBoss answers GetWorldBossCommand (Operation Siren's world
// boss): no boss summoned or fought, own boss level 1.
func (game *Game) handleWorldBoss(client *connection.Client, req *protobuf.CS_34501) error {
	return client.Send(&protobuf.SC_34502{SelfBossLv: proto.Uint32(1)})
}

// handleMetaPTData answers MetaPTDataRequestCommand, the META ship points of
// the groups the client asks for: none earned.
func (game *Game) handleMetaPTData(client *connection.Client, req *protobuf.CS_34001) error {
	reply := &protobuf.SC_34002{}
	for _, group := range req.GetGroupId() {
		reply.MetaShipList = append(reply.MetaShipList, &protobuf.META_SHIP_INFO{GroupId: proto.Uint32(group)})
	}
	return client.Send(reply)
}
