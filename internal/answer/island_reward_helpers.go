package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandOrderFavorCategory   = "ShareCfg/island_order_favor.json"
	islandOrderPriceCategory   = "ShareCfg/island_order_price.json"
	islandOrderRandomCategory  = "ShareCfg/island_order_publish_random.json"
	islandRewardSetCategory    = "ShareCfg/island_set.json"
	islandRewardSeasonCategory = "ShareCfg/island_season.json"
	islandProsperityCategory   = "ShareCfg/island_prosperity.json"
)

type islandOrderFavorConfig struct {
	ID           uint32     `json:"id"`
	Level        uint32     `json:"level"`
	Exp          uint32     `json:"exp"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

type islandOrderPriceConfig struct {
	ID                uint32   `json:"id"`
	OrderAwardSpecial []uint32 `json:"order_award_special"`
}

type islandOrderRandomConfig struct {
	ID uint32 `json:"id"`
}

type islandSetConfig struct {
	Key         string          `json:"key"`
	KeyValueInt uint32          `json:"key_value_int"`
	KeyValueRaw json.RawMessage `json:"key_value_varchar"`
}

type islandSeasonConfig struct {
	ID             uint32       `json:"id"`
	Target         []uint32     `json:"target"`
	PTAwardDisplay [][]uint32   `json:"ptaward_display"`
	Time           [][][]uint32 `json:"time"`
}

type islandProsperityConfig struct {
	ID           uint32     `json:"id"`
	Prosperity   uint32     `json:"prosperity"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

func loadIslandOrderFavorConfig() (map[uint32]islandOrderFavorConfig, error) {
	entries, err := orm.ListConfigEntries(islandOrderFavorCategory)
	if err != nil {
		return nil, err
	}
	lookup := make(map[uint32]islandOrderFavorConfig)
	for _, entry := range entries {
		var single islandOrderFavorConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.Level == 0 {
				single.Level = single.ID
			}
			if single.Level != 0 {
				lookup[single.Level] = single
			}
			continue
		}
		var list []islandOrderFavorConfig
		if err := json.Unmarshal(entry.Data, &list); err != nil {
			return nil, err
		}
		for i := range list {
			if list[i].Level == 0 {
				list[i].Level = list[i].ID
			}
			if list[i].Level != 0 {
				lookup[list[i].Level] = list[i]
			}
		}
	}
	return lookup, nil
}

func loadIslandOrderPriceConfig(orderLv uint32) (*islandOrderPriceConfig, bool, error) {
	key := fmt.Sprintf("%d", orderLv)
	if entry, err := orm.GetConfigEntry(islandOrderPriceCategory, key); err == nil {
		var single islandOrderPriceConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == 0 {
				single.ID = orderLv
			}
			return &single, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandOrderPriceCategory)
	if err != nil {
		return nil, false, err
	}
	for _, entry := range entries {
		var single islandOrderPriceConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == orderLv {
				return &single, true, nil
			}
			continue
		}
		var list []islandOrderPriceConfig
		if err := json.Unmarshal(entry.Data, &list); err != nil {
			return nil, false, err
		}
		for i := range list {
			if list[i].ID == orderLv {
				return &list[i], true, nil
			}
		}
	}
	return nil, false, nil
}

func loadIslandSetIntConfig(key string) (uint32, bool, error) {
	entry, err := orm.GetConfigEntry(islandRewardSetCategory, key)
	if err == nil {
		var parsed islandSetConfig
		if err := json.Unmarshal(entry.Data, &parsed); err != nil {
			return 0, false, err
		}
		return parsed.KeyValueInt, true, nil
	}

	entries, err := orm.ListConfigEntries(islandRewardSetCategory)
	if err != nil {
		return 0, false, err
	}
	for _, cfgEntry := range entries {
		var parsed islandSetConfig
		if err := json.Unmarshal(cfgEntry.Data, &parsed); err != nil {
			continue
		}
		if parsed.Key == key {
			return parsed.KeyValueInt, true, nil
		}
	}
	return 0, false, nil
}

