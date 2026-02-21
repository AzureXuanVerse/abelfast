package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandOpenRestaurantOK           = uint32(0)
	islandOpenRestaurantInvalid      = uint32(1)
	islandOpenRestaurantState        = uint32(2)
	islandOpenRestaurantInsufficient = uint32(3)
	islandOpenRestaurantPersist      = uint32(4)

	islandManageRestaurantCategory = "ShareCfg/island_manage_restaurant.json"
)

type islandManageRestaurantConfig struct {
	ID            uint32     `json:"id"`
	AssistantSlot []uint32   `json:"assistant_slot"`
	ItemID        [][]uint32 `json:"item_id"`
	OpeningTime   uint32     `json:"opening_time"`
}

var errIslandOpenRestaurantInsufficientRollback = errors.New("island open restaurant insufficient rollback")

func IslandOpenRestaurant(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21418
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21419, err
	}

	response := &protobuf.SC_21419{
		Result:    proto.Uint32(islandOpenRestaurantInvalid),
		TradeData: emptyIslandTrade(payload.GetTradeId()),
		ShipPower: []*protobuf.PB_ISLAND_SHIP_POWER{},
	}
	tradeID := payload.GetTradeId()
	if tradeID == 0 || payload.GetPresell() == nil || payload.GetPresell().GetTradeId() != tradeID {
		return client.SendMessage(21419, response)
	}
	if err := ensureCommanderLoaded(client, "Island/OpenRestaurant"); err != nil {
		response.Result = proto.Uint32(islandOpenRestaurantPersist)
		return client.SendMessage(21419, response)
	}

	restaurantCfg, found, err := loadIslandManageRestaurantConfig(tradeID)
	if err != nil {
		response.Result = proto.Uint32(islandOpenRestaurantPersist)
		return client.SendMessage(21419, response)
	}
	if !found {
		return client.SendMessage(21419, response)
	}

	postList, shipPowerList, postsOK := normalizeRestaurantPosts(payload.GetPostList(), restaurantCfg.AssistantSlot)
	if !postsOK {
		return client.SendMessage(21419, response)
	}
	foodList, sellList, totalSell, foodsOK := normalizeRestaurantFoods(payload.GetFoodList(), restaurantCfg.ItemID)
	if !foodsOK {
		return client.SendMessage(21419, response)
	}

	now := uint32(time.Now().Unix())
	endTime := now + restaurantCfg.OpeningTime
	if restaurantCfg.OpeningTime == 0 {
		endTime = now
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		trade, presell, totalSales, err := orm.GetIslandManageTradeForUpdateTx(context.Background(), tx, client.Commander.CommanderID, tradeID)
		if err != nil && !db.IsNotFound(err) {
			response.Result = proto.Uint32(islandOpenRestaurantPersist)
			return err
		}
		if err == nil && trade.GetEndTime() > now {
			response.Result = proto.Uint32(islandOpenRestaurantState)
			return nil
		}
		if err == nil && presell != nil {
			_ = totalSales
		}

		for _, food := range foodList {
			err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, food.GetFoodId(), food.GetNum())
			if err != nil {
				if errors.Is(err, orm.ErrInsufficientIslandInventory) {
					response.Result = proto.Uint32(islandOpenRestaurantInsufficient)
					return errIslandOpenRestaurantInsufficientRollback
				}
				response.Result = proto.Uint32(islandOpenRestaurantPersist)
				return err
			}
		}

		tradeData := &protobuf.PB_ISLAND_TRADE{
			Id:        proto.Uint32(tradeID),
			Lv:        proto.Uint32(1),
			TotalSell: proto.Uint32(totalSell),
			SellList:  sellList,
			RestList:  foodList,
			PostList:  postList,
			EndTime:   proto.Uint32(endTime),
			SpeedTime: proto.Uint32(0),
		}
		if err := orm.UpsertIslandManageTradeTx(context.Background(), tx, client.Commander.CommanderID, tradeData, payload.GetPresell(), 0); err != nil {
			response.Result = proto.Uint32(islandOpenRestaurantPersist)
			return err
		}
		if err := orm.UpsertIslandSpeedupTargetTx(context.Background(), tx, client.Commander.CommanderID, islandTicketTypeManage, tradeID, endTime); err != nil {
			response.Result = proto.Uint32(islandOpenRestaurantPersist)
			return err
		}

		response.Result = proto.Uint32(islandOpenRestaurantOK)
		response.TradeData = tradeData
		response.ShipPower = shipPowerList
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandOpenRestaurantInsufficientRollback) {
			return client.SendMessage(21419, response)
		}
		return client.SendMessage(21419, response)
	}

	return client.SendMessage(21419, response)
}

