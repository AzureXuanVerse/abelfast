package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSellOrConvertItems(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21014
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21015, err
	}

	response := &protobuf.SC_21015{Result: proto.Uint32(1), ItemList: []*protobuf.PB_ISLAND_ITEM{}}
	if payload.GetType() != 1 && payload.GetType() != 2 {
		return client.SendMessage(21015, response)
	}

	consumes := make(map[uint32]uint32)
	for _, item := range payload.GetItemList() {
		if item == nil || item.GetId() == 0 || item.GetNum() == 0 {
			continue
		}
		consumes[item.GetId()] += item.GetNum()
	}
	if len(consumes) == 0 {
		return client.SendMessage(21015, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		converted := make(map[uint32]uint32)
		addedPT := uint32(0)

		for itemID, count := range consumes {
			cfg, found, err := loadIslandItemTemplate(itemID)
			if err != nil || !found {
				return nil
			}
			if payload.GetType() == 1 {
				if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, itemID, count); err != nil {
					return nil
				}
			} else {
				if err := orm.ConsumeIslandOverflowInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, itemID, count); err != nil {
					return nil
				}
			}

			if cfg.Convert > 0 {
				converted[cfg.Convert] += count
				if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, cfg.Convert, count); err != nil {
					return err
				}
			}
			if cfg.PTNum > 0 {
				addedPT += cfg.PTNum * count
			}
		}

		if addedPT > 0 {
			if err := orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, addedPT); err != nil {
				return err
			}
		}

		for itemID, count := range converted {
			response.ItemList = append(response.ItemList, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(itemID), Num: proto.Uint32(count)})
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(21015, response)
}
