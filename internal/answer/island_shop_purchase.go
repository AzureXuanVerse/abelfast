package answer

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

type islandPurchaseRequest struct {
	ShopID  uint32
	GoodsID uint32
	Count   uint32
}

func IslandShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21018
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21019, err
	}

	response := &protobuf.SC_21019{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/ShopPurchase"); err != nil {
		return client.SendMessage(21019, response)
	}

	requests := make([]islandPurchaseRequest, 0, len(payload.GetGoodsList()))
	for _, item := range payload.GetGoodsList() {
		if item == nil || item.GetKey() == 0 || item.GetValue1() == 0 || item.GetValue2() == 0 {
			continue
		}
		requests = append(requests, islandPurchaseRequest{ShopID: item.GetKey(), GoodsID: item.GetValue1(), Count: item.GetValue2()})
	}
	if len(requests) == 0 {
		return client.SendMessage(21019, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		type costKey struct {
			DropType uint32
			DropID   uint32
		}
		costs := make(map[costKey]uint32)
		drops := make([]*protobuf.DROPINFO, 0)
		seasonPT := uint32(0)

		shopStates := make(map[uint32]*orm.IslandShopState)
		for _, req := range requests {
			state := shopStates[req.ShopID]
			if state == nil {
				loaded, err := orm.GetIslandShopState(client.Commander.CommanderID, req.ShopID)
				if err != nil {
					return nil
				}
				shopStates[req.ShopID] = loaded
				state = loaded
			}

			cfg, found, err := loadIslandShopGoodsTemplate(req.GoodsID)
			if err != nil || !found {
				return nil
			}
			if cfg.PayID != 0 {
				return nil
			}

			goodsIndex := -1
			for i := range state.Goods {
				if state.Goods[i].ID == req.GoodsID {
					goodsIndex = i
					break
				}
			}
			if goodsIndex < 0 {
				return nil
			}
			if cfg.LimitedNum > 0 && state.Goods[goodsIndex].Num+req.Count > cfg.LimitedNum {
				return nil
			}

			state.Goods[goodsIndex].Num += req.Count
			if len(cfg.ResourceConsume) >= 3 {
				ck := costKey{DropType: cfg.ResourceConsume[0], DropID: cfg.ResourceConsume[1]}
				costs[ck] += cfg.ResourceConsume[2] * req.Count
			}
			for _, item := range cfg.Items {
				if len(item) < 3 {
					continue
				}
				drops = append(drops, newDropInfo(item[0], item[1], item[2]*req.Count))
			}
			seasonPT += cfg.PTAward * req.Count
		}

		for key, count := range costs {
			switch key.DropType {
			case consts.DROP_TYPE_RESOURCE:
				if err := client.Commander.ConsumeResourceTx(context.Background(), tx, key.DropID, count); err != nil {
					return nil
				}
			case consts.DROP_TYPE_ITEM:
				if err := client.Commander.ConsumeItemTx(context.Background(), tx, key.DropID, count); err != nil {
					return nil
				}
			case consts.DROP_TYPE_ISLAND_ITEM:
				if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, key.DropID, count); err != nil {
					return nil
				}
			default:
				return nil
			}
		}

		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}
		if seasonPT > 0 {
			if err := orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, seasonPT); err != nil {
				return err
			}
		}
		for _, state := range shopStates {
			if err := orm.UpsertIslandShopStateTx(context.Background(), tx, state); err != nil {
				return err
			}
		}

		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}

	return client.SendMessage(21019, response)
}
