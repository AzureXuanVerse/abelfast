package answer

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestIslandClaimOrderFavorRewardSuccessAndDuplicate(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandOrderFavorCategory, "1", `{"id":1,"level":1,"exp":0,"award_display":[[41,1,10]]}`)
	seedConfigEntry(t, islandOrderFavorCategory, "2", `{"id":2,"level":2,"exp":10,"award_display":[[41,1,20]]}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		state.Favor = 10
		return orm.SaveIslandOrderStateTx(context.Background(), tx, state)
	})
	if err != nil {
		t.Fatalf("seed order state: %v", err)
	}

	payload := protobuf.CS_21412{Lv: proto.Uint32(2)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandClaimOrderFavorReward(&buffer, client); err != nil {
		t.Fatalf("claim favor reward: %v", err)
	}
	var response protobuf.SC_21413
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 1 || response.GetDropList()[0].GetNumber() != 20 {
		t.Fatalf("unexpected response: %+v", response)
	}

	client.Buffer.Reset()
	if _, _, err := IslandClaimOrderFavorReward(&buffer, client); err != nil {
		t.Fatalf("duplicate claim: %v", err)
	}
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected duplicate claim to fail")
	}
}

func TestIslandSubmitUrgencyOrderSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandSetCategory, "order_favor", `{"key":"order_favor","key_value_int":20,"key_value_varchar":""}`)
	seedConfigEntry(t, islandOrderPriceCategory, "3", `{"id":3,"order_award_special":[7001,5]}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 9001, 10); err != nil {
			return err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{Id: proto.Uint32(201), Type: proto.Uint32(2), CurSelect: proto.Uint32(1), StartTime: proto.Uint32(1), SubmitTime: proto.Uint32(1), Position: proto.Uint32(1), DialogId: proto.Uint32(1), Cost: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9001), Num: proto.Uint32(4)}}, OrderLv: proto.Uint32(3), ViewFlag: proto.Uint32(0)}
		return orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot)
	})
	if err != nil {
		t.Fatalf("seed urgency state: %v", err)
	}

	payload := protobuf.CS_21405{SlotId: proto.Uint32(201)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandSubmitUrgencyOrder(&buffer, client); err != nil {
		t.Fatalf("submit urgency: %v", err)
	}
	var response protobuf.SC_21406
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestIslandSubmitUrgencyOrderMissingCostRollsBackConsumption(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandSetCategory, "order_favor", `{"key":"order_favor","key_value_int":20,"key_value_varchar":""}`)
	seedConfigEntry(t, islandOrderPriceCategory, "3", `{"id":3,"order_award_special":[7001,5]}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 9001, 10); err != nil {
			return err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{Id: proto.Uint32(202), Type: proto.Uint32(2), CurSelect: proto.Uint32(1), StartTime: proto.Uint32(1), SubmitTime: proto.Uint32(1), Position: proto.Uint32(1), DialogId: proto.Uint32(1), Cost: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9001), Num: proto.Uint32(4)}, {Id: proto.Uint32(9002), Num: proto.Uint32(1)}}, OrderLv: proto.Uint32(3), ViewFlag: proto.Uint32(0)}
		return orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot)
	})
	if err != nil {
		t.Fatalf("seed urgency state: %v", err)
	}

	payload := protobuf.CS_21405{SlotId: proto.Uint32(202)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandSubmitUrgencyOrder(&buffer, client); err != nil {
		t.Fatalf("submit urgency: %v", err)
	}
	var response protobuf.SC_21406
	decodeResponse(t, client, &response)
	if response.GetResult() != islandUrgencySubmitInsufficient {
		t.Fatalf("expected insufficient result, got: %+v", response)
	}

	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 9001)
	if err != nil {
		t.Fatalf("load inventory after failure: %v", err)
	}
	if item.Count != 10 {
		t.Fatalf("expected inventory rollback to preserve count 10, got %d", item.Count)
	}
}

func TestIslandSetOrderTendencyPersistsAndSyncs(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)

	payload := protobuf.CS_21410{Type: proto.Uint32(2)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandSetOrderTendency(&buffer, client); err != nil {
		t.Fatalf("set tendency: %v", err)
	}
	var setResponse protobuf.SC_21411
	decodeResponse(t, client, &setResponse)
	if setResponse.GetResult() != 0 {
		t.Fatalf("unexpected set tendency response: %+v", setResponse)
	}

	syncPayload := protobuf.CS_21024{Type: proto.Uint32(1)}
	syncBuffer, _ := proto.Marshal(&syncPayload)
	client.Buffer.Reset()
	if _, _, err := IslandOrderSync(&syncBuffer, client); err != nil {
		t.Fatalf("order sync: %v", err)
	}
	var syncResponse protobuf.SC_21025
	decodeResponse(t, client, &syncResponse)
	if syncResponse.GetResult() != 0 || syncResponse.GetOrderSys().GetDailySelect() != 2 {
		t.Fatalf("unexpected sync response: %+v", syncResponse)
	}

	invalidPayload := protobuf.CS_21410{Type: proto.Uint32(9)}
	invalidBuffer, _ := proto.Marshal(&invalidPayload)
	client.Buffer.Reset()
	if _, _, err := IslandSetOrderTendency(&invalidBuffer, client); err != nil {
		t.Fatalf("set invalid tendency: %v", err)
	}
	decodeResponse(t, client, &setResponse)
	if setResponse.GetResult() == 0 {
		t.Fatalf("expected invalid tendency failure")
	}
}

func TestIslandSubmitCommonOrderSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandSetCategory, "order_favor", `{"key":"order_favor","key_value_int":20,"key_value_varchar":""}`)
	seedConfigEntry(t, islandSetCategory, "order_complete_refresh_time", `{"key":"order_complete_refresh_time","key_value_int":5,"key_value_varchar":""}`)
	seedConfigEntry(t, islandOrderRandomCategory, "10", `{"id":10}`)
	seedConfigEntry(t, islandOrderRandomCategory, "11", `{"id":11}`)
	seedConfigEntry(t, islandOrderPriceCategory, "3", `{"id":3,"order_award":[7001,6],"order_easy_award":[7002,3],"order_award_challenge":[7003,2]}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 9001, 10); err != nil {
			return err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{Id: proto.Uint32(211), Type: proto.Uint32(1), CurSelect: proto.Uint32(1), StartTime: proto.Uint32(1), SubmitTime: proto.Uint32(1), Position: proto.Uint32(1), DialogId: proto.Uint32(10), Cost: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9001), Num: proto.Uint32(4)}}, OrderLv: proto.Uint32(3), ViewFlag: proto.Uint32(0)}
		return orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot)
	})
	if err != nil {
		t.Fatalf("seed common order: %v", err)
	}

	payload := protobuf.CS_21401{SlotId: proto.Uint32(211)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandSubmitCommonOrder(&buffer, client); err != nil {
		t.Fatalf("submit common: %v", err)
	}
	var response protobuf.SC_21402
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != 7001 {
		t.Fatalf("unexpected common response: %+v", response)
	}
}

