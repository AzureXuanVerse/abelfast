package answer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type islandAgoraExpansionCost struct {
	DropType uint32
	DropID   uint32
	Count    uint32
}

func IslandUpgradeAgora(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21305
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21306, err
	}

	response := &protobuf.SC_21306{Result: proto.Uint32(1)}
	if payload.GetType() != 0 {
		return client.SendMessage(21306, response)
	}
	if err := ensureCommanderLoaded(client, "Island/UpgradeAgora"); err != nil {
		return client.SendMessage(21306, response)
	}

	expansionCosts, err := loadIslandAgoraExpansionCosts()
	if err != nil || len(expansionCosts) == 0 {
		return client.SendMessage(21306, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
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

		currentLevel := maxUint32(snapshot.AgoraLevel, 1)
		if int(currentLevel) > len(expansionCosts) {
			return nil
		}
		cost := expansionCosts[currentLevel-1]
		if cost.Count == 0 || cost.DropID == 0 {
			return nil
		}

		switch cost.DropType {
		case consts.DROP_TYPE_ISLAND_ITEM:
			if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, cost.DropID, cost.Count); err != nil {
				return nil
			}
		case consts.DROP_TYPE_ITEM:
			consumed, err := consumeCommanderItemTx(context.Background(), tx, client.Commander.CommanderID, cost.DropID, cost.Count)
			if err != nil || !consumed {
				return nil
			}
		case consts.DROP_TYPE_RESOURCE:
			if err := client.Commander.ConsumeResourceTx(context.Background(), tx, cost.DropID, cost.Count); err != nil {
				return nil
			}
		default:
			return nil
		}

		snapshot.AgoraLevel = currentLevel + 1
		if err := orm.UpsertIslandSnapshotTx(context.Background(), tx, snapshot); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}

	return client.SendMessage(21306, response)
}

func loadIslandAgoraExpansionCosts() ([]islandAgoraExpansionCost, error) {
	entries, err := listConfigEntriesWithFallback(islandSetCategory, islandSetCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].Key != "island_build_expansion" {
			continue
		}
		costs, err := parseIslandAgoraExpansionCosts(entries[i].Data)
		if err != nil {
			return nil, err
		}
		if len(costs) > 0 {
			return costs, nil
		}
	}
	return nil, nil
}

func parseIslandAgoraExpansionCosts(raw json.RawMessage) ([]islandAgoraExpansionCost, error) {
	var entry struct {
		KeyValueRaw json.RawMessage `json:"key_value_varchar"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, err
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(entry.KeyValueRaw, &rows); err != nil {
		return nil, err
	}

	costs := make([]islandAgoraExpansionCost, 0, len(rows))
	for i := range rows {
		var row []json.RawMessage
		if err := json.Unmarshal(rows[i], &row); err != nil || len(row) < 2 {
			continue
		}

		var level uint32
		if err := json.Unmarshal(row[0], &level); err != nil || level == 0 {
			continue
		}

		var costParts []uint32
		if err := json.Unmarshal(row[1], &costParts); err != nil || len(costParts) < 3 {
			continue
		}

		idx := level - 1
		if int(idx) >= len(costs) {
			costs = append(costs, make([]islandAgoraExpansionCost, int(idx)-len(costs)+1)...)
		}
		costs[idx] = islandAgoraExpansionCost{DropType: costParts[0], DropID: costParts[1], Count: costParts[2]}
	}
	return costs, nil
}
