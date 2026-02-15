package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	activityTaskResultSuccess uint32 = 0
	activityTaskResultFailure uint32 = 1
)

const quickTaskTicketItemID uint32 = 15013

type activityTaskTemplate struct {
	ID           uint32     `json:"id"`
	QuickFinish  uint32     `json:"quick_finish"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

func loadActivityTaskTemplate(taskID uint32) (activityTaskTemplate, error) {
	key := strconv.FormatUint(uint64(taskID), 10)
	entry, err := orm.GetConfigEntry("ShareCfg/task_data_template.json", key)
	if err != nil && db.IsNotFound(err) {
		entry, err = orm.GetConfigEntry("sharecfgdata/task_data_template.json", key)
	}
	if err != nil {
		return activityTaskTemplate{}, err
	}
	var template activityTaskTemplate
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return activityTaskTemplate{}, err
	}
	return template, nil
}

func loadActivityTaskIDSet(actID uint32) (map[uint32]struct{}, error) {
	template, err := loadActivityTemplate(actID)
	if err != nil {
		return nil, err
	}
	ids, err := parseActivityTaskIDs(template.ConfigData)
	if err != nil {
		return nil, err
	}
	set := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

func buildAwardDropMap(awardDisplay [][]uint32) (map[string]*protobuf.DROPINFO, error) {
	drops := make(map[string]*protobuf.DROPINFO, len(awardDisplay))
	for _, entry := range awardDisplay {
		if len(entry) < 3 {
			return nil, errors.New("award display entry missing fields")
		}
		dropType := entry[0]
		dropID := entry[1]
		count := entry[2]
		if count == 0 {
			continue
		}
		key := fmt.Sprintf("%d_%d", dropType, dropID)
		if existing := drops[key]; existing != nil {
			existing.Number = proto.Uint32(existing.GetNumber() + count)
			continue
		}
		drops[key] = newDropInfo(dropType, dropID, count)
	}
	return drops, nil
}

func applyActivityTaskDropsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, drops map[string]*protobuf.DROPINFO) error {
	for _, drop := range drops {
		switch drop.GetType() {
		case consts.DROP_TYPE_RESOURCE:
			_, err := tx.Exec(ctx, `
INSERT INTO owned_resources (commander_id, resource_id, amount)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, resource_id)
DO UPDATE SET amount = owned_resources.amount + EXCLUDED.amount
`, int64(commanderID), int64(drop.GetId()), int64(drop.GetNumber()))
			if err != nil {
				return err
			}
		case consts.DROP_TYPE_ITEM:
			_, err := tx.Exec(ctx, `
INSERT INTO commander_items (commander_id, item_id, count)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, item_id)
DO UPDATE SET count = commander_items.count + EXCLUDED.count
`, int64(commanderID), int64(drop.GetId()), int64(drop.GetNumber()))
			if err != nil {
				return err
			}
		case consts.DROP_TYPE_VITEM:
			continue
		default:
			return fmt.Errorf("unsupported drop type %d", drop.GetType())
		}
	}
	return nil
}

func consumeCommanderItemTx(ctx context.Context, tx pgx.Tx, commanderID uint32, itemID uint32, count uint32) (bool, error) {
	result, err := tx.Exec(ctx, `
UPDATE commander_items
SET count = count - $3
WHERE commander_id = $1 AND item_id = $2 AND count >= $3
`, int64(commanderID), int64(itemID), int64(count))
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 1 {
		return true, nil
	}

	result, err = tx.Exec(ctx, `
UPDATE commander_misc_items
SET data = data - $3
WHERE commander_id = $1 AND item_id = $2 AND data >= $3
`, int64(commanderID), int64(itemID), int64(count))
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 1 {
		return true, nil
	}
	return false, nil
}

func activityDropMapToSortedList(drops map[string]*protobuf.DROPINFO) []*protobuf.DROPINFO {
	keys := make([]string, 0, len(drops))
	for key := range drops {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]*protobuf.DROPINFO, 0, len(drops))
	for _, key := range keys {
		out = append(out, drops[key])
	}
	return out
}