func TestIslandSubmitFirmOrderActivityTracksActGroup(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandSetCategory, "order_favor", `{"key":"order_favor","key_value_int":20,"key_value_varchar":""}`)
	seedConfigEntry(t, islandOrderCategory, "2001", `{"id":2001,"type":3,"award":[7101,5],"season_pt_num":12,"activity_id":88,"group_id":99,"next_order":0}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 9011, 10); err != nil {
			return err
		}
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		state.Favor = 30
		if err := orm.SaveIslandOrderStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{Id: proto.Uint32(301), Type: proto.Uint32(4), CurSelect: proto.Uint32(1), StartTime: proto.Uint32(1), SubmitTime: proto.Uint32(1), Position: proto.Uint32(1), DialogId: proto.Uint32(2001), Cost: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9011), Num: proto.Uint32(2)}}, OrderLv: proto.Uint32(3), ViewFlag: proto.Uint32(0)}
		return orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot)
	})
	if err != nil {
		t.Fatalf("seed firm order: %v", err)
	}

	payload := protobuf.CS_21414{OrderId: proto.Uint32(301)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandSubmitFirmOrder(&buffer, client); err != nil {
		t.Fatalf("submit firm: %v", err)
	}
	var response protobuf.SC_21415
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != 7101 {
		t.Fatalf("unexpected firm response: %+v", response)
	}

	var orderSys *protobuf.PB_ISLAND_ORDER_SYSTEM
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		if state.Favor != 30 {
			t.Fatalf("expected favor unchanged for activity firm, got %d", state.Favor)
		}
		groups, err := orm.ListIslandOrderActGroupsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		orderSys = &protobuf.PB_ISLAND_ORDER_SYSTEM{ActGroup: groups}
		return nil
	})
	if err != nil {
		t.Fatalf("verify act group: %v", err)
	}
	if len(orderSys.GetActGroup()) != 1 || orderSys.GetActGroup()[0].GetActId() != 88 || len(orderSys.GetActGroup()[0].GetGroups()) != 1 || orderSys.GetActGroup()[0].GetGroups()[0] != 99 {
		t.Fatalf("unexpected act group state: %+v", orderSys.GetActGroup())
	}
}

func TestIslandReplaceOrderSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandOrderRandomCategory, "10", `{"id":10}`)
	seedConfigEntry(t, islandOrderRandomCategory, "11", `{"id":11}`)
	seedConfigEntry(t, islandSetCategory, "order_complete_refresh_time", `{"key":"order_complete_refresh_time","key_value_int":5,"key_value_varchar":""}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{Id: proto.Uint32(101), Type: proto.Uint32(1), CurSelect: proto.Uint32(1), StartTime: proto.Uint32(10), SubmitTime: proto.Uint32(uint32(time.Now().Add(-time.Hour).Unix())), Position: proto.Uint32(1), DialogId: proto.Uint32(10), Cost: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(1), Num: proto.Uint32(2)}}, OrderLv: proto.Uint32(1), ViewFlag: proto.Uint32(0)}
		return orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot)
	})
	if err != nil {
		t.Fatalf("seed slot: %v", err)
	}

	payload := protobuf.CS_21403{SlotId: proto.Uint32(101)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandReplaceOrder(&buffer, client); err != nil {
		t.Fatalf("replace order: %v", err)
	}
	var response protobuf.SC_21404
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || response.GetSlot().GetDialogId() != 11 || response.GetSlot().GetCost()[0].GetNum() != 3 {
		t.Fatalf("unexpected replacement response: %+v", response)
	}
}

