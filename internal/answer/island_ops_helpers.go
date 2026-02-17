package answer

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandSpeedupTicketCategory = "ShareCfg/island_speedup_ticket.json"
	islandItemTemplateCategory  = "ShareCfg/island_item_data_template.json"
	islandOpsSetCategory        = "ShareCfg/island_set.json"
)

type islandSpeedupTicketConfig struct {
	ID          uint32 `json:"id"`
	SpeedupTime uint32 `json:"speedup_time"`
}

type islandItemTemplateConfig struct {
	ID         uint32 `json:"id"`
	OrderPrice uint32 `json:"order_price"`
}

type islandOpsSetConfigEntry struct {
	KeyValue []uint32 `json:"key_value"`
}

func loadIslandSpeedupSeconds(speedID uint32) (uint32, bool, error) {
	entry, err := orm.GetConfigEntry(islandSpeedupTicketCategory, fmt.Sprintf("%d", speedID))
	if err != nil {
		return 0, false, nil
	}
	var cfg islandSpeedupTicketConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return 0, false, err
	}
	if cfg.SpeedupTime == 0 {
		return 0, false, nil
	}
	return cfg.SpeedupTime, true, nil
}

func loadIslandItemOrderPrice(itemID uint32) (uint32, bool, error) {
	entry, err := orm.GetConfigEntry(islandItemTemplateCategory, fmt.Sprintf("%d", itemID))
	if err != nil {
		return 0, false, nil
	}
	var cfg islandItemTemplateConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return 0, false, err
	}
	if cfg.OrderPrice == 0 {
		return 0, false, nil
	}
	return cfg.OrderPrice, true, nil
}

func loadIslandSetKeyValue(name string) ([]uint32, bool, error) {
	entry, err := orm.GetConfigEntry(islandOpsSetCategory, name)
	if err != nil {
		entries, listErr := orm.ListConfigEntries(islandOpsSetCategory)
		if listErr != nil {
			return nil, false, listErr
		}
		for i := range entries {
			if entries[i].Key != name {
				continue
			}
			entry = &entries[i]
			err = nil
			break
		}
	}
	if err != nil || entry == nil {
		return nil, false, nil
	}
	var cfg islandOpsSetConfigEntry
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, false, err
	}
	if len(cfg.KeyValue) == 0 {
		return nil, false, nil
	}
	return cfg.KeyValue, true, nil
}

func nowUnix() uint32 {
	return uint32(time.Now().Unix())
}

func speedTicketConsumeFromProto(tickets []*protobuf.PB_SPEEDUP_TICKET) ([]orm.IslandSpeedupTicketConsume, bool) {
	out := make([]orm.IslandSpeedupTicketConsume, 0, len(tickets))
	for i := range tickets {
		if tickets[i] == nil || tickets[i].Key == nil || tickets[i].GetKey().GetSpeedId() == 0 || tickets[i].GetNum() == 0 {
			return nil, false
		}
		out = append(out, orm.IslandSpeedupTicketConsume{
			SpeedID: tickets[i].GetKey().GetSpeedId(),
			EndTime: tickets[i].GetKey().GetEndTime(),
			Count:   tickets[i].GetNum(),
		})
	}
	return out, true
}
