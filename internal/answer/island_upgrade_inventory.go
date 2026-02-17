package answer

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandUpgradeInventory(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21012
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21013, err
	}

	response := &protobuf.SC_21013{Ret: proto.Uint32(1)}
	if payload.GetType() != 0 {
		return client.SendMessage(21013, response)
	}
	if err := ensureCommanderLoaded(client, "Island/UpgradeInventory"); err != nil {
		return client.SendMessage(21013, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		snapshot, err := orm.GetIslandSnapshotForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
			snapshot = defaultIslandSnapshot(client.Commander.CommanderID)
			snapshot.DailyTimestamp = uint32(time.Now().UTC().Unix())
			if err := orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot); err != nil {
				return err
			}
			snapshot, err = orm.GetIslandSnapshotForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
			if err != nil {
				return err
			}
		}

		level := maxUint32(snapshot.StorageLevel, 1)
		cfg, found, err := loadIslandStorageLevelTemplate(level)
		if err != nil || !found {
			return nil
		}
		if _, found, err := loadIslandStorageLevelTemplate(level + 1); err != nil || !found {
			return nil
		}

		for _, material := range cfg.UpgradeMaterial {
			if len(material) < 2 || material[1] == 0 {
				continue
			}
			dropType := consts.DROP_TYPE_ISLAND_ITEM
			dropID := material[0]
			count := material[1]
			if len(material) >= 3 {
				dropType = material[0]
				dropID = material[1]
				count = material[2]
			}
			switch dropType {
			case consts.DROP_TYPE_ISLAND_ITEM:
				if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, dropID, count); err != nil {
					return nil
				}
			case consts.DROP_TYPE_RESOURCE:
				if err := client.Commander.ConsumeResourceTx(context.Background(), tx, dropID, count); err != nil {
					return nil
				}
			case consts.DROP_TYPE_ITEM:
				if err := client.Commander.ConsumeItemTx(context.Background(), tx, dropID, count); err != nil {
					return nil
				}
			default:
				return nil
			}
		}

		snapshot.StorageLevel = level + 1
		if err := orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot); err != nil {
			return err
		}
		response.Ret = proto.Uint32(0)
		return nil
	})
	if err != nil {
		response.Ret = proto.Uint32(1)
	}
	return client.SendMessage(21013, response)
}