func TestIslandClaimSeasonPTRewardClaimAll(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandSetCategory, "season_now", `{"key":"season_now","key_value_int":1,"key_value_varchar":""}`)
	seedConfigEntry(t, islandSeasonCategory, "1", `{"id":1,"target":[100,400,800],"ptaward_display":[[41,1,2],[41,2,3],[41,3,4]]}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, 500)
	})
	if err != nil {
		t.Fatalf("seed season pt: %v", err)
	}

	payload := protobuf.CS_21022{TargetPt: proto.Uint32(0)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandClaimSeasonPTReward(&buffer, client); err != nil {
		t.Fatalf("claim season rewards: %v", err)
	}
	var response protobuf.SC_21023
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 2 {
		t.Fatalf("unexpected season response: %+v", response)
	}
}

func TestIslandClaimSeasonPTRewardClaimAllSkipsConflictedClaims(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandSetCategory, "season_now", `{"key":"season_now","key_value_int":1,"key_value_varchar":""}`)
	seedConfigEntry(t, islandSeasonCategory, "1", `{"id":1,"target":[100,100],"ptaward_display":[[41,1,2],[41,1,2]]}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, 500)
	})
	if err != nil {
		t.Fatalf("seed season pt: %v", err)
	}

	payload := protobuf.CS_21022{TargetPt: proto.Uint32(0)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandClaimSeasonPTReward(&buffer, client); err != nil {
		t.Fatalf("claim season rewards: %v", err)
	}
	var response protobuf.SC_21023
	decodeResponse(t, client, &response)
	if response.GetResult() != islandSeasonPTAwardOK || len(response.GetDropList()) != 1 || response.GetDropList()[0].GetNumber() != 2 {
		t.Fatalf("unexpected season response: %+v", response)
	}

	var claimed []uint32
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		var listErr error
		claimed, listErr = orm.ListIslandSeasonRewardClaimsTx(context.Background(), tx, client.Commander.CommanderID)
		return listErr
	})
	if err != nil {
		t.Fatalf("list reward claims: %v", err)
	}
	if len(claimed) != 1 || claimed[0] != 100 {
		t.Fatalf("expected one claim row for target 100, got %v", claimed)
	}
}

