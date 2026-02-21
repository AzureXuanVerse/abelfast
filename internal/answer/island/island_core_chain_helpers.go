package island

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/orm"
)

const (
	islandLevelCategory         = "ShareCfg/island_level.json"
	islandLevelCategoryLC       = "sharecfgdata/island_level.json"
	islandStorageLevelCategory  = "ShareCfg/island_storage_level.json"
	islandStorageLevelCategoryL = "sharecfgdata/island_storage_level.json"
	islandItemTemplateCategoryL = "sharecfgdata/island_item_data_template.json"
	islandShopGoodsCategory     = "ShareCfg/island_shop_goods.json"
	islandShopGoodsCategoryLC   = "sharecfgdata/island_shop_goods.json"
)

type islandLevelTemplate struct {
	ID               uint32     `json:"id"`
	IslandLevel      uint32     `json:"island_level"`
	IslandExp        uint32     `json:"island_exp"`
	Cost             [][]uint32 `json:"cost"`
	IslandLevelAward [][]uint32 `json:"island_level_award"`
}

type islandStorageLevelTemplate struct {
	ID              uint32     `json:"id"`
	Level           uint32     `json:"level"`
	UpgradeMaterial [][]uint32 `json:"upgrade_material"`
}

type islandItemTemplate struct {
	ID           uint32          `json:"id"`
	Convert      uint32          `json:"convert"`
	PTNum        uint32          `json:"pt_num"`
	Usage        string          `json:"usage"`
	UsageArg     json.RawMessage `json:"usage_arg"`
	DropAfterUse uint32          `json:"drop_after_use"`
}

type islandShopGoodsTemplate struct {
	ID              uint32     `json:"id"`
	ResourceConsume []uint32   `json:"resource_consume"`
	Items           [][]uint32 `json:"items"`
	LimitedNum      uint32     `json:"limited_num"`
	PayID           uint32     `json:"pay_id"`
	PTAward         uint32     `json:"pt_award"`
}

func loadIslandLevelTemplate(level uint32) (*islandLevelTemplate, bool, error) {
	lookup, err := loadIslandLevelTemplates()
	if err != nil {
		return nil, false, err
	}
	entry, ok := lookup[level]
	if !ok {
		return nil, false, nil
	}
	return &entry, true, nil
}

func loadIslandLevelTemplates() (map[uint32]islandLevelTemplate, error) {
	lookup := make(map[uint32]islandLevelTemplate)
	loaded, err := loadIslandLevelTemplatesFromCategory(islandLevelCategory, lookup)
	if err != nil {
		return nil, err
	}
	if loaded {
		return lookup, nil
	}
	_, err = loadIslandLevelTemplatesFromCategory(islandLevelCategoryLC, lookup)
	if err != nil {
		return nil, err
	}
	return lookup, nil
}

func loadIslandLevelTemplatesFromCategory(category string, lookup map[uint32]islandLevelTemplate) (bool, error) {
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return false, nil
	}
	if len(entries) == 0 {
		return false, nil
	}
	for i := range entries {
		if err := parseIslandLevelTemplateEntry(entries[i].Data, lookup); err != nil {
			return false, err
		}
	}
	return true, nil
}

func parseIslandLevelTemplateEntry(raw json.RawMessage, lookup map[uint32]islandLevelTemplate) error {
	var single islandLevelTemplate
	if err := json.Unmarshal(raw, &single); err == nil && (single.ID != 0 || single.IslandLevel != 0) {
		if single.IslandLevel == 0 {
			single.IslandLevel = single.ID
		}
		lookup[single.IslandLevel] = single
		return nil
	}
	var list []islandLevelTemplate
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	for i := range list {
		if list[i].IslandLevel == 0 {
			list[i].IslandLevel = list[i].ID
		}
		if list[i].IslandLevel != 0 {
			lookup[list[i].IslandLevel] = list[i]
		}
	}
	return nil
}

func loadIslandStorageLevelTemplate(level uint32) (*islandStorageLevelTemplate, bool, error) {
	lookup := make(map[uint32]islandStorageLevelTemplate)
	loaded, err := loadIslandStorageTemplatesFromCategory(islandStorageLevelCategory, lookup)
	if err != nil {
		return nil, false, err
	}
	if !loaded {
		if _, err := loadIslandStorageTemplatesFromCategory(islandStorageLevelCategoryL, lookup); err != nil {
			return nil, false, err
		}
	}
	entry, ok := lookup[level]
	if !ok {
		return nil, false, nil
	}
	return &entry, true, nil
}

