package orm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	shipMetaRepairCategory      = "ShareCfg/ship_meta_repair.json"
	shipMetaSkillTaskCategory   = "ShareCfg/ship_meta_skilltask.json"
	shipMetaBreakoutCategory    = "ShareCfg/ship_meta_breakout.json"
	shipStrengthenMetaCategory  = "ShareCfg/ship_strengthen_meta.json"
	skillDataTemplateCategory   = "sharecfgdata/skill_data_template.json"
	skillDataTemplateCategory2  = "ShareCfg/skill_data_template.json"
	itemDataStatisticsCategory  = "sharecfgdata/item_data_statistics.json"
	itemDataStatisticsCategory2 = "ShareCfg/item_data_statistics.json"
)

type ShipMetaRepairConfig struct {
	ID        uint32 `json:"id"`
	ItemID    uint32 `json:"item_id"`
	ItemNum   uint32 `json:"item_num"`
	RepairExp uint32 `json:"repair_exp"`
}

type ShipMetaBreakoutConfig struct {
	ID         uint32 `json:"id"`
	BreakoutID uint32 `json:"breakout_id"`
	Gold       uint32 `json:"gold"`
	Item1      uint32 `json:"item1"`
	Item1Num   uint32 `json:"item1_num"`
	Item2      uint32 `json:"item2"`
	Item2Num   uint32 `json:"item2_num"`
	Level      uint32 `json:"level"`
	Repair     uint32 `json:"repair"`
}

type ShipStrengthenMetaConfig struct {
	ID             uint32   `json:"id"`
	ShipID         uint32   `json:"ship_id"`
	Type           uint32   `json:"type"`
	RepairCannon   []uint32 `json:"repair_cannon"`
	RepairTorpedo  []uint32 `json:"repair_torpedo"`
	RepairAir      []uint32 `json:"repair_air"`
	RepairReload   []uint32 `json:"repair_reload"`
	RepairTotalExp uint32   `json:"repair_total_exp"`
}

type ShipMetaSkillTaskConfig struct {
	ID               uint32     `json:"id"`
	Level            uint32     `json:"level"`
	NeedExp          uint32     `json:"need_exp"`
	SkillID          uint32     `json:"skill_ID"`
	SkillLevelupTask [][]uint32 `json:"skill_levelup_task"`
	SkillUnlock      [][]uint32 `json:"skill_unlock"`
}

type SkillDataTemplateConfig struct {
	ID       uint32 `json:"id"`
	MaxLevel uint32 `json:"max_level"`
}

type ShipDataTemplateMetaConfig struct {
	ID              uint32   `json:"id"`
	BuffListDisplay []uint32 `json:"buff_list_display"`
}

type ItemDataStatisticsConfig struct {
	ID       uint32          `json:"id"`
	Type     uint32          `json:"type"`
	UsageArg json.RawMessage `json:"usage_arg"`
}

func GetShipMetaRepairConfig(repairID uint32) (*ShipMetaRepairConfig, error) {
	entry, err := GetConfigEntry(shipMetaRepairCategory, fmt.Sprintf("%d", repairID))
	if err != nil {
		return nil, err
	}
	var cfg ShipMetaRepairConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetShipMetaBreakoutConfig(templateID uint32) (*ShipMetaBreakoutConfig, error) {
	entry, err := GetConfigEntry(shipMetaBreakoutCategory, fmt.Sprintf("%d", templateID))
	if err != nil {
		return nil, err
	}
	var cfg ShipMetaBreakoutConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetShipStrengthenMetaConfig(metaID uint32) (*ShipStrengthenMetaConfig, error) {
	entry, err := GetConfigEntry(shipStrengthenMetaCategory, fmt.Sprintf("%d", metaID))
	if err != nil {
		return nil, err
	}
	var cfg ShipStrengthenMetaConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetShipMetaSkillTaskConfigByID(id uint32) (*ShipMetaSkillTaskConfig, error) {
	entry, err := GetConfigEntry(shipMetaSkillTaskCategory, fmt.Sprintf("%d", id))
	if err != nil {
		return nil, err
	}
	var cfg ShipMetaSkillTaskConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetShipMetaSkillTaskConfig(skillID uint32, level uint32) (*ShipMetaSkillTaskConfig, error) {
	entries, err := ListConfigEntries(shipMetaSkillTaskCategory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		var cfg ShipMetaSkillTaskConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, err
		}
		if cfg.SkillID == skillID && cfg.Level == level {
			return &cfg, nil
		}
	}
	return nil, db.ErrNotFound
}

func ListShipMetaSkillTaskConfigsBySkill(skillID uint32) ([]ShipMetaSkillTaskConfig, error) {
	entries, err := ListConfigEntries(shipMetaSkillTaskCategory)
	if err != nil {
		return nil, err
	}
	result := make([]ShipMetaSkillTaskConfig, 0)
	for _, entry := range entries {
		var cfg ShipMetaSkillTaskConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, err
		}
		if cfg.SkillID == skillID {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func GetSkillDataTemplateConfig(skillID uint32) (*SkillDataTemplateConfig, error) {
	key := fmt.Sprintf("%d", skillID)
	entry, err := GetConfigEntry(skillDataTemplateCategory, key)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return nil, err
		}
		entry, err = GetConfigEntry(skillDataTemplateCategory2, key)
		if err != nil {
			return nil, err
		}
	}
	var cfg SkillDataTemplateConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetShipDataTemplateMetaConfig(templateID uint32) (*ShipDataTemplateMetaConfig, error) {
	entry, err := GetConfigEntry(shipDataTemplateCategory, fmt.Sprintf("%d", templateID))
	if err != nil {
		return nil, err
	}
	var cfg ShipDataTemplateMetaConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetItemDataStatisticsConfig(itemID uint32) (*ItemDataStatisticsConfig, error) {
	key := fmt.Sprintf("%d", itemID)
	entry, err := GetConfigEntry(itemDataStatisticsCategory, key)
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return nil, err
		}
		entry, err = GetConfigEntry(itemDataStatisticsCategory2, key)
		if err != nil {
			return nil, err
		}
	}
	var cfg ItemDataStatisticsConfig
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
