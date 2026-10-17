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

// handleWorldState answers WorldConst.ReqWorldCheck (type 0: entering mail,
// compensation or Operation Siren scenes, or a ship status check while the
// world is still the SC_33114 base one) and ReqWorldForServer (type 1, after
// a reconnect) with the full Operation Siren state: open as in SC_33114, but
// never started. WorldProxy:NetFullUpdate reads world and count_info
// unconditionally, so both are sent with every required field zero; map_id 0
// means no active map.
func (game *Game) handleWorldState(client *connection.Client, req *protobuf.CS_33000) error {
	return client.Send(&protobuf.SC_33001{
		World: &protobuf.WORLDINFO{
			MapId:                    proto.Uint32(0),
			SubmarineState:           proto.Uint32(0),
			ActionPower:              proto.Uint32(0),
			ActionPowerExtra:         proto.Uint32(0),
			LastRecoverTimestamp:     proto.Uint32(0),
			ActionPowerFetchCount:    proto.Uint32(0),
			LastChangeGroupTimestamp: proto.Uint32(0),
			EnterMapId:               proto.Uint32(0),
		},
		IsWorldOpen: proto.Uint32(1),
		Camp:        proto.Uint32(0),
		CountInfo: &protobuf.COUNTINFO{
			StepCount:     proto.Uint32(0),
			TreasureCount: proto.Uint32(0),
			TaskProgress:  proto.Uint32(0),
			ActivateCount: proto.Uint32(0),
		},
	})
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
