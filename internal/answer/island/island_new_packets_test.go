package island

import (
	"encoding/json"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandExitAndQueueRelease(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	leader := setupHandlerCommander(t)
	follower := setupHandlerCommander(t)

	enterPayload, _ := proto.Marshal(&protobuf.CS_21202{IslandId: proto.Uint32(9901)})
	if _, _, err := IslandEnter(&enterPayload, leader); err != nil {
		t.Fatalf("leader enter failed: %v", err)
	}
	if _, _, err := IslandEnter(&enterPayload, follower); err != nil {
		t.Fatalf("follower enter failed: %v", err)
	}

	leader.Buffer.Reset()
	exitPayload, _ := proto.Marshal(&protobuf.CS_21204{IslandId: proto.Uint32(9901)})
	if _, _, err := IslandExit(&exitPayload, leader); err != nil {
		t.Fatalf("island exit failed: %v", err)
	}
	var exitResponse protobuf.SC_21205
	decodePacketAt(t, leader, 0, 21205, &exitResponse)
	if exitResponse.GetResult() != 0 {
		t.Fatalf("expected exit success, got %d", exitResponse.GetResult())
	}

	follower.Buffer.Reset()
	pollPayload, _ := proto.Marshal(&protobuf.CS_21208{IslandId: proto.Uint32(9901)})
	if _, _, err := HandleIslandQueuePoll(&pollPayload, follower); err != nil {
		t.Fatalf("queue poll failed: %v", err)
	}
	var pollResponse protobuf.SC_21203
	decodePacketAt(t, follower, 0, 21203, &pollResponse)
	if pollResponse.GetResult() != 0 {
		t.Fatalf("expected queued follower to enter after exit, got %d", pollResponse.GetResult())
	}
}

func TestIslandReleaseSessionClearsObjectSlotOwnership(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	owner := setupHandlerCommander(t)
	peer := setupHandlerCommander(t)

	globalIslandRuntimeState.setSessionForTest(owner.Commander.CommanderID, 9902)
	globalIslandRuntimeState.seedIslandObjects(9902, []islandObjectTemplate{{ID: 3001, Type: 1, SlotIDs: []uint32{1}, Status: 1}})

	ownedObject, ok := globalIslandRuntimeState.applyIslandControl(owner.Commander.CommanderID, 9902, 1, 3001, 1, 1, 1)
	if !ok || ownedObject.GetSlots()[0].GetOwnerId() != owner.Commander.CommanderID {
		t.Fatalf("expected owner to acquire object slot")
	}

	if !globalIslandRuntimeState.releaseSession(owner.Commander.CommanderID, 9902) {
		t.Fatalf("expected release session to succeed")
	}

	peerObject, ok := globalIslandRuntimeState.applyIslandControl(peer.Commander.CommanderID, 9902, 1, 3001, 1, 1, 1)
	if !ok || peerObject.GetSlots()[0].GetOwnerId() != peer.Commander.CommanderID {
		t.Fatalf("expected slot ownership to clear on release")
	}
}

func TestIslandExitFailureCases(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	payload, _ := proto.Marshal(&protobuf.CS_21204{IslandId: proto.Uint32(1201)})
	if _, _, err := IslandExit(&payload, client); err != nil {
		t.Fatalf("exit without session should still ack: %v", err)
	}
	var noSession protobuf.SC_21205
	decodePacketAt(t, client, 0, 21205, &noSession)
	if noSession.GetResult() == 0 {
		t.Fatalf("expected non-zero result without session")
	}

	globalIslandRuntimeState.setSessionForTest(client.Commander.CommanderID, 1202)
	client.Buffer.Reset()
	if _, _, err := IslandExit(&payload, client); err != nil {
		t.Fatalf("mismatch exit should ack: %v", err)
	}
	var mismatch protobuf.SC_21205
	decodePacketAt(t, client, 0, 21205, &mismatch)
	if mismatch.GetResult() == 0 {
		t.Fatalf("expected non-zero result for island mismatch")
	}
	if !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, 1202) {
		t.Fatalf("expected mismatched exit to preserve active session")
	}

	bad := []byte{0xff, 0x00}
	_, packetID, err := IslandExit(&bad, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if packetID != 21205 {
		t.Fatalf("expected packet id 21205 on decode failure, got %d", packetID)
	}
}

func TestIslandEnterMapAndSyncControl(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	sender := setupHandlerCommander(t)
	peer := setupHandlerCommander(t)
	shareServer(t, sender, peer)

	globalIslandRuntimeState.setSessionForTest(sender.Commander.CommanderID, 8801)
	globalIslandRuntimeState.setSessionForTest(peer.Commander.CommanderID, 8801)
	seedIslandMapConfig(t)

	enterMapPayload, _ := proto.Marshal(&protobuf.CS_21213{IslandId: proto.Uint32(8801), MapId: proto.Uint32(1001)})
	if _, _, err := IslandEnterMap(&enterMapPayload, sender); err != nil {
		t.Fatalf("map enter failed: %v", err)
	}
	var mapResponse protobuf.SC_21214
	decodePacketAt(t, sender, 0, 21214, &mapResponse)
	if mapResponse.GetResult() != 0 {
		t.Fatalf("expected map enter success, got %d", mapResponse.GetResult())
	}
	if len(mapResponse.GetObjectList()) != 1 || len(mapResponse.GetGatherList()) != 1 || len(mapResponse.GetFragmentList()) != 1 {
		t.Fatalf("expected seeded object/gather/fragment lists")
	}

	sender.Buffer.Reset()
	invalidMapPayload, _ := proto.Marshal(&protobuf.CS_21213{IslandId: proto.Uint32(8801), MapId: proto.Uint32(199999)})
	if _, _, err := IslandEnterMap(&invalidMapPayload, sender); err != nil {
		t.Fatalf("invalid map request should still ack: %v", err)
	}
	var invalidMapResponse protobuf.SC_21214
	decodePacketAt(t, sender, 0, 21214, &invalidMapResponse)
	if invalidMapResponse.GetResult() == 0 {
		t.Fatalf("expected invalid map request to fail")
	}

	sender.Buffer.Reset()
	peer.Buffer.Reset()
	controlPayload, _ := proto.Marshal(&protobuf.CS_21209{
		IslandId: proto.Uint32(8801),
		ObjId:    proto.Uint32(3001),
		SlotId:   proto.Uint32(1),
		Op:       proto.Uint32(1),
		Status:   proto.Uint32(7),
		Type:     proto.Uint32(1),
	})
	if _, _, err := IslandSyncControl(&controlPayload, sender); err != nil {
		t.Fatalf("sync control acquire failed: %v", err)
	}
	var senderPush protobuf.SC_21207
	offset := decodePacketAt(t, sender, 0, 21207, &senderPush)
	if senderPush.GetObjectList()[0].GetSlots()[0].GetOwnerId() != sender.Commander.CommanderID {
		t.Fatalf("expected sender to own slot after acquire")
	}
	var controlAck protobuf.SC_21210
	decodePacketAt(t, sender, offset, 21210, &controlAck)
	if controlAck.GetResult() != 0 {
		t.Fatalf("expected control success, got %d", controlAck.GetResult())
	}
	var peerPush protobuf.SC_21207
	decodePacketAt(t, peer, 0, 21207, &peerPush)

	peer.Buffer.Reset()
	conflictPayload, _ := proto.Marshal(&protobuf.CS_21209{
		IslandId: proto.Uint32(8801),
		ObjId:    proto.Uint32(3001),
		SlotId:   proto.Uint32(1),
		Op:       proto.Uint32(1),
		Status:   proto.Uint32(7),
		Type:     proto.Uint32(1),
	})
	if _, _, err := IslandSyncControl(&conflictPayload, peer); err != nil {
		t.Fatalf("sync control conflict request failed: %v", err)
	}
	var conflictAck protobuf.SC_21210
	decodePacketAt(t, peer, 0, 21210, &conflictAck)
	if conflictAck.GetResult() == 0 {
		t.Fatalf("expected conflict acquire to fail")
	}

	invalidPayload, _ := proto.Marshal(&protobuf.CS_21209{
		IslandId: proto.Uint32(8801),
		ObjId:    proto.Uint32(999999),
		SlotId:   proto.Uint32(1),
		Op:       proto.Uint32(1),
		Status:   proto.Uint32(1),
		Type:     proto.Uint32(1),
	})
	sender.Buffer.Reset()
	if _, _, err := IslandSyncControl(&invalidPayload, sender); err != nil {
		t.Fatalf("sync control invalid tuple request failed: %v", err)
	}
	var invalidAck protobuf.SC_21210
	decodePacketAt(t, sender, 0, 21210, &invalidAck)
	if invalidAck.GetResult() == 0 {
		t.Fatalf("expected invalid tuple to fail")
	}

	sender.Buffer.Reset()
	globalIslandRuntimeState.clearSessionForTest(sender.Commander.CommanderID)
	if _, _, err := IslandEnterMap(&enterMapPayload, sender); err != nil {
		t.Fatalf("session mismatch map request should still ack: %v", err)
	}
	var noSessionMap protobuf.SC_21214
	decodePacketAt(t, sender, 0, 21214, &noSessionMap)
	if noSessionMap.GetResult() == 0 {
		t.Fatalf("expected map enter failure without matching session")
	}
}

func TestIslandSyncDataAndAnimationBroadcast(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	sender := setupHandlerCommander(t)
	peer := setupHandlerCommander(t)
	outsider := setupHandlerCommander(t)
	shareServer(t, sender, peer, outsider)

	globalIslandRuntimeState.setSessionForTest(sender.Commander.CommanderID, 8701)
	globalIslandRuntimeState.setSessionForTest(peer.Commander.CommanderID, 8701)
	globalIslandRuntimeState.setSessionForTest(outsider.Commander.CommanderID, 8702)

	syncPayload, _ := proto.Marshal(&protobuf.CS_21211{
		IslandId: proto.Uint32(8701),
		SyncObList: []*protobuf.PB_SYNC_OBJECT{{
			Id:     proto.Uint32(42),
			Status: []int32{1, 2},
		}},
	})
	if _, _, err := IslandSyncData(&syncPayload, sender); err != nil {
		t.Fatalf("sync data failed: %v", err)
	}
	if sender.Buffer.Len() != 0 {
		t.Fatalf("expected sender to not receive sync echo")
	}
	var syncPush protobuf.SC_21212
	decodePacketAt(t, peer, 0, 21212, &syncPush)
	if len(syncPush.GetSyncObList()) != 1 || syncPush.GetSyncObList()[0].GetId() != 42 {
		t.Fatalf("unexpected sync push payload")
	}
	if outsider.Buffer.Len() != 0 {
		t.Fatalf("expected outsider to not receive island sync data")
	}

	peer.Buffer.Reset()
	oversized := make([]*protobuf.PB_SYNC_OBJECT, 0, islandSyncObjectBatchLimit+1)
	for i := 0; i <= islandSyncObjectBatchLimit; i++ {
		oversized = append(oversized, &protobuf.PB_SYNC_OBJECT{Id: proto.Uint32(uint32(i + 1))})
	}
	oversizedPayload, _ := proto.Marshal(&protobuf.CS_21211{IslandId: proto.Uint32(8701), SyncObList: oversized})
	if _, _, err := IslandSyncData(&oversizedPayload, sender); err != nil {
		t.Fatalf("oversized sync payload should not error: %v", err)
	}
	if peer.Buffer.Len() != 0 {
		t.Fatalf("expected oversized sync payload to be dropped")
	}

	peer.Buffer.Reset()
	sender.Buffer.Reset()
	animationPayload, _ := proto.Marshal(&protobuf.CS_21700{
		IslandId: proto.Uint32(8701),
		TargetId: proto.Uint32(5001),
		ActionId: proto.Uint32(33),
	})
	if _, _, err := IslandAnimationOp(&animationPayload, sender); err != nil {
		t.Fatalf("animation op failed: %v", err)
	}
	var senderAnimation protobuf.SC_21701
	decodePacketAt(t, sender, 0, 21701, &senderAnimation)
	if senderAnimation.GetPlayerId() != sender.Commander.CommanderID {
		t.Fatalf("expected sender id in animation push")
	}
	var peerAnimation protobuf.SC_21701
	decodePacketAt(t, peer, 0, 21701, &peerAnimation)
	if peerAnimation.GetActionId() != 33 {
		t.Fatalf("expected action id 33, got %d", peerAnimation.GetActionId())
	}

	globalIslandRuntimeState.clearSessionForTest(sender.Commander.CommanderID)
	peer.Buffer.Reset()
	sender.Buffer.Reset()
	if _, _, err := IslandAnimationOp(&animationPayload, sender); err != nil {
		t.Fatalf("animation op without session should not error: %v", err)
	}
	if peer.Buffer.Len() != 0 || sender.Buffer.Len() != 0 {
		t.Fatalf("expected no animation broadcast without matching session")
	}

	bad := []byte{0x01, 0x02}
	if _, _, err := IslandAnimationOp(&bad, sender); err == nil {
		t.Fatalf("expected decode error for malformed animation packet")
	}
}

func TestIslandRecordLastPosition(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	client := setupHandlerCommander(t)

	recordPayload, _ := proto.Marshal(&protobuf.CS_21229{
		IslandId: proto.Uint32(client.Commander.CommanderID),
		PlayerPosition: &protobuf.PB_PLAYER_POS_RECORD{
			MapId: proto.Uint32(2002),
			Position: &protobuf.PB_VECTOR3{
				X: proto.Float32(1.25),
				Y: proto.Float32(2.5),
				Z: proto.Float32(3.75),
			},
			Rotation: &protobuf.PB_VECTOR3{
				X: proto.Float32(4.25),
				Y: proto.Float32(5.5),
				Z: proto.Float32(6.75),
			},
		},
	})
	if _, _, err := HandleIslandRecordLastPosition(&recordPayload, client); err != nil {
		t.Fatalf("record last position failed: %v", err)
	}

	getPayload, _ := proto.Marshal(&protobuf.CS_21200{IslandId: proto.Uint32(client.Commander.CommanderID)})
	if _, _, err := IslandGetData(&getPayload, client); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}
	var response protobuf.SC_21201
	decodePacketAt(t, client, 0, 21201, &response)
	if response.GetPlayerPosition().GetMapId() != 2002 {
		t.Fatalf("expected persisted map id, got %d", response.GetPlayerPosition().GetMapId())
	}

	mismatchPayload, _ := proto.Marshal(&protobuf.CS_21229{
		IslandId: proto.Uint32(client.Commander.CommanderID + 1000),
		PlayerPosition: &protobuf.PB_PLAYER_POS_RECORD{
			MapId:    proto.Uint32(9999),
			Position: &protobuf.PB_VECTOR3{X: proto.Float32(9), Y: proto.Float32(9), Z: proto.Float32(9)},
			Rotation: &protobuf.PB_VECTOR3{X: proto.Float32(9), Y: proto.Float32(9), Z: proto.Float32(9)},
		},
	})
	if _, _, err := HandleIslandRecordLastPosition(&mismatchPayload, client); err != nil {
		t.Fatalf("mismatch write should be ignored without error: %v", err)
	}

	bad := []byte{0x01, 0x02}
	if _, _, err := HandleIslandRecordLastPosition(&bad, client); err == nil {
		t.Fatalf("expected decode error on malformed record payload")
	}
}

