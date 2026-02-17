package answer

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func seedIslandHandPlantConfig(t *testing.T) {
	t.Helper()
	clearTable(t, &orm.ConfigEntry{})
	seedConfigEntry(t, islandProductionSlotCategory, "2001", `{"id":2001,"place":10101,"type":1,"formula":[3001,3002]}`)
	seedConfigEntry(t, islandProductionSlotCategory, "2002", `{"id":2002,"place":10101,"type":1,"formula":[3001]}`)
	seedConfigEntry(t, islandProductionSlotCategory, "2003", `{"id":2003,"place":10102,"type":1,"formula":[3001]}`)
	seedConfigEntry(t, islandProductionSlotCategory, "2004", `{"id":2004,"place":10101,"type":2,"formula":[3001]}`)
	seedConfigEntry(t, islandFormulaCategory, "3001", `{"id":3001,"cost":[[17001,2]],"workload":240,"drop_display":[[99,8001,3]]}`)
	seedConfigEntry(t, islandFormulaCategory, "3002", `{"id":3002,"cost":[[17001,1]],"workload":120,"drop_display":[[99,8002,1]]}`)
	seedConfigEntry(t, islandSetCategory, "base_efficiency", `{"key":"base_efficiency","description":[120]}`)
}

func TestStartIslandHandPlantSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	seedIslandHandPlantConfig(t)
	seedHandlerCommanderItem(t, client, 17001, 20)

	payload := protobuf.CS_21509{BuildId: proto.Uint32(10101), SlotList: []uint32{2001, 2002}, FormulaId: proto.Uint32(3001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, _, err := StartIslandHandPlant(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21510
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetHandList()) != 2 {
		t.Fatalf("expected two hand slots, got %d", len(response.GetHandList()))
	}
	for _, hand := range response.GetHandList() {
		if hand.GetState() != 1 || hand.GetFormulaId() != 3001 || hand.GetStartTime() == 0 || hand.GetEndTime() <= hand.GetStartTime() {
			t.Fatalf("unexpected hand slot payload %+v", hand)
		}
	}

	remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(17001))
	if remaining != 16 {
		t.Fatalf("expected 4 seeds consumed, remaining=%d", remaining)
	}

	stateCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM island_hand_plants WHERE commander_id = $1 AND build_id = $2 AND state = 1", int64(client.Commander.CommanderID), int64(10101))
	if stateCount != 2 {
		t.Fatalf("expected two active hand plant slots, got %d", stateCount)
	}
}

func TestStartIslandHandPlantFailsOnBuildMismatch(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	seedIslandHandPlantConfig(t)
	seedHandlerCommanderItem(t, client, 17001, 20)

	payload := protobuf.CS_21509{BuildId: proto.Uint32(10101), SlotList: []uint32{2001, 2003}, FormulaId: proto.Uint32(3001)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := StartIslandHandPlant(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21510
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	stateCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM island_hand_plants WHERE commander_id = $1", int64(client.Commander.CommanderID))
	if stateCount != 0 {
		t.Fatalf("expected no state writes on validation failure")
	}
}

func TestStartIslandHandPlantFailsOnInsufficientSeeds(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	seedIslandHandPlantConfig(t)
	seedHandlerCommanderItem(t, client, 17001, 1)

	payload := protobuf.CS_21509{BuildId: proto.Uint32(10101), SlotList: []uint32{2001}, FormulaId: proto.Uint32(3001)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := StartIslandHandPlant(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21510
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(17001))
	if remaining != 1 {
		t.Fatalf("expected no seed consumption, got %d", remaining)
	}
}

func TestIslandClaimHandPlantAwardSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandHandPlantConfig(t)

	now := uint32(time.Now().UTC().Unix())
	seedIslandHandPlantState(t, client.Commander.CommanderID, orm.IslandHandPlant{BuildID: 10101, SlotID: 2001, State: 1, FormulaID: 3001, StartTime: now - 100, EndTime: now - 1})
	seedIslandHandPlantState(t, client.Commander.CommanderID, orm.IslandHandPlant{BuildID: 10101, SlotID: 2002, State: 1, FormulaID: 3001, StartTime: now - 100, EndTime: now - 1})

	payload := protobuf.CS_21511{BuildId: proto.Uint32(10101), AreaIds: []uint32{2001, 2002}}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandClaimHandPlantAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21512
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != 8001 || response.GetDropList()[0].GetNumber() != 6 {
		t.Fatalf("unexpected drops: %+v", response.GetDropList())
	}
	inv := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(8001))
	if inv != 6 {
		t.Fatalf("expected island inventory count 6, got %d", inv)
	}
	active := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM island_hand_plants WHERE commander_id = $1 AND state <> 0", int64(client.Commander.CommanderID))
	if active != 0 {
		t.Fatalf("expected slots reset after harvest, got %d active", active)
	}
}

func TestIslandClaimHandPlantAwardFailsWhenNotReady(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandHandPlantConfig(t)

	now := uint32(time.Now().UTC().Unix())
	seedIslandHandPlantState(t, client.Commander.CommanderID, orm.IslandHandPlant{BuildID: 10101, SlotID: 2001, State: 1, FormulaID: 3001, StartTime: now - 100, EndTime: now + 3600})

	payload := protobuf.CS_21511{BuildId: proto.Uint32(10101), AreaIds: []uint32{2001}}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandClaimHandPlantAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21512
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	invRows := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM island_inventories WHERE commander_id = $1", int64(client.Commander.CommanderID))
	if invRows != 0 {
		t.Fatalf("expected no reward inventory writes")
	}
}

func TestHandleIslandStopHandPlantHalfwaySuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	seedIslandHandPlantConfig(t)

	now := uint32(time.Now().UTC().Unix())
	seedIslandHandPlantState(t, client.Commander.CommanderID, orm.IslandHandPlant{BuildID: 10101, SlotID: 2001, State: 1, FormulaID: 3001, StartTime: now - 200, EndTime: now + 100})

	payload := protobuf.CS_21516{BuildId: proto.Uint32(10101), SlotList: []uint32{2001}}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := HandleIslandStopHandPlantHalfway(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21517
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	state := queryAnswerTestInt64(t, "SELECT state FROM island_hand_plants WHERE commander_id = $1 AND slot_id = $2", int64(client.Commander.CommanderID), int64(2001))
	if state != 0 {
		t.Fatalf("expected slot reset to state 0, got %d", state)
	}
}

func TestHandleIslandStopHandPlantHalfwayFailsOnInvalidSlot(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandHandPlant{})
	seedIslandHandPlantConfig(t)

	payload := protobuf.CS_21516{BuildId: proto.Uint32(10101), SlotList: []uint32{2004}}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := HandleIslandStopHandPlantHalfway(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21517
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
}

func seedIslandHandPlantState(t *testing.T, commanderID uint32, state orm.IslandHandPlant) {
	t.Helper()
	state.CommanderID = commanderID
	execAnswerTestSQLT(t, `
INSERT INTO island_hand_plants (commander_id, build_id, slot_id, state, formula_id, start_time, end_time)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (commander_id, slot_id) DO UPDATE SET
	build_id = EXCLUDED.build_id,
	state = EXCLUDED.state,
	formula_id = EXCLUDED.formula_id,
	start_time = EXCLUDED.start_time,
	end_time = EXCLUDED.end_time
`, int64(state.CommanderID), int64(state.BuildID), int64(state.SlotID), int64(state.State), int64(state.FormulaID), int64(state.StartTime), int64(state.EndTime))
}
