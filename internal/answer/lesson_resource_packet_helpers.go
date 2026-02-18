package answer

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
)

const (
	classFieldResourceID       = 10
	classUpgradeTemplateConfig = "ShareCfg/class_upgrade_template.json"
	gameSetConfig              = "ShareCfg/gameset.json"
	shipExpBooksGameSetKey     = "ship_exp_books"
	itemConfigCategoryPrimary  = "sharecfgdata/item_data_statistics.json"
	itemConfigCategoryFallback = "ShareCfg/item_data_statistics.json"
)

type classUpgradeTemplateConfigEntry struct {
	Level  uint32 `json:"level"`
	ItemID uint32 `json:"item_id"`
}

type itemStatisticsConfigEntry struct {
	ID       uint32          `json:"id"`
	UsageArg json.RawMessage `json:"usage_arg"`
	MaxNum   uint32          `json:"max_num"`
}

type gameSetConfigEntry struct {
	Description json.RawMessage `json:"description"`
}

func loadClassResourceItemID() (uint32, error) {
	entries, err := orm.ListConfigEntries(classUpgradeTemplateConfig)
	if err != nil {
		return 0, err
	}

	var bestLevel uint32
	var itemID uint32
	for _, entry := range entries {
		var config classUpgradeTemplateConfigEntry
		if err := json.Unmarshal(entry.Data, &config); err != nil {
			return 0, err
		}
		if config.ItemID == 0 {
			continue
		}
		if itemID == 0 || config.Level < bestLevel {
			bestLevel = config.Level
			itemID = config.ItemID
		}
	}

	if itemID == 0 {
		return 0, db.ErrNotFound
	}
	return itemID, nil
}

func loadItemStatisticsConfig(itemID uint32) (*itemStatisticsConfigEntry, error) {
	itemKey := strconv.FormatUint(uint64(itemID), 10)
	entry, err := orm.GetConfigEntry(itemConfigCategoryPrimary, itemKey)
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		entry, err = orm.GetConfigEntry(itemConfigCategoryFallback, itemKey)
		if err != nil {
			return nil, err
		}
	}

	var config itemStatisticsConfigEntry
	if err := json.Unmarshal(entry.Data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func parseUsageArgExpValue(raw json.RawMessage) (uint32, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("usage_arg is empty")
	}

	var number uint32
	if err := json.Unmarshal(raw, &number); err == nil {
		if number == 0 {
			return 0, fmt.Errorf("usage_arg must be positive")
		}
		return number, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		parsed, err := strconv.ParseUint(text, 10, 32)
		if err != nil || parsed == 0 {
			return 0, fmt.Errorf("usage_arg must be a positive integer")
		}
		return uint32(parsed), nil
	}

	var values []uint32
	if err := decodeUsageArg(raw, &values); err == nil {
		if len(values) == 0 || values[0] == 0 {
			return 0, fmt.Errorf("usage_arg array must contain a positive integer")
		}
		return values[0], nil
	}

	return 0, fmt.Errorf("usage_arg format is unsupported")
}

func loadShipExpBookSet() (map[uint32]struct{}, error) {
	entry, err := orm.GetConfigEntry(gameSetConfig, shipExpBooksGameSetKey)
	if err != nil {
		return nil, err
	}

	var gameSetEntry gameSetConfigEntry
	if err := json.Unmarshal(entry.Data, &gameSetEntry); err != nil {
		return nil, err
	}

	bookIDs := make([]uint32, 0)
	if err := decodeUsageArg(gameSetEntry.Description, &bookIDs); err != nil {
		var raw any
		if err := decodeUsageArg(gameSetEntry.Description, &raw); err != nil {
			return nil, err
		}
		bookIDs = collectUint32Values(raw, bookIDs)
	}

	bookSet := make(map[uint32]struct{}, len(bookIDs))
	for _, id := range bookIDs {
		if id == 0 {
			continue
		}
		bookSet[id] = struct{}{}
	}
	if len(bookSet) == 0 {
		return nil, db.ErrNotFound
	}
	return bookSet, nil
}

func collectUint32Values(input any, out []uint32) []uint32 {
	switch value := input.(type) {
	case float64:
		if value > 0 {
			out = append(out, uint32(value))
		}
	case string:
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err == nil && parsed > 0 {
			out = append(out, uint32(parsed))
		}
	case []any:
		for _, nested := range value {
			out = collectUint32Values(nested, out)
		}
	case map[string]any:
		for _, nested := range value {
			out = collectUint32Values(nested, out)
		}
	}
	return out
}