func shareServer(t *testing.T, clients ...*connection.Client) {
	t.Helper()
	if len(clients) == 0 {
		return
	}
	server := clients[0].Server
	server.AddClient(clients[0])
	for i := 1; i < len(clients); i++ {
		clients[i].Hash = clients[0].Hash + uint32(i)
		clients[i].Server = server
		server.AddClient(clients[i])
	}
}

func seedIslandMapConfig(t *testing.T) {
	t.Helper()

	seedIslandConfigEntry(t, islandMapCategory, "1001", map[string]any{"id": 1001})
	seedIslandConfigEntry(t, islandWorldObjectsCategory, "3001", map[string]any{"id": 3001, "map_id": 1001, "type": 1, "slot_count": 1})
	seedIslandConfigEntry(t, islandWildGatherCategory, "4001", map[string]any{"id": 4001, "map_id": 1001, "pos": 3001, "state": 0, "mark": 0})
	seedIslandConfigEntry(t, islandFragmentCategory, "5001", map[string]any{"id": 5001, "map_id": 1001, "pos": 3001, "mark": 0})
	seedIslandConfigEntry(t, islandNpcCategory, "6001", map[string]any{"id": 6001, "map_id": 1001, "object_id": 3001})
}

func seedIslandConfigEntry(t *testing.T, category string, key string, value map[string]any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal config value: %v", err)
	}
	if err := orm.UpsertConfigEntry(category, key, payload); err != nil {
		t.Fatalf("upsert config entry %s/%s: %v", category, key, err)
	}
}