func TestIslandClaimProsperityReward(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandProsperityCategory, "1", `{"id":1,"prosperity":300,"award_display":[[41,1,5]]}`)
	if err := orm.SetIslandProsperity(client.Commander.CommanderID, 400); err != nil {
		t.Fatalf("seed prosperity: %v", err)
	}

	payload := protobuf.CS_21010{Level: proto.Uint32(1)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandClaimProsperityReward(&buffer, client); err != nil {
		t.Fatalf("claim prosperity reward: %v", err)
	}
	var response protobuf.SC_21011
	decodeResponse(t, client, &response)
	if response.GetRet() != 0 || len(response.GetDropList()) != 1 {
		t.Fatalf("unexpected prosperity response: %+v", response)
	}
}

func TestIslandExchangeShipOrderDelegate(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot := &protobuf.PB_ISLAND_ORDER_SHIP_SLOT{Id: proto.Uint32(301), State: proto.Uint32(1), LoadTime: proto.Uint32(0), GetTime: proto.Uint32(0), Cost: []*protobuf.PB_ISLAND_ORDER_SHIP_LOAD{{Id: proto.Uint32(10), Num: proto.Uint32(1), State: proto.Uint32(1)}}, Reward: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(20), Num: proto.Uint32(1)}}, FinishNum: proto.Uint32(0), AutoTime: proto.Uint32(0)}
		if err := orm.UpsertIslandShipOrderSlotDataTx(context.Background(), tx, client.Commander.CommanderID, slot); err != nil {
			return err
		}
		appoint := &protobuf.PB_SHIP_ORDER_APPOINT{Id: proto.Uint32(9), ViewTime: proto.Uint32(100), Cost: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(11), Num: proto.Uint32(2)}}, Reward: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(22), Num: proto.Uint32(3)}}}
		return orm.UpsertIslandShipOrderAppointTx(context.Background(), tx, client.Commander.CommanderID, appoint)
	})
	if err != nil {
		t.Fatalf("seed ship order state: %v", err)
	}

	payload := protobuf.CS_21431{SlotId: proto.Uint32(301), AppointId: proto.Uint32(9)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandExchangeShipOrderDelegate(&buffer, client); err != nil {
		t.Fatalf("exchange delegate: %v", err)
	}
	var response protobuf.SC_21432
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || response.GetAppoint().GetId() != 1000009 {
		t.Fatalf("unexpected exchange response: %+v", response)
	}
}

func TestIslandOrderSyncIncludesOrderSystem(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		state.Favor = 77
		state.DailySelect = 3
		if err := orm.SaveIslandOrderStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		if _, err := orm.AddIslandOrderFavorClaimTx(context.Background(), tx, client.Commander.CommanderID, 2); err != nil {
			return err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{Id: proto.Uint32(101), Type: proto.Uint32(1), CurSelect: proto.Uint32(1), StartTime: proto.Uint32(1), SubmitTime: proto.Uint32(1), Position: proto.Uint32(1), DialogId: proto.Uint32(10), Cost: []*protobuf.PB_ISLAND_ITEM{}, OrderLv: proto.Uint32(1), ViewFlag: proto.Uint32(0)}
		return orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot)
	})
	if err != nil {
		t.Fatalf("seed sync state: %v", err)
	}

	payload := protobuf.CS_21024{Type: proto.Uint32(1)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandOrderSync(&buffer, client); err != nil {
		t.Fatalf("order sync: %v", err)
	}
	var response protobuf.SC_21025
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || response.GetOrderSys() == nil || response.GetOrderSys().GetFavor() != 77 || len(response.GetOrderSys().GetGetFavor_()) != 1 {
		t.Fatalf("unexpected sync response: %+v", response)
	}
}

func clearIslandEconomyTables(t *testing.T) {
	t.Helper()
	execAnswerTestSQLT(t, `
TRUNCATE TABLE
	config_entries,
	island_inventories,
	island_seasons,
		island_order_states,
		island_prosperity_states,
		island_order_favor_claims,
		island_order_act_groups,
		island_order_slots,
		island_manage_trades,
		island_ship_order_slots,
		island_ship_order_appoints,
		island_season_reward_claims
RESTART IDENTITY CASCADE
`)
}