func loadIslandManageRestaurantConfig(tradeID uint32) (*islandManageRestaurantConfig, bool, error) {
	key := fmt.Sprintf("%d", tradeID)
	if entry, err := orm.GetConfigEntry(islandManageRestaurantCategory, key); err == nil {
		var single islandManageRestaurantConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == 0 {
				single.ID = tradeID
			}
			return &single, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandManageRestaurantCategory)
	if err != nil {
		return nil, false, err
	}
	for _, entry := range entries {
		var single islandManageRestaurantConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == tradeID {
				return &single, true, nil
			}
			continue
		}
		var list []islandManageRestaurantConfig
		if err := json.Unmarshal(entry.Data, &list); err != nil {
			return nil, false, err
		}
		for i := range list {
			if list[i].ID == tradeID {
				return &list[i], true, nil
			}
		}
	}
	return nil, false, nil
}

func normalizeRestaurantPosts(input []*protobuf.PB_TRADE_POST, allowedPosts []uint32) ([]*protobuf.PB_TRADE_POST, []*protobuf.PB_ISLAND_SHIP_POWER, bool) {
	if len(input) == 0 {
		return nil, nil, false
	}
	allowed := make(map[uint32]struct{}, len(allowedPosts))
	for _, postID := range allowedPosts {
		allowed[postID] = struct{}{}
	}
	seenPosts := make(map[uint32]struct{}, len(input))
	seenShips := make(map[uint32]struct{}, len(input))
	postList := make([]*protobuf.PB_TRADE_POST, 0, len(input))
	shipPowers := make([]*protobuf.PB_ISLAND_SHIP_POWER, 0, len(input))
	for _, post := range input {
		postID := post.GetPostId()
		shipID := post.GetShipId()
		if postID == 0 || shipID == 0 {
			return nil, nil, false
		}
		if _, ok := allowed[postID]; !ok {
			return nil, nil, false
		}
		if _, exists := seenPosts[postID]; exists {
			return nil, nil, false
		}
		if _, exists := seenShips[shipID]; exists {
			return nil, nil, false
		}
		seenPosts[postID] = struct{}{}
		seenShips[shipID] = struct{}{}
		postList = append(postList, &protobuf.PB_TRADE_POST{PostId: proto.Uint32(postID), ShipId: proto.Uint32(shipID)})
		shipPowers = append(shipPowers, &protobuf.PB_ISLAND_SHIP_POWER{ShipId: proto.Uint32(shipID), Power: proto.Uint32(100)})
	}
	return postList, shipPowers, true
}

func normalizeRestaurantFoods(input []*protobuf.PB_TRADE_FOOD, configuredItems [][]uint32) ([]*protobuf.PB_TRADE_FOOD, []*protobuf.PB_TRADE_SELL_FOOD, uint32, bool) {
	if len(input) == 0 {
		return nil, nil, 0, false
	}
	allowedFoodIDs := make(map[uint32]struct{}, len(configuredItems))
	for _, row := range configuredItems {
		if len(row) > 0 {
			allowedFoodIDs[row[0]] = struct{}{}
		}
	}
	aggregated := make(map[uint32]uint32, len(input))
	for _, food := range input {
		foodID := food.GetFoodId()
		num := food.GetNum()
		if num == 0 {
			return nil, nil, 0, false
		}
		if _, ok := allowedFoodIDs[foodID]; !ok {
			return nil, nil, 0, false
		}
		aggregated[foodID] += num
	}

	foodList := make([]*protobuf.PB_TRADE_FOOD, 0, len(aggregated))
	sellList := make([]*protobuf.PB_TRADE_SELL_FOOD, 0, len(aggregated))
	totalSell := uint32(0)
	for foodID, num := range aggregated {
		foodList = append(foodList, &protobuf.PB_TRADE_FOOD{FoodId: proto.Uint32(foodID), Num: proto.Uint32(num)})
		sellList = append(sellList, &protobuf.PB_TRADE_SELL_FOOD{FoodId: proto.Uint32(foodID), Num: proto.Uint32(num), SellMoney: proto.Uint32(num)})
		totalSell += num
	}
	return foodList, sellList, totalSell, true
}

func emptyIslandTrade(tradeID uint32) *protobuf.PB_ISLAND_TRADE {
	return &protobuf.PB_ISLAND_TRADE{
		Id:        proto.Uint32(tradeID),
		Lv:        proto.Uint32(0),
		TotalSell: proto.Uint32(0),
		SellList:  []*protobuf.PB_TRADE_SELL_FOOD{},
		RestList:  []*protobuf.PB_TRADE_FOOD{},
		PostList:  []*protobuf.PB_TRADE_POST{},
		EndTime:   proto.Uint32(0),
		SpeedTime: proto.Uint32(0),
	}
}
