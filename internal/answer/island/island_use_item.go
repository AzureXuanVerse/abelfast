package island

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

var errIslandUseItemRollback = errors.New("island use item rollback")

func IslandUseItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21026
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21027, err
	}

	response := &protobuf.SC_21027{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}, ShipList: []*protobuf.PB_ISLAND_SHIP{}}
	itemID := payload.GetId()
	count := payload.GetCount()
	if itemID == 0 || count == 0 {
		return client.SendMessage(21027, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, itemID, count); err != nil {
			return errIslandUseItemRollback
		}
		cfg, found, err := loadIslandItemTemplate(itemID)
		if err != nil {
			return err
		}
		if !found {
			return errIslandUseItemRollback
		}

		drops := make([]*protobuf.DROPINFO, 0)
		if cfg.Convert > 0 {
			drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, cfg.Convert, count))
		}
		usageDrops, err := decodeIslandUsageAward(cfg.UsageArg)
		if err == nil {
			for _, entry := range usageDrops {
				if len(entry) < 3 {
					continue
				}
				drops = append(drops, newDropInfo(entry[0], entry[1], entry[2]*count))
			}
		}

		if len(drops) > 0 {
			if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
				return err
			}
		}
		if cfg.PTNum > 0 {
			if err := orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, cfg.PTNum*count); err != nil {
				return err
			}
		}

		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
		_ = client.Commander.Load()
	}

	return client.SendMessage(21027, response)
}
