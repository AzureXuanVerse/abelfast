package island

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupIslandProgressionTest(t *testing.T) uint32 {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandShip{})
	clearTable(t, &orm.IslandInventory{})
	return client.Commander.CommanderID
}

func TestHandleIslandShipBreakoutSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandShip{})
	clearTable(t, &orm.IslandInventory{})
	seedConfigEntry(t, islandShipBreakoutConfigCategory, "1051700", `{"id":1051700,"upgrade_level":[10,4],"upgrade_material":[[[100201,1]],[[100202,2]]]}`)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1051700, Level: 10, BreakLv: 1, CanFollow: true}); err != nil {
		t.Fatalf("seed island ship: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(100201), int64(1))

	request := protobuf.CS_21601{ShipId: proto.Uint32(1051700)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := HandleIslandShipBreakout(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21602
	decodeResponse(t, client, &response)
	if response.GetResult() != islandShipBreakoutResultSuccess {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	ship, err := orm.GetIslandShip(client.Commander.CommanderID, 1051700)
	if err != nil {
		t.Fatalf("get island ship: %v", err)
	}
	if ship.BreakLv != 2 {
		t.Fatalf("expected break level 2, got %d", ship.BreakLv)
	}
	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 100201)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	if item.Count != 0 {
		t.Fatalf("expected item consumed, got %d", item.Count)
	}
}

func TestHandleIslandShipBreakoutInsufficientItemsDoesNotMutate(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandShip{})
	clearTable(t, &orm.IslandInventory{})
	seedConfigEntry(t, islandShipBreakoutConfigCategory, "1051700", `{"id":1051700,"upgrade_level":[10,4],"upgrade_material":[[[100201,1],[100202,1]]]}`)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1051700, Level: 10, BreakLv: 1, CanFollow: true}); err != nil {
		t.Fatalf("seed island ship: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(100201), int64(1))

	request := protobuf.CS_21601{ShipId: proto.Uint32(1051700)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := HandleIslandShipBreakout(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21602
	decodeResponse(t, client, &response)
	if response.GetResult() == islandShipBreakoutResultSuccess {
		t.Fatalf("expected failure result")
	}

	ship, err := orm.GetIslandShip(client.Commander.CommanderID, 1051700)
	if err != nil {
		t.Fatalf("get island ship: %v", err)
	}
	if ship.BreakLv != 1 {
		t.Fatalf("expected break level unchanged, got %d", ship.BreakLv)
	}
	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 100201)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	if item.Count != 1 {
		t.Fatalf("expected item unchanged, got %d", item.Count)
	}
}
