package answer

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestIslandUpgradeSuccessPersistsSnapshot(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedHandlerCommanderResource(t, client, 1, 100)
	seedConfigEntry(t, islandLevelCategory, "1", `[{"id":1,"island_level":1,"island_exp":50,"cost":[[1,10]],"island_level_award":[[41,7001,2]]},{"id":2,"island_level":2,"island_exp":100,"cost":[],"island_level_award":[]}]`)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, Level: 1, Exp: 60, StorageLevel: 1}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21000{Type: proto.Uint32(0)})
	client.Buffer.Reset()
	if _, _, err := IslandUpgrade(&payload, client); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	var response protobuf.SC_21001
	decodeResponse(t, client, &response)
	if response.GetRet() != 0 || len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != 7001 {
		t.Fatalf("unexpected response: %+v", response)
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if snapshot.Level != 2 || snapshot.Exp != 10 {
		t.Fatalf("unexpected snapshot state: %+v", snapshot)
	}
}

func TestIslandSetAccessAuthorityAppliesFlags(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, OpenFlag: 3, StorageLevel: 1, Level: 1}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21002{OpenFlag: []uint32{4, 2}, CloseFlag: []uint32{2, 1}})
	client.Buffer.Reset()
	if _, _, err := IslandSetAccessAuthority(&payload, client); err != nil {
		t.Fatalf("set authority: %v", err)
	}
	var response protobuf.SC_21003
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("unexpected result: %d", response.GetResult())
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.OpenFlag != 6 {
		t.Fatalf("expected open_flag=6, got %d", snapshot.OpenFlag)
	}
}

func TestIslandSetNameValidatesAndPersists(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	payload, _ := proto.Marshal(&protobuf.CS_21004{Name: proto.String("Beacon"), Type: proto.Uint32(1)})
	client.Buffer.Reset()
	if _, _, err := IslandSetName(&payload, client); err != nil {
		t.Fatalf("set name: %v", err)
	}
	var response protobuf.SC_21005
	decodeResponse(t, client, &response)
	if response.GetRet() != 0 {
		t.Fatalf("expected success ret")
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.Name != "Beacon" {
		t.Fatalf("expected renamed snapshot, got %q", snapshot.Name)
	}
}

func TestIslandUpgradeInventoryConsumesMaterial(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedConfigEntry(t, islandStorageLevelCategory, "1", `[{"id":1,"level":1,"upgrade_material":[[41,8001,2]]},{"id":2,"level":2,"upgrade_material":[]}]`)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, StorageLevel: 1, Level: 1}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 8001, 3)
	})
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21012{Type: proto.Uint32(0)})
	client.Buffer.Reset()
	if _, _, err := IslandUpgradeInventory(&payload, client); err != nil {
		t.Fatalf("upgrade inventory: %v", err)
	}
	var response protobuf.SC_21013
	decodeResponse(t, client, &response)
	if response.GetRet() != 0 {
		t.Fatalf("expected success ret")
	}
	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.StorageLevel != 2 {
		t.Fatalf("expected storage level 2, got %d", snapshot.StorageLevel)
	}
	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 8001)
	if err != nil || item.Count != 1 {
		t.Fatalf("expected remaining material count 1, err=%v item=%+v", err, item)
	}
}

func TestIslandUpgradeInventoryRollsBackWhenLaterMaterialMissing(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedConfigEntry(t, islandStorageLevelCategory, "1", `[{"id":1,"level":1,"upgrade_material":[[41,8001,2],[41,8002,2]]},{"id":2,"level":2,"upgrade_material":[]}]`)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, StorageLevel: 1, Level: 1}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 8001, 2); err != nil {
			return err
		}
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 8002, 1)
	})
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21012{Type: proto.Uint32(0)})
	client.Buffer.Reset()
	if _, _, err := IslandUpgradeInventory(&payload, client); err != nil {
		t.Fatalf("upgrade inventory: %v", err)
	}
	var response protobuf.SC_21013
	decodeResponse(t, client, &response)
	if response.GetRet() == 0 {
		t.Fatalf("expected failure ret when one material is missing")
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.StorageLevel != 1 {
		t.Fatalf("expected storage level unchanged on rollback, got %d", snapshot.StorageLevel)
	}
	matA, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 8001)
	if err != nil || matA.Count != 2 {
		t.Fatalf("expected first material rollback, err=%v item=%+v", err, matA)
	}
	matB, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 8002)
	if err != nil || matB.Count != 1 {
		t.Fatalf("expected second material unchanged, err=%v item=%+v", err, matB)
	}
}

