package educate

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27033
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27034, err
	}

	response := &protobuf.SC_27034{Result: proto.Uint32(educateResultFailed), Drops: []*protobuf.CHILD_DROP{}}
	if client.Commander == nil || payload.GetShopId() == 0 || len(payload.GetGoods()) == 0 {
		return client.SendMessage(27034, response)
	}

	shops, templates, err := loadEducateShopConfigs()
	if err != nil {
		return 0, 27034, err
	}
	shop, ok := shops[payload.GetShopId()]
	if !ok {
		return client.SendMessage(27034, response)
	}

	now := educateNow()
	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := ensureEducateShopStateTx(ctx, tx, client.Commander.CommanderID, shop, templates, now)
		if err != nil {
			return err
		}
		index := make(map[uint32]int, len(state.Goods))
		for i, row := range state.Goods {
			index[row.ID] = i
		}

		resourceCost := map[uint32]uint32{}
		drops := make([]*protobuf.CHILD_DROP, 0, len(payload.GetGoods()))
		for _, row := range payload.GetGoods() {
			if row == nil || row.GetId() == 0 || row.GetNum() == 0 {
				return nil
			}
			goodIndex, exists := index[row.GetId()]
			if !exists {
				return nil
			}
			if state.Goods[goodIndex].Num < row.GetNum() {
				return nil
			}
			tpl, ok := templates[row.GetId()]
			if !ok || tpl.Resource == 0 {
				return nil
			}
			resourceCost[tpl.Resource] += tpl.ResourceNum * row.GetNum()
			state.Goods[goodIndex].Num -= row.GetNum()
			n := int32(tpl.BuyNum * row.GetNum())
			drops = append(drops, &protobuf.CHILD_DROP{Type: proto.Uint32(2), Id: proto.Uint32(tpl.ItemID), Number: proto.Int32(n)})
		}

		if len(drops) == 0 {
			return nil
		}
		for resourceID, amount := range resourceCost {
			if !client.Commander.HasEnoughResource(resourceID, amount) {
				return nil
			}
		}
		for resourceID, amount := range resourceCost {
			if err := client.Commander.ConsumeResourceTx(ctx, tx, resourceID, amount); err != nil {
				return err
			}
		}
		for _, drop := range drops {
			if err := applyEducateChildDropTx(ctx, tx, client, drop); err != nil {
				return err
			}
		}
		if err := orm.UpsertEducateShopStateTx(ctx, tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(educateResultOK)
		response.Drops = drops
		return nil
	})
	if err != nil {
		return 0, 27034, err
	}

	return client.SendMessage(27034, response)
}
