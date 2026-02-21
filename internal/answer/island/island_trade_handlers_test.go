package island

import (
	"strconv"
	"testing"

	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedIslandTradeConfig(t *testing.T, buyLimit uint32, sellLimit uint32, buyPercent uint32, initialPrice uint32) {
	t.Helper()
	seedConfigEntry(t, islandSetCategory, islandTradeConfigLimitKey, `{"key":"treasure_week_limit","key_value":[`+uintToString(buyLimit)+`,`+uintToString(sellLimit)+`]}`)
	seedConfigEntry(t, islandSetCategory, islandTradeConfigBuyKey, `{"key":"treasure_price_buy","key_value_int":`+uintToString(buyPercent)+`}`)
	seedConfigEntry(t, islandSetCategory, islandTradeConfigInitKey, `{"key":"treasure_price_initial","key_value_int":`+uintToString(initialPrice)+`}`)
}

func TestIslandTradeOpPurchaseSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandTreasureState{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandTradeConfig(t, 10, 6, 80, 100)

	if err := orm.UpsertIslandTreasureState(&orm.IslandTreasureState{
		CommanderID: client.Commander.CommanderID,
		PriceList:   []orm.IslandTreasurePriceState{{Timestamp: 1000, Price: 100}},
	}); err != nil {
		t.Fatalf("seed treasure state: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(islandTradeGoldItemID), int64(500))

	payload, _ := proto.Marshal(&protobuf.CS_21240{IslandId: proto.Uint32(client.Commander.CommanderID), Type: proto.Uint32(islandTradePurchaseType), Num: proto.Uint32(2)})
	if _, _, err := IslandTradeOp(&payload, client); err != nil {
		t.Fatalf("trade purchase failed: %v", err)
	}

	var response protobuf.SC_21241
	decodePacketAt(t, client, 0, 21241, &response)
	if response.GetResult() != islandTradeResultOK {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetType() != consts.DROP_TYPE_ISLAND_ITEM || response.GetDropList()[0].GetId() != islandTradePearlItemID || response.GetDropList()[0].GetNumber() != 2 {
		t.Fatalf("unexpected purchase drops: %+v", response.GetDropList())
	}

	gold := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(islandTradeGoldItemID))
	if gold != 340 {
		t.Fatalf("expected gold 340 after purchase, got %d", gold)
	}
	pearls := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(islandTradePearlItemID))
	if pearls != 2 {
		t.Fatalf("expected pearls 2 after purchase, got %d", pearls)
	}
	state, err := orm.GetIslandTreasureState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.WeekBuyNum != 2 {
		t.Fatalf("expected week buy num 2, got %d", state.WeekBuyNum)
	}
}

func TestIslandTradeOpSellUpdatesForeignSellLimit(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandTreasureState{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandTradeConfig(t, 10, 3, 80, 120)

	if err := orm.UpsertIslandTreasureState(&orm.IslandTreasureState{
		CommanderID: client.Commander.CommanderID,
		PriceList:   []orm.IslandTreasurePriceState{{Timestamp: 2000, Price: 120}},
	}); err != nil {
		t.Fatalf("seed treasure state: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(islandTradePearlItemID), int64(6))

	payload, _ := proto.Marshal(&protobuf.CS_21240{IslandId: proto.Uint32(7788), Type: proto.Uint32(islandTradeSellType), Num: proto.Uint32(2)})
	if _, _, err := IslandTradeOp(&payload, client); err != nil {
		t.Fatalf("trade sell failed: %v", err)
	}

	var response protobuf.SC_21241
	decodePacketAt(t, client, 0, 21241, &response)
	if response.GetResult() != islandTradeResultOK {
		t.Fatalf("expected sell success result, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != islandTradeGoldItemID || response.GetDropList()[0].GetNumber() != 240 {
		t.Fatalf("unexpected sell drops: %+v", response.GetDropList())
	}
	pearls := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(islandTradePearlItemID))
	if pearls != 4 {
		t.Fatalf("expected pearls 4 after sell, got %d", pearls)
	}
	gold := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(islandTradeGoldItemID))
	if gold != 240 {
		t.Fatalf("expected gold 240 after sell, got %d", gold)
	}
	state, err := orm.GetIslandTreasureState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.SellCount(7788) != 2 {
		t.Fatalf("expected foreign island sell count 2, got %d", state.SellCount(7788))
	}
}

func TestIslandTradeOpValidationAndFailurePaths(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandTreasureState{})
	clearTable(t, &orm.IslandInventory{})
	seedIslandTradeConfig(t, 10, 1, 80, 90)

	payloadZeroNum, _ := proto.Marshal(&protobuf.CS_21240{IslandId: proto.Uint32(client.Commander.CommanderID), Type: proto.Uint32(islandTradePurchaseType), Num: proto.Uint32(0)})
	if _, _, err := IslandTradeOp(&payloadZeroNum, client); err != nil {
		t.Fatalf("zero num request should still ack: %v", err)
	}
	var invalidResponse protobuf.SC_21241
	decodePacketAt(t, client, 0, 21241, &invalidResponse)
	if invalidResponse.GetResult() != islandTradeResultInvalid {
		t.Fatalf("expected invalid result for num=0, got %d", invalidResponse.GetResult())
	}

	client.Buffer.Reset()
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(islandTradePearlItemID), int64(1))
	payloadLack, _ := proto.Marshal(&protobuf.CS_21240{IslandId: proto.Uint32(6611), Type: proto.Uint32(islandTradeSellType), Num: proto.Uint32(2)})
	if _, _, err := IslandTradeOp(&payloadLack, client); err != nil {
		t.Fatalf("lack request should still ack: %v", err)
	}
	var lackResponse protobuf.SC_21241
	decodePacketAt(t, client, 0, 21241, &lackResponse)
	if lackResponse.GetResult() != islandTradeResultLack {
		t.Fatalf("expected lack result, got %d", lackResponse.GetResult())
	}
}

func TestIslandGetFriendTradeRankReturnsSnapshotAndPrice(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandTreasureState{})
	seedIslandTradeConfig(t, 10, 5, 80, 70)

	targetID := client.Commander.CommanderID + 100
	if err := orm.CreateCommanderRoot(targetID, targetID, "Rank Target", 0, 0); err != nil {
		t.Fatalf("create target commander: %v", err)
	}
	execAnswerTestSQLT(t, "UPDATE island_snapshots SET level = $1 WHERE commander_id = $2", int64(15), int64(targetID))
	if err := orm.UpsertIslandTreasureState(&orm.IslandTreasureState{CommanderID: targetID, PriceList: []orm.IslandTreasurePriceState{{Timestamp: 5000, Price: 135}}}); err != nil {
		t.Fatalf("seed target treasure state: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21243{IslandId: proto.Uint32(targetID)})
	if _, _, err := IslandGetFriendTradeRank(&payload, client); err != nil {
		t.Fatalf("friend rank request failed: %v", err)
	}
	var response protobuf.SC_21244
	decodePacketAt(t, client, 0, 21244, &response)
	if response.GetIslandLv() != 15 {
		t.Fatalf("expected island level 15, got %d", response.GetIslandLv())
	}
	if response.GetTodayPrice() == nil || response.GetTodayPrice().GetPrice() != 135 {
		t.Fatalf("unexpected today price payload: %+v", response.GetTodayPrice())
	}

	client.Buffer.Reset()
	fallbackPayload, _ := proto.Marshal(&protobuf.CS_21243{IslandId: proto.Uint32(0)})
	if _, _, err := IslandGetFriendTradeRank(&fallbackPayload, client); err != nil {
		t.Fatalf("fallback rank request failed: %v", err)
	}
	var fallbackResponse protobuf.SC_21244
	decodePacketAt(t, client, 0, 21244, &fallbackResponse)
	if fallbackResponse.GetTodayPrice() == nil {
		t.Fatalf("expected non-nil fallback today price")
	}
}

func TestIslandGetDataIncludesTreasureState(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandTreasureState{})

	if err := orm.UpsertIslandTreasureState(&orm.IslandTreasureState{
		CommanderID: client.Commander.CommanderID,
		WeekBuyNum:  4,
		SellList:    []orm.IslandTreasureSellState{{IslandID: 4401, Num: 2}},
		PriceList:   []orm.IslandTreasurePriceState{{Timestamp: 9001, Price: 123}},
	}); err != nil {
		t.Fatalf("seed treasure state: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_21200{IslandId: proto.Uint32(client.Commander.CommanderID)})
	if _, _, err := IslandGetData(&payload, client); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}

	var response protobuf.SC_21201
	decodePacketAt(t, client, 0, 21201, &response)
	treasure := response.GetIsland().GetPublicData().GetTreasure()
	if treasure.GetWeekBuyNum() != 4 {
		t.Fatalf("expected week buy num 4, got %d", treasure.GetWeekBuyNum())
	}
	if len(treasure.GetSellList()) != 1 || treasure.GetSellList()[0].GetIslandId() != 4401 || treasure.GetSellList()[0].GetNum() != 2 {
		t.Fatalf("unexpected sell list payload: %+v", treasure.GetSellList())
	}
	if len(treasure.GetPriceList()) != 1 || treasure.GetPriceList()[0].GetPrice() != 123 {
		t.Fatalf("unexpected price list payload: %+v", treasure.GetPriceList())
	}
}

func uintToString(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}
