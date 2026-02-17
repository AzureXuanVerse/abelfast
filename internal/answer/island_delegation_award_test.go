package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandGetDelegationAwardTypeOneSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandDelegation{})
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandSeason{})

	seedConfigEntry(t, islandFormulaCategory, "101001", `{"id":101001,"commission_product":[[2000,2]],"second_product":[[2001,3]],"pt_award":5}`)
	seedIslandDelegation(t, client.Commander.CommanderID, orm.IslandDelegation{
		BuildID:       10101,
		AreaID:        9001,
		HasRole:       true,
		RewardReady:   true,
		FormulaID:     101001,
		MainNum:       2,
		OtherNum:      1,
		ExtraMainNum:  1,
		ExtraOtherNum: 2,
		GetTimes:      4,
	})

	payload := protobuf.CS_21505{Type: proto.Uint32(1), BuildId: proto.Uint32(10101), AreaId: proto.Uint32(9001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := IslandGetDelegationAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21506
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetGetTimes() != 5 {
		t.Fatalf("expected get_times 5, got %d", response.GetGetTimes())
	}
	if response.GetPtAward() != 5 {
		t.Fatalf("expected pt_award 5, got %d", response.GetPtAward())
	}
	if response.GetFormulaId() != 101001 {
		t.Fatalf("expected formula_id 101001, got %d", response.GetFormulaId())
	}
	if len(response.GetDropList()) != 2 {
		t.Fatalf("expected two drops, got %d", len(response.GetDropList()))
	}
	if response.GetDropList()[0].GetType() != consts.DROP_TYPE_ISLAND_ITEM {
		t.Fatalf("expected island item drop type")
	}
	if response.GetDropList()[0].GetId() != 2000 || response.GetDropList()[0].GetNumber() != 5 {
		t.Fatalf("unexpected main drop %#v", response.GetDropList()[0])
	}
	if response.GetDropList()[1].GetId() != 2001 || response.GetDropList()[1].GetNumber() != 5 {
		t.Fatalf("unexpected second drop %#v", response.GetDropList()[1])
	}

	state, err := orm.GetIslandDelegation(client.Commander.CommanderID, 10101, 9001)
	if err != nil {
		t.Fatalf("reload delegation: %v", err)
	}
	if state.RewardReady {
		t.Fatalf("expected reward to be consumed")
	}
	if !state.HasRole {
		t.Fatalf("expected role assignment to remain for type=1")
	}

	inventoryMain, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 2000)
	if err != nil || inventoryMain.Count != 5 {
		t.Fatalf("expected island item 2000 count 5, err=%v count=%v", err, inventoryMain)
	}
	inventorySecond, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 2001)
	if err != nil || inventorySecond.Count != 5 {
		t.Fatalf("expected island item 2001 count 5, err=%v count=%v", err, inventorySecond)
	}
	season, err := orm.GetIslandSeason(client.Commander.CommanderID)
	if err != nil || season.PT != 5 {
		t.Fatalf("expected season pt 5, err=%v season=%v", err, season)
	}
}

func TestIslandGetDelegationAwardTypeTwoSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandDelegation{})
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandSeason{})

	seedConfigEntry(t, islandFormulaCategory, "101002", `{"id":101002,"commission_product":[[2002,4]],"pt_award":0}`)
	seedIslandDelegation(t, client.Commander.CommanderID, orm.IslandDelegation{
		BuildID:     10102,
		AreaID:      9002,
		HasRole:     false,
		RewardReady: true,
		FormulaID:   101002,
		MainNum:     1,
		GetTimes:    8,
		PTAward:     11,
	})

	payload := protobuf.CS_21505{Type: proto.Uint32(2), BuildId: proto.Uint32(10102), AreaId: proto.Uint32(9002)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := IslandGetDelegationAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21506
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetGetTimes() != 0 {
		t.Fatalf("expected get_times 0, got %d", response.GetGetTimes())
	}
	if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != 2002 {
		t.Fatalf("expected one drop for item 2002")
	}

	state, err := orm.GetIslandDelegation(client.Commander.CommanderID, 10102, 9002)
	if err != nil {
		t.Fatalf("reload delegation: %v", err)
	}
	if state.GetTimes != 0 {
		t.Fatalf("expected get_times reset to 0, got %d", state.GetTimes)
	}
}

func TestIslandGetDelegationAwardValidationFailure(t *testing.T) {
	client := setupHandlerCommander(t)
	payload := protobuf.CS_21505{Type: proto.Uint32(9), BuildId: proto.Uint32(10101), AreaId: proto.Uint32(9001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := IslandGetDelegationAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21506
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	if len(response.GetDropList()) != 0 || response.GetPtAward() != 0 {
		t.Fatalf("expected empty reward payload on validation failure")
	}
}

func TestIslandGetDelegationAwardMissingRewardDoesNotMutate(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandDelegation{})
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandSeason{})

	seedConfigEntry(t, islandFormulaCategory, "101003", `{"id":101003,"commission_product":[[2003,2]],"pt_award":3}`)
	seedIslandDelegation(t, client.Commander.CommanderID, orm.IslandDelegation{
		BuildID:     10103,
		AreaID:      9003,
		HasRole:     true,
		RewardReady: false,
		FormulaID:   101003,
		MainNum:     2,
		GetTimes:    7,
	})

	payload := protobuf.CS_21505{Type: proto.Uint32(1), BuildId: proto.Uint32(10103), AreaId: proto.Uint32(9003)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := IslandGetDelegationAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21506
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}

	state, err := orm.GetIslandDelegation(client.Commander.CommanderID, 10103, 9003)
	if err != nil {
		t.Fatalf("reload delegation: %v", err)
	}
	if state.GetTimes != 7 {
		t.Fatalf("expected get_times unchanged, got %d", state.GetTimes)
	}
	if _, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 2003); err == nil {
		t.Fatalf("expected no island inventory row")
	}
}

func TestIslandGetDelegationAwardPersistenceFailureRollsBack(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandDelegation{})
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandSeason{})

	seedConfigEntry(t, islandFormulaCategory, "101004", `{"id":101004,"commission_product":[[2004,2]],"second_product":[[0,1]],"pt_award":4}`)
	seedIslandDelegation(t, client.Commander.CommanderID, orm.IslandDelegation{
		BuildID:     10104,
		AreaID:      9004,
		HasRole:     true,
		RewardReady: true,
		FormulaID:   101004,
		MainNum:     2,
		OtherNum:    1,
		GetTimes:    3,
	})

	payload := protobuf.CS_21505{Type: proto.Uint32(1), BuildId: proto.Uint32(10104), AreaId: proto.Uint32(9004)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := IslandGetDelegationAward(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21506
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result on persistence failure")
	}

	if _, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 2004); err == nil {
		t.Fatalf("expected inventory write rollback")
	}
	if _, err := orm.GetIslandSeason(client.Commander.CommanderID); err == nil {
		t.Fatalf("expected season pt write rollback")
	}
	state, err := orm.GetIslandDelegation(client.Commander.CommanderID, 10104, 9004)
	if err != nil {
		t.Fatalf("reload delegation: %v", err)
	}
	if !state.RewardReady || state.GetTimes != 3 {
		t.Fatalf("expected delegation state unchanged after rollback")
	}
}

func seedIslandDelegation(t *testing.T, commanderID uint32, state orm.IslandDelegation) {
	t.Helper()
	state.CommanderID = commanderID
	if err := orm.UpsertIslandDelegation(&state); err != nil {
		t.Fatalf("seed island delegation: %v", err)
	}
}
