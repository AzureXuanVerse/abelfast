package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandCloseRestaurantOK      = uint32(0)
	islandCloseRestaurantInvalid = uint32(1)
	islandCloseRestaurantState   = uint32(2)
	islandCloseRestaurantPersist = uint32(3)
)

func IslandCloseRestaurant(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21420
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21421, err
	}

	response := &protobuf.SC_21421{Result: proto.Uint32(islandCloseRestaurantInvalid), DropList: []*protobuf.DROPINFO{}}
	tradeID := payload.GetTradeId()
	if tradeID == 0 {
		return client.SendMessage(21421, response)
	}
	if err := ensureCommanderLoaded(client, "Island/CloseRestaurant"); err != nil {
		response.Result = proto.Uint32(islandCloseRestaurantPersist)
		return client.SendMessage(21421, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		trade, presell, totalSales, err := orm.GetIslandManageTradeForUpdateTx(context.Background(), tx, client.Commander.CommanderID, tradeID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandCloseRestaurantState)
				return nil
			}
			response.Result = proto.Uint32(islandCloseRestaurantPersist)
			return err
		}
		if trade.GetEndTime() == 0 {
			response.Result = proto.Uint32(islandCloseRestaurantState)
			return nil
		}

		drops := make([]*protobuf.DROPINFO, 0, len(trade.GetSellList()))
		soldCount := uint32(0)
		for _, food := range trade.GetSellList() {
			if food.GetFoodId() == 0 || food.GetNum() == 0 {
				continue
			}
			drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, food.GetFoodId(), food.GetNum()))
			soldCount += food.GetNum()
		}
		if len(drops) > 0 {
			if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
				response.Result = proto.Uint32(islandCloseRestaurantPersist)
				return err
			}
		}

		trade.TotalSell = proto.Uint32(totalSales + soldCount)
		trade.SellList = []*protobuf.PB_TRADE_SELL_FOOD{}
		trade.RestList = []*protobuf.PB_TRADE_FOOD{}
		trade.PostList = []*protobuf.PB_TRADE_POST{}
		trade.EndTime = proto.Uint32(0)
		trade.SpeedTime = proto.Uint32(0)
		if err := orm.UpsertIslandManageTradeTx(context.Background(), tx, client.Commander.CommanderID, trade, presell, totalSales+soldCount); err != nil {
			response.Result = proto.Uint32(islandCloseRestaurantPersist)
			return err
		}
		if err := orm.DeleteIslandSpeedupTargetTx(context.Background(), tx, client.Commander.CommanderID, islandTicketTypeManage, tradeID); err != nil {
			response.Result = proto.Uint32(islandCloseRestaurantPersist)
			return err
		}

		response.Result = proto.Uint32(islandCloseRestaurantOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21421, response)
	}

	return client.SendMessage(21421, response)
}
