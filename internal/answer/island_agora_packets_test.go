package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedIslandBuildExpansionConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, islandSetCategory, "island_build_expansion", `{"key":"island_build_expansion","key_value_varchar":[[1,[41,2001,2],700],[2,[41,2001,3],1000]]}`)
}

func TestIslandUpgradeAgoraSuccessAndGuards(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandInventory{})

	seedIslandBuildExpansionConfig(t)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, AgoraLevel: 1}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(2001), int64(5))

	payload := protobuf.CS_21305{Type: proto.Uint32(0)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandUpgradeAgora(&buffer, client); err != nil {
		t.Fatalf("IslandUpgradeAgora failed: %v", err)
	}

	var response protobuf.SC_21306
	decodePacketAt(t, client, 0, 21306, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.AgoraLevel != 2 {
		t.Fatalf("expected agora level 2, got %d", snapshot.AgoraLevel)
	}
	remaining := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(2001))
	if remaining != 3 {
		t.Fatalf("expected item cost consumed, got %d", remaining)
	}

	client.Buffer.Reset()
	invalid := protobuf.CS_21305{Type: proto.Uint32(99)}
	invalidBuffer, _ := proto.Marshal(&invalid)
	if _, _, err := IslandUpgradeAgora(&invalidBuffer, client); err != nil {
		t.Fatalf("IslandUpgradeAgora invalid type failed unexpectedly: %v", err)
	}
	decodePacketAt(t, client, 0, 21306, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid type failure")
	}

	client.Buffer.Reset()
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, AgoraLevel: 3}); err != nil {
		t.Fatalf("seed max snapshot: %v", err)
	}
	maxBuffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandUpgradeAgora(&maxBuffer, client); err != nil {
		t.Fatalf("IslandUpgradeAgora max level failed unexpectedly: %v", err)
	}
	decodePacketAt(t, client, 0, 21306, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected max level failure")
	}
}

func TestIslandUpgradeAgoraInsufficientItems(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandInventory{})

	seedIslandBuildExpansionConfig(t)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, AgoraLevel: 1}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(2001), int64(1))

	payload := protobuf.CS_21305{Type: proto.Uint32(0)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandUpgradeAgora(&buffer, client); err != nil {
		t.Fatalf("IslandUpgradeAgora failed unexpectedly: %v", err)
	}

	var response protobuf.SC_21306
	decodePacketAt(t, client, 0, 21306, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected insufficient items failure")
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.AgoraLevel != 1 {
		t.Fatalf("expected unchanged agora level, got %d", snapshot.AgoraLevel)
	}
}

func TestIslandSaveAgoraPlacementPersistsAndBroadcasts(t *testing.T) {
	globalIslandRuntimeState.resetForTest()
	owner := setupHandlerCommander(t)
	watcher := setupHandlerCommander(t)
	clearTable(t, &orm.IslandAgoraPlacement{})

	owner.Server.AddClient(owner)
	watcher.Hash = owner.Hash + 1
	watcher.Server = owner.Server
	watcher.Server.AddClient(watcher)

	globalIslandRuntimeState.setSessionForTest(owner.Commander.CommanderID, owner.Commander.CommanderID)
	globalIslandRuntimeState.setSessionForTest(watcher.Commander.CommanderID, owner.Commander.CommanderID)

	payload := protobuf.CS_21307{UpdateData: &protobuf.PB_PLACEMENT_DATA{
		PlacedList: []*protobuf.PB_FURNITURE_DATA{{Id: proto.Uint32(501), X: proto.Int32(11), Y: proto.Int32(22), Dir: proto.Uint32(2)}},
		FloorData:  []uint32{3, 4},
		TileData:   []uint32{5},
	}}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandSaveAgoraPlacement(&buffer, owner); err != nil {
		t.Fatalf("IslandSaveAgoraPlacement failed: %v", err)
	}

	var ack protobuf.SC_21308
	decodePacketAt(t, owner, 0, 21308, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected placement save success, got %d", ack.GetResult())
	}

	var push protobuf.SC_21309
	decodePacketAt(t, watcher, 0, 21309, &push)
	if push.GetIslandId() != owner.Commander.CommanderID || len(push.GetUpdateData().GetPlacedList()) != 1 {
		t.Fatalf("unexpected broadcast payload: %+v", push)
	}

	stored, err := orm.GetIslandAgoraPlacement(owner.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load placement: %v", err)
	}
	if len(stored.PlacedData) == 0 {
		t.Fatalf("expected persisted placement payload")
	}

	getDataPayload := protobuf.CS_21200{IslandId: proto.Uint32(owner.Commander.CommanderID)}
	getDataBuffer, _ := proto.Marshal(&getDataPayload)
	owner.Buffer.Reset()
	if _, _, err := IslandGetData(&getDataBuffer, owner); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}
	var getDataResponse protobuf.SC_21201
	decodePacketAt(t, owner, 0, 21201, &getDataResponse)
	if len(getDataResponse.GetIsland().GetPublicData().GetPlacedData().GetPlacedList()) != 1 {
		t.Fatalf("expected persisted placement in island data")
	}
}

func TestIslandSaveAgoraPlacementRejectsMissingData(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandAgoraPlacement{})

	malformed := []byte{0x08, 0x01}
	_, packetID, err := IslandSaveAgoraPlacement(&malformed, client)
	if err == nil {
		t.Fatalf("expected unmarshal error for missing required update_data")
	}
	if packetID != 21308 {
		t.Fatalf("expected packet id 21308, got %d", packetID)
	}
}