func loadIslandRandomDialogIDs() ([]uint32, error) {
	entries, err := orm.ListConfigEntries(islandOrderRandomCategory)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0)
	for _, entry := range entries {
		var single islandOrderRandomConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID != 0 {
				ids = append(ids, single.ID)
			}
			continue
		}
		var list []islandOrderRandomConfig
		if err := json.Unmarshal(entry.Data, &list); err != nil {
			return nil, err
		}
		for i := range list {
			if list[i].ID != 0 {
				ids = append(ids, list[i].ID)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func loadIslandSeasonConfig() (*islandSeasonConfig, bool, error) {
	seasonID, found, err := loadIslandSetIntConfig("season_now")
	if err != nil {
		return nil, false, err
	}
	if !found || seasonID == 0 {
		seasonID = 1
	}

	key := fmt.Sprintf("%d", seasonID)
	if entry, err := orm.GetConfigEntry(islandRewardSeasonCategory, key); err == nil {
		var single islandSeasonConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == 0 {
				single.ID = seasonID
			}
			return &single, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandRewardSeasonCategory)
	if err != nil {
		return nil, false, err
	}
	for _, entry := range entries {
		var single islandSeasonConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == seasonID {
				return &single, true, nil
			}
			continue
		}
		var list []islandSeasonConfig
		if err := json.Unmarshal(entry.Data, &list); err != nil {
			return nil, false, err
		}
		for i := range list {
			if list[i].ID == seasonID {
				return &list[i], true, nil
			}
		}
	}
	return nil, false, nil
}

func loadIslandProsperityConfig(level uint32) (*islandProsperityConfig, bool, error) {
	key := fmt.Sprintf("%d", level)
	if entry, err := orm.GetConfigEntry(islandProsperityCategory, key); err == nil {
		var single islandProsperityConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == 0 {
				single.ID = level
			}
			return &single, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandProsperityCategory)
	if err != nil {
		return nil, false, err
	}
	for _, entry := range entries {
		var single islandProsperityConfig
		if err := json.Unmarshal(entry.Data, &single); err == nil {
			if single.ID == level {
				return &single, true, nil
			}
			continue
		}
		var list []islandProsperityConfig
		if err := json.Unmarshal(entry.Data, &list); err != nil {
			return nil, false, err
		}
		for i := range list {
			if list[i].ID == level {
				return &list[i], true, nil
			}
		}
	}
	return nil, false, nil
}

func applyIslandDropsTx(ctx context.Context, tx pgx.Tx, client *connection.Client, drops []*protobuf.DROPINFO) error {
	for _, drop := range drops {
		switch drop.GetType() {
		case consts.DROP_TYPE_ISLAND_ITEM:
			if err := orm.AddIslandInventoryTx(ctx, tx, client.Commander.CommanderID, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_RESOURCE:
			if err := client.Commander.AddResourceTx(ctx, tx, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_ITEM:
			if err := client.Commander.AddItemTx(ctx, tx, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.DROP_TYPE_SHIP:
			for i := uint32(0); i < drop.GetNumber(); i++ {
				if _, err := client.Commander.AddShipTx(ctx, tx, drop.GetId()); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported island drop type %d", drop.GetType())
		}
	}
	return nil
}

func mergeDropList(drops []*protobuf.DROPINFO) []*protobuf.DROPINFO {
	merged := make(map[string]*protobuf.DROPINFO, len(drops))
	for _, drop := range drops {
		key := fmt.Sprintf("%d_%d", drop.GetType(), drop.GetId())
		existing := merged[key]
		if existing == nil {
			merged[key] = newDropInfo(drop.GetType(), drop.GetId(), drop.GetNumber())
			continue
		}
		existing.Number = proto.Uint32(existing.GetNumber() + drop.GetNumber())
	}
	out := make([]*protobuf.DROPINFO, 0, len(merged))
	for _, drop := range merged {
		out = append(out, drop)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GetType() == out[j].GetType() {
			return out[i].GetId() < out[j].GetId()
		}
		return out[i].GetType() < out[j].GetType()
	})
	return out
}
