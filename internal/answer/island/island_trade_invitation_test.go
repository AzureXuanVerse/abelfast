package island

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandTradeInvitationSuccessPersistsAndPushes(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	inviter := setupHandlerCommander(t)
	onlineID := inviter.Commander.CommanderID + 1
	offlineID := inviter.Commander.CommanderID + 2

	if err := orm.CreateCommanderRoot(onlineID, onlineID, "online target", 0, 0); err != nil {
		t.Fatalf("create online target commander: %v", err)
	}
	if err := orm.CreateCommanderRoot(offlineID, offlineID, "offline target", 0, 0); err != nil {
		t.Fatalf("create offline target commander: %v", err)
	}
	onlineCommander := orm.Commander{CommanderID: onlineID}
	if err := onlineCommander.Load(); err != nil {
		t.Fatalf("load online target commander: %v", err)
	}
	onlineTarget := &connection.Client{Commander: &onlineCommander}
	onlineTarget.Hash = inviter.Hash + 1
	onlineTarget.Server = inviter.Server
	inviter.Server.AddClient(inviter)
	inviter.Server.AddClient(onlineTarget)

	friendList := []uint32{
		onlineID,
		offlineID,
		onlineID,
		0,
		inviter.Commander.CommanderID,
		4294967295,
	}
	payload, err := proto.Marshal(&protobuf.CS_21245{
		FriendList: friendList,
		MapId:      proto.Uint32(1003),
		Price:      proto.Uint32(777),
	})
	if err != nil {
		t.Fatalf("marshal invitation payload: %v", err)
	}

	if _, _, err := IslandTradeInvitation(&payload, inviter); err != nil {
		t.Fatalf("trade invitation handler failed: %v", err)
	}

	var ack protobuf.SC_21246
	decodePacketAt(t, inviter, 0, 21246, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected success ack, got %d", ack.GetResult())
	}

	var push protobuf.SC_21247
	decodePacketAt(t, onlineTarget, 0, 21247, &push)
	if push.GetIslandId() != inviter.Commander.CommanderID {
		t.Fatalf("expected inviter id %d in push, got %d", inviter.Commander.CommanderID, push.GetIslandId())
	}
	if push.GetMapId() != 1003 || push.GetPrice() != 777 {
		t.Fatalf("unexpected push payload map_id=%d price=%d", push.GetMapId(), push.GetPrice())
	}

	state, err := orm.GetCommanderIslandTradeInviteState(inviter.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load trade invite state: %v", err)
	}
	if len(state.InvitedCommanderIDs) != 2 {
		t.Fatalf("expected 2 invited commander ids, got %v", state.InvitedCommanderIDs)
	}
	if state.InvitedCommanderIDs[0] != onlineID || state.InvitedCommanderIDs[1] != offlineID {
		t.Fatalf("unexpected persisted invite ids: %v", state.InvitedCommanderIDs)
	}

	inviter.Buffer.Reset()
	getDataPayload, _ := proto.Marshal(&protobuf.CS_21200{IslandId: proto.Uint32(inviter.Commander.CommanderID)})
	if _, _, err := IslandGetData(&getDataPayload, inviter); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}
	var getDataResp protobuf.SC_21201
	decodePacketAt(t, inviter, 0, 21201, &getDataResp)
	invites := getDataResp.GetIsland().GetPublicData().GetTreasure().GetInviteList()
	if len(invites) != 2 {
		t.Fatalf("expected 2 treasure invite ids, got %v", invites)
	}
	if invites[0] != onlineID || invites[1] != offlineID {
		t.Fatalf("unexpected treasure invite list: %v", invites)
	}
}

func TestIslandTradeInvitationRejectsInvalidTargets(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	inviter := setupHandlerCommander(t)

	payload, err := proto.Marshal(&protobuf.CS_21245{
		FriendList: []uint32{0, inviter.Commander.CommanderID},
		MapId:      proto.Uint32(1003),
		Price:      proto.Uint32(555),
	})
	if err != nil {
		t.Fatalf("marshal invitation payload: %v", err)
	}

	if _, _, err := IslandTradeInvitation(&payload, inviter); err != nil {
		t.Fatalf("trade invitation handler failed: %v", err)
	}

	var ack protobuf.SC_21246
	decodePacketAt(t, inviter, 0, 21246, &ack)
	if ack.GetResult() == 0 {
		t.Fatalf("expected non-zero ack for invalid target list")
	}

	_, err = orm.GetCommanderIslandTradeInviteState(inviter.Commander.CommanderID)
	if err == nil {
		t.Fatalf("expected no persisted invite state on validation failure")
	}
	if !db.IsNotFound(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
}
