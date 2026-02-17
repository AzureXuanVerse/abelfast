package answer

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21000
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21001, err
	}

	response := &protobuf.SC_21001{Ret: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if payload.GetType() != 0 {
		return client.SendMessage(21001, response)
	}
	if err := ensureCommanderLoaded(client, "Island/Upgrade"); err != nil {
		return client.SendMessage(21001, response)
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

		level := maxUint32(snapshot.Level, 1)
		currentCfg, found, err := loadIslandLevelTemplate(level)
		if err != nil || !found {
			return nil
		}
		if _, found, err := loadIslandLevelTemplate(level + 1); err != nil || !found {
			return nil
		}
		if snapshot.Exp < currentCfg.IslandExp {
			return nil
		}

		for _, cost := range currentCfg.Cost {
			if len(cost) < 2 || cost[1] == 0 {
				continue
			}
			if err := client.Commander.ConsumeResourceTx(context.Background(), tx, cost[0], cost[1]); err != nil {
				return nil
			}
		}

		snapshot.Level = level + 1
		snapshot.Exp -= currentCfg.IslandExp
		drops, err := buildAwardDrops(currentCfg.IslandLevelAward)
		if err != nil {
			return nil
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}
		if err := orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot); err != nil {
			return err
		}

		response.Ret = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		response.Ret = proto.Uint32(1)
	}
	return client.SendMessage(21001, response)
}
