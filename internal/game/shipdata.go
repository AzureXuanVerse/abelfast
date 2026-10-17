package game

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
)

// shipTemplate is a ship_data_template row, with its ship_data_statistics
// rarity and nationality.
type shipTemplate struct {
	Star     uint32   `json:"star"`
	Energy   uint32   `json:"energy"`
	MaxLevel uint32   `json:"max_level"`
	Equip1   uint32   `json:"equip_id_1"`
	Equip2   uint32   `json:"equip_id_2"`
	Equip3   uint32   `json:"equip_id_3"`
	Skills   []uint32 `json:"buff_list"`

	Rarity      uint32 `json:"-"`
	Nationality uint32 `json:"-"`
}

// equips are what the ship comes with, equip_id_1..3.
func (template shipTemplate) equips() [3]uint32 {
	return [3]uint32{template.Equip1, template.Equip2, template.Equip3}
}

// ship_data_statistics row
type shipStatisticsConfig struct {
	Rarity      uint32 `json:"rarity"`
	Nationality uint32 `json:"nationality"`
}

// ship_data_group row: description lists where the ship can be obtained,
// [text, [action, params], ...]
type shipGroupConfig struct {
	GroupType   uint32          `json:"group_type"`
	Description json.RawMessage `json:"description"`
}

// buildPool is a ship_data_create_material row with its candidates.
type buildPool struct {
	Gold          uint32   `json:"use_gold"`
	Item          uint32   `json:"use_item"`
	ItemCount     uint32   `json:"number_1"`
	ExchangeCount uint32   `json:"exchange_count"`
	RateTip       []string `json:"rate_tip"`

	Rates  map[string]float64 `json:"-"` // rate_tip, by rarity name
	Groups []uint32           `json:"-"`
}

// equipUpgrade is an equip_data_template row: what upgrading it costs and
// gives.
type equipUpgrade struct {
	Next  uint32      `json:"next"`
	Gold  uint32      `json:"trans_use_gold"`
	Items [][2]uint32 `json:"trans_use_item"` // [item, count]
}

// composeRecipe is a compose_data_template row.
type composeRecipe struct {
	Equip         uint32 `json:"equip_id"`
	Gold          uint32 `json:"gold_num"`
	Material      uint32 `json:"material_id"`
	MaterialCount uint32 `json:"material_num"`
}

// item_data_statistics row
type itemConfig struct {
	Price []uint32 `json:"price"`
}

var (
	shipTemplates map[uint32]shipTemplate
	buildPools    map[uint32]buildPool
	equipUpgrades map[uint32]equipUpgrade
	composes      map[uint32]composeRecipe
	itemPrices    map[uint32][2]uint32 // item_data_statistics.price: [resource, amount]
)

// buildPoolNames are the GETBOAT projectName of the ship_data_create_material
// pools, how ship_data_group descriptions link a ship to them.
var buildPoolNames = map[uint32]string{1: "special", 2: "light", 3: "heavy"}

// rateTipPattern reads a rate_tip line once its color tags are stripped:
// "Super Rare: 7%".
var (
	rateTipTags    = regexp.MustCompile(`<[^>]*>`)
	rateTipPattern = regexp.MustCompile(`^([A-Za-z ]+): ([0-9.]+)%`)
)

// loadShipData builds the ship, construction and equipment data: ships as
// obtained (base templates, id ending in 1, that have statistics), the
// construction pools, equipment upgrades, crafting recipes and item prices.
func loadShipData() error {
	templates, err := readConfigTable[shipTemplate]("ship_data_template")
	if err != nil {
		return err
	}
	stats, err := readConfigTable[shipStatisticsConfig]("ship_data_statistics")
	if err != nil {
		return err
	}
	shipTemplates = map[uint32]shipTemplate{}
	for id, row := range templates {
		stat, ok := stats[id]
		if id%10 != 1 || !ok {
			continue
		}
		row.Rarity, row.Nationality = stat.Rarity, stat.Nationality
		shipTemplates[id] = row
	}

	pools, err := readConfigTable[buildPool]("ship_data_create_material")
	if err != nil {
		return err
	}
	groups, err := readConfigTable[shipGroupConfig]("ship_data_group")
	if err != nil {
		return err
	}
	buildPools = map[uint32]buildPool{}
	for id, name := range buildPoolNames {
		pool, ok := pools[id]
		if !ok {
			return fmt.Errorf("config table ship_data_create_material in %s: no pool %d", regionDataDir(), id)
		}
		pool.Rates = map[string]float64{}
		for _, tip := range pool.RateTip {
			match := rateTipPattern.FindStringSubmatch(rateTipTags.ReplaceAllString(tip, ""))
			if match == nil {
				continue
			}
			if rate, err := strconv.ParseFloat(match[2], 64); err == nil {
				pool.Rates[match[1]] = rate
			}
		}
		for _, group := range groups {
			if obtainedFromPool(group.Description, name) {
				pool.Groups = append(pool.Groups, group.GroupType)
			}
		}
		slices.Sort(pool.Groups)
		buildPools[id] = pool
	}

	if equipUpgrades, err = readConfigTable[equipUpgrade]("equip_data_template"); err != nil {
		return err
	}
	// only the equipment that upgrades into another
	maps.DeleteFunc(equipUpgrades, func(_ uint32, upgrade equipUpgrade) bool { return upgrade.Next == 0 })

	if composes, err = readConfigTable[composeRecipe]("compose_data_template"); err != nil {
		return err
	}

	items, err := readConfigTable[itemConfig]("item_data_statistics")
	if err != nil {
		return err
	}
	itemPrices = map[uint32][2]uint32{}
	for id, row := range items {
		if len(row.Price) >= 2 {
			itemPrices[id] = [2]uint32{row.Price[0], row.Price[1]}
		}
	}
	return nil
}

// obtainedFromPool is whether a ship_data_group description has a
// ["GETBOAT", {projectName: pool}] link.
func obtainedFromPool(description json.RawMessage, pool string) bool {
	var entries []json.RawMessage
	if json.Unmarshal(description, &entries) != nil {
		return false
	}
	for _, raw := range entries {
		var entry, link []json.RawMessage
		if json.Unmarshal(raw, &entry) != nil || len(entry) < 2 || json.Unmarshal(entry[1], &link) != nil || len(link) < 2 {
			continue
		}
		var action string
		var params struct {
			ProjectName string `json:"projectName"`
		}
		if json.Unmarshal(link[0], &action) == nil && action == "GETBOAT" &&
			json.Unmarshal(link[1], &params) == nil && params.ProjectName == pool {
			return true
		}
	}
	return false
}
