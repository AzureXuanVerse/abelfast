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

func TestIslandOpenAndCloseRestaurantFlow(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandManageRestaurantCategory, "601", `{"id":601,"assistant_slot":[5,6],"item_id":[[3011,601001],[3012,601002]],"opening_time":300}`)

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 3011, 10)
	})
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	openPayload := protobuf.CS_21418{
		TradeId:  proto.Uint32(601),
		PostList: []*protobuf.PB_TRADE_POST{{PostId: proto.Uint32(5), ShipId: proto.Uint32(11)}},
		FoodList: []*protobuf.PB_TRADE_FOOD{{FoodId: proto.Uint32(3011), Num: proto.Uint32(3)}},
		Presell:  &protobuf.PB_TRADE_PRESELL{TradeId: proto.Uint32(601), SellNumMin: proto.Uint32(1), SellNumMax: proto.Uint32(3), SellMoneyMin: proto.Uint32(1), SellMoneyMax: proto.Uint32(3)},
	}
	openBuffer, _ := proto.Marshal(&openPayload)
	client.Buffer.Reset()
	if _, _, err := IslandOpenRestaurant(&openBuffer, client); err != nil {
		t.Fatalf("open restaurant: %v", err)
	}
	var openResponse protobuf.SC_21419
	decodeResponse(t, client, &openResponse)
	if openResponse.GetResult() != 0 || openResponse.GetTradeData().GetId() != 601 || openResponse.GetTradeData().GetEndTime() == 0 || len(openResponse.GetShipPower()) != 1 {
		t.Fatalf("unexpected open response: %+v", openResponse)
	}

	closePayload := protobuf.CS_21420{TradeId: proto.Uint32(601)}
	closeBuffer, _ := proto.Marshal(&closePayload)
	client.Buffer.Reset()
	if _, _, err := IslandCloseRestaurant(&closeBuffer, client); err != nil {
		t.Fatalf("close restaurant: %v", err)
	}
	var closeResponse protobuf.SC_21421
	decodeResponse(t, client, &closeResponse)
	if closeResponse.GetResult() != 0 || len(closeResponse.GetDropList()) == 0 {
		t.Fatalf("unexpected close response: %+v", closeResponse)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		trade, _, _, err := orm.GetIslandManageTradeForUpdateTx(context.Background(), tx, client.Commander.CommanderID, 601)
		if err != nil {
			return err
		}
		if trade.GetEndTime() != 0 || len(trade.GetSellList()) != 0 || len(trade.GetRestList()) != 0 || len(trade.GetPostList()) != 0 {
			t.Fatalf("expected trade reset after close, got %+v", trade)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("verify closed trade state: %v", err)
	}

	if speedupTargetExists(t, client.Commander.CommanderID, islandTicketTypeManage, 601) {
		t.Fatalf("expected speedup target cleanup after close")
	}
}

func TestIslandOpenRestaurantRejectsInvalidFood(t *testing.T) {
	client := setupHandlerCommander(t)
	clearIslandEconomyTables(t)
	seedConfigEntry(t, islandManageRestaurantCategory, "601", `{"id":601,"assistant_slot":[5,6],"item_id":[[3011,601001]],"opening_time":300}`)

	payload := protobuf.CS_21418{
		TradeId:  proto.Uint32(601),
		PostList: []*protobuf.PB_TRADE_POST{{PostId: proto.Uint32(5), ShipId: proto.Uint32(11)}},
		FoodList: []*protobuf.PB_TRADE_FOOD{{FoodId: proto.Uint32(9999), Num: proto.Uint32(1)}},
		Presell:  &protobuf.PB_TRADE_PRESELL{TradeId: proto.Uint32(601), SellNumMin: proto.Uint32(1), SellNumMax: proto.Uint32(1), SellMoneyMin: proto.Uint32(1), SellMoneyMax: proto.Uint32(1)},
	}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := IslandOpenRestaurant(&buffer, client); err != nil {
		t.Fatalf("open restaurant invalid food: %v", err)
	}

	var response protobuf.SC_21419
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid food to fail")
	}
}

func speedupTargetExists(t *testing.T, commanderID uint32, targetType uint32, targetID uint32) bool {
	t.Helper()
	var count int64
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT count(*)
FROM island_speedup_targets
WHERE commander_id = $1 AND target_type = $2 AND target_id = $3
`, int64(commanderID), int64(targetType), int64(targetID)).Scan(&count)
	if err != nil {
		t.Fatalf("query speedup target: %v", err)
	}
	return count > 0
}