func TestIslandSellConvertFromOverflowAddsOutputs(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedConfigEntry(t, islandItemTemplateCategory, "9001", `{"id":9001,"convert":9100,"pt_num":5}`)
	if err := orm.UpsertIslandOverflowInventory(client.Commander.CommanderID, 9001, 4); err != nil {
		t.Fatalf("seed overflow: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21014{Type: proto.Uint32(2), ItemList: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9001), Num: proto.Uint32(3)}}})
	client.Buffer.Reset()
	if _, _, err := IslandSellOrConvertItems(&payload, client); err != nil {
		t.Fatalf("sell/convert: %v", err)
	}
	var response protobuf.SC_21015
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetItemList()) != 1 || response.GetItemList()[0].GetId() != 9100 {
		t.Fatalf("unexpected response: %+v", response)
	}
	converted, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 9100)
	if err != nil || converted.Count != 3 {
		t.Fatalf("expected converted items, err=%v item=%+v", err, converted)
	}
	season, err := orm.GetIslandSeason(client.Commander.CommanderID)
	if err != nil || season.PT != 15 {
		t.Fatalf("expected season pt gain 15, err=%v season=%+v", err, season)
	}
}

func TestIslandShopGetDataAndPurchase(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedConfigEntry(t, islandShopTemplateCategory, "10", `{"id":10,"goods_id":[1001]}`)
	seedConfigEntry(t, islandShopNormalCategory, "10", `{"id":10,"refresh_set":3,"refresh_player":[1,1,5],"refresh_free":1,"refresh_time":60,"exist_time":600}`)
	seedConfigEntry(t, islandShopGoodsCategory, "1001", `{"id":1001,"resource_consume":[1,1,5],"items":[[41,9301,2]],"limited_num":2,"pay_id":0,"pt_award":3}`)
	seedHandlerCommanderResource(t, client, 1, 100)

	getPayload, _ := proto.Marshal(&protobuf.CS_21016{ShopId: proto.Uint32(10)})
	client.Buffer.Reset()
	if _, _, err := IslandShopGetData(&getPayload, client); err != nil {
		t.Fatalf("shop get: %v", err)
	}
	var getResponse protobuf.SC_21017
	decodeResponse(t, client, &getResponse)
	if getResponse.GetResult() != 0 || getResponse.GetShopInfo() == nil {
		t.Fatalf("unexpected shop get response: %+v", getResponse)
	}

	buyPayload, _ := proto.Marshal(&protobuf.CS_21018{GoodsList: []*protobuf.KVDATA2{{Key: proto.Uint32(10), Value1: proto.Uint32(1001), Value2: proto.Uint32(1)}}})
	client.Buffer.Reset()
	if _, _, err := IslandShopPurchase(&buyPayload, client); err != nil {
		t.Fatalf("shop purchase: %v", err)
	}
	var buyResponse protobuf.SC_21019
	decodeResponse(t, client, &buyResponse)
	if buyResponse.GetResult() != 0 || len(buyResponse.GetDropList()) != 1 || buyResponse.GetDropList()[0].GetId() != 9301 {
		t.Fatalf("unexpected shop purchase response: %+v", buyResponse)
	}
	if client.Commander.GetResourceCount(1) != 95 {
		t.Fatalf("expected resource spend to 95, got %d", client.Commander.GetResourceCount(1))
	}
}