func loadIslandStorageTemplatesFromCategory(category string, lookup map[uint32]islandStorageLevelTemplate) (bool, error) {
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return false, nil
	}
	if len(entries) == 0 {
		return false, nil
	}
	for i := range entries {
		var single islandStorageLevelTemplate
		if err := json.Unmarshal(entries[i].Data, &single); err == nil && (single.ID != 0 || single.Level != 0) {
			if single.Level == 0 {
				single.Level = single.ID
			}
			lookup[single.Level] = single
			continue
		}
		var list []islandStorageLevelTemplate
		if err := json.Unmarshal(entries[i].Data, &list); err != nil {
			return false, err
		}
		for j := range list {
			if list[j].Level == 0 {
				list[j].Level = list[j].ID
			}
			if list[j].Level != 0 {
				lookup[list[j].Level] = list[j]
			}
		}
	}
	return true, nil
}

func loadIslandItemTemplate(itemID uint32) (*islandItemTemplate, bool, error) {
	key := strconv.FormatUint(uint64(itemID), 10)
	if entry, err := orm.GetConfigEntry(islandItemTemplateCategory, key); err == nil {
		var cfg islandItemTemplate
		if err := json.Unmarshal(entry.Data, &cfg); err == nil {
			if cfg.ID == 0 {
				cfg.ID = itemID
			}
			return &cfg, true, nil
		}
	}
	if entry, err := orm.GetConfigEntry(islandItemTemplateCategoryL, key); err == nil {
		var cfg islandItemTemplate
		if err := json.Unmarshal(entry.Data, &cfg); err == nil {
			if cfg.ID == 0 {
				cfg.ID = itemID
			}
			return &cfg, true, nil
		}
	}

	for _, category := range []string{islandItemTemplateCategory, islandItemTemplateCategoryL} {
		entries, err := orm.ListConfigEntries(category)
		if err != nil {
			continue
		}
		for i := range entries {
			var single islandItemTemplate
			if err := json.Unmarshal(entries[i].Data, &single); err == nil && single.ID == itemID {
				return &single, true, nil
			}
			var list []islandItemTemplate
			if err := json.Unmarshal(entries[i].Data, &list); err != nil {
				continue
			}
			for j := range list {
				if list[j].ID == itemID {
					return &list[j], true, nil
				}
			}
		}
	}
	return nil, false, nil
}

func loadIslandShopGoodsTemplate(goodsID uint32) (*islandShopGoodsTemplate, bool, error) {
	key := strconv.FormatUint(uint64(goodsID), 10)
	for _, category := range []string{islandShopGoodsCategory, islandShopGoodsCategoryLC} {
		if entry, err := orm.GetConfigEntry(category, key); err == nil {
			var cfg islandShopGoodsTemplate
			if err := json.Unmarshal(entry.Data, &cfg); err == nil {
				if cfg.ID == 0 {
					cfg.ID = goodsID
				}
				return &cfg, true, nil
			}
		}
	}

	for _, category := range []string{islandShopGoodsCategory, islandShopGoodsCategoryLC} {
		entries, err := orm.ListConfigEntries(category)
		if err != nil {
			continue
		}
		for i := range entries {
			var single islandShopGoodsTemplate
			if err := json.Unmarshal(entries[i].Data, &single); err == nil && single.ID == goodsID {
				return &single, true, nil
			}
			var list []islandShopGoodsTemplate
			if err := json.Unmarshal(entries[i].Data, &list); err != nil {
				continue
			}
			for j := range list {
				if list[j].ID == goodsID {
					return &list[j], true, nil
				}
			}
		}
	}
	return nil, false, nil
}

func normalizeIslandName(name string) string {
	trimmed := ""
	for _, r := range name {
		if r == 0 {
			continue
		}
		trimmed += string(r)
	}
	return trimmed
}

func decodeIslandUsageAward(raw json.RawMessage) ([][]uint32, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var list [][]uint32
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	var flat []uint32
	if err := json.Unmarshal(raw, &flat); err == nil {
		if len(flat) >= 3 {
			return [][]uint32{flat[:3]}, nil
		}
	}
	return nil, fmt.Errorf("unsupported usage_arg format")
}