func TestIslandShopPurchaseRollsBackOnMixedCurrencyFailure(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedConfigEntry(t, islandShopTemplateCategory, "10", `{"id":10,"goods_id":[1001,1002]}`)
	seedConfigEntry(t, islandShopNormalCategory, "10", `{"id":10,"refresh_set":3,"refresh_player":[1,1,5],"refresh_free":1,"refresh_time":60,"exist_time":600}`)
	seedConfigEntry(t, islandShopGoodsCategory, "1001", `{"id":1001,"resource_consume":[1,1,5],"items":[[41,9301,1]],"limited_num":2,"pay_id":0,"pt_award":0}`)
	seedConfigEntry(t, islandShopGoodsCategory, "1002", `{"id":1002,"resource_consume":[2,20001,2],"items":[[41,9302,1]],"limited_num":2,"pay_id":0,"pt_award":0}`)
	seedHandlerCommanderResource(t, client, 1, 100)
	seedHandlerCommanderItem(t, client, 20001, 1)

	getPayload, _ := proto.Marshal(&protobuf.CS_21016{ShopId: proto.Uint32(10)})
	client.Buffer.Reset()
	if _, _, err := IslandShopGetData(&getPayload, client); err != nil {
		t.Fatalf("shop get: %v", err)
	}

	buyPayload, _ := proto.Marshal(&protobuf.CS_21018{GoodsList: []*protobuf.KVDATA2{
		{Key: proto.Uint32(10), Value1: proto.Uint32(1001), Value2: proto.Uint32(1)},
		{Key: proto.Uint32(10), Value1: proto.Uint32(1002), Value2: proto.Uint32(1)},
	}})
	client.Buffer.Reset()
	if _, _, err := IslandShopPurchase(&buyPayload, client); err != nil {
		t.Fatalf("shop purchase: %v", err)
	}
	var response protobuf.SC_21019
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected purchase failure when one currency is insufficient")
	}

	resourceAfter := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(1))
	if resourceAfter != 100 {
		t.Fatalf("expected resource rollback in DB, got %d", resourceAfter)
	}
	itemAfter := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(20001))
	if itemAfter != 1 {
		t.Fatalf("expected item rollback in DB, got %d", itemAfter)
	}
	state, err := orm.GetIslandShopState(client.Commander.CommanderID, 10)
	if err != nil {
		t.Fatalf("load shop state: %v", err)
	}
	for _, goods := range state.Goods {
		if goods.Num != 0 {
			t.Fatalf("expected shop purchase counts unchanged, got %+v", state.Goods)
		}
	}
}

func TestIslandUseItemGrantsDropsAndPT(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	seedConfigEntry(t, islandItemTemplateCategory, "9401", `{"id":9401,"pt_num":4,"usage_arg":[[41,9501,1],[1,1,2]]}`)
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 9401, 2)
	})
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21026{Id: proto.Uint32(9401), Count: proto.Uint32(2)})
	client.Buffer.Reset()
	if _, _, err := IslandUseItem(&payload, client); err != nil {
		t.Fatalf("island use item: %v", err)
	}
	var response protobuf.SC_21027
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 2 {
		t.Fatalf("unexpected use-item response: %+v", response)
	}
	if client.Commander.GetResourceCount(1) != 4 {
		t.Fatalf("expected granted resource 4, got %d", client.Commander.GetResourceCount(1))
	}
	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 9501)
	if err != nil || item.Count != 2 {
		t.Fatalf("expected granted island item, err=%v item=%+v", err, item)
	}
}

func TestIslandUseItemRollsBackWhenTemplateMissing(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 9402, 2)
	})
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21026{Id: proto.Uint32(9402), Count: proto.Uint32(1)})
	client.Buffer.Reset()
	if _, _, err := IslandUseItem(&payload, client); err != nil {
		t.Fatalf("island use item: %v", err)
	}
	var response protobuf.SC_21027
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure when item template is missing")
	}
	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 9402)
	if err != nil || item.Count != 2 {
		t.Fatalf("expected inventory rollback on missing template, err=%v item=%+v", err, item)
	}
}

func TestIslandOrderSyncRejectsUnknownType(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandCoreChainTables(t)
	payload, _ := proto.Marshal(&protobuf.CS_21024{Type: proto.Uint32(99)})
	client.Buffer.Reset()
	if _, _, err := IslandOrderSync(&payload, client); err != nil {
		t.Fatalf("order sync: %v", err)
	}
	var response protobuf.SC_21025
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected unknown type to fail")
	}
}

func clearIslandCoreChainTables(t *testing.T) {
	t.Helper()
	execAnswerTestSQLT(t, `
TRUNCATE TABLE
	config_entries,
	island_snapshots,
	island_inventories,
	island_overflow_inventories,
	island_shop_states,
	island_seasons,
	island_order_states,
	island_order_slots,
	island_ship_order_slots,
	island_ship_order_appoints,
	island_season_reward_claims
RESTART IDENTITY CASCADE
`)
}
