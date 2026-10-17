package game

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

// shipTemplate is a ship_data_template row as obtained (+ its
// ship_data_statistics rarity and nationality).
type shipTemplate struct {
	Star        uint32
	Energy      uint32
	MaxLevel    uint32
	Rarity      uint32
	Nationality uint32
	Equips      [3]uint32 // equip_id_1..3, what the ship comes with
	Skills      []uint32  // buff_list
}

// buildPool is a ship_data_create_material row with its candidates.
type buildPool struct {
	Gold          uint32
	Item          uint32
	ItemCount     uint32
	ExchangeCount uint32
	Rates         map[string]float64 // rate_tip, by rarity name
	Groups        []uint32
}

// equipUpgrade is what upgrading an equip_data_template row costs and gives.
type equipUpgrade struct {
	Next  uint32
	Gold  uint32
	Items [][2]uint32 // [item, count]
}

// composeRecipe is a compose_data_template row.
type composeRecipe struct {
	Equip         uint32
	Gold          uint32
	Material      uint32
	MaterialCount uint32
}

var (
	shipTemplates map[uint32]shipTemplate
	buildPools    map[uint32]buildPool
	equipUpgrades map[uint32]equipUpgrade
	composes      map[uint32]composeRecipe
	itemPrices    map[uint32][2]uint32 // item_data_statistics.price: [resource, amount]
)

// ship_data_template row
type shipTemplateConfig struct {
	Star     uint32   `json:"star"`
	Energy   uint32   `json:"energy"`
	MaxLevel uint32   `json:"max_level"`
	Equip1   uint32   `json:"equip_id_1"`
	Equip2   uint32   `json:"equip_id_2"`
	Equip3   uint32   `json:"equip_id_3"`
	BuffList []uint32 `json:"buff_list"`
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

// ship_data_create_material row
type buildPoolConfig struct {
	UseGold       uint32   `json:"use_gold"`
	UseItem       uint32   `json:"use_item"`
	Number1       uint32   `json:"number_1"`
	ExchangeCount uint32   `json:"exchange_count"`
	RateTip       []string `json:"rate_tip"`
}

// equip_data_template row
type equipTemplateConfig struct {
	Next         uint32      `json:"next"`
	TransUseGold uint32      `json:"trans_use_gold"`
	TransUseItem [][2]uint32 `json:"trans_use_item"`
}

// compose_data_template row
type composeConfig struct {
	EquipID     uint32 `json:"equip_id"`
	GoldNum     uint32 `json:"gold_num"`
	MaterialID  uint32 `json:"material_id"`
	MaterialNum uint32 `json:"material_num"`
}

// item_data_statistics row
type itemConfig struct {
	Price []uint32 `json:"price"`
}

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
	templates, err := readConfigTable[shipTemplateConfig]("ship_data_template")
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
		shipTemplates[id] = shipTemplate{
			Star: row.Star, Energy: row.Energy, MaxLevel: row.MaxLevel,
			Rarity: stat.Rarity, Nationality: stat.Nationality,
			Equips: [3]uint32{row.Equip1, row.Equip2, row.Equip3},
			Skills: row.BuffList,
		}
	}

	pools, err := readConfigTable[buildPoolConfig]("ship_data_create_material")
	if err != nil {
		return err
	}
	groups, err := readConfigTable[shipGroupConfig]("ship_data_group")
	if err != nil {
		return err
	}
	buildPools = map[uint32]buildPool{}
	for id, name := range buildPoolNames {
		row, ok := pools[id]
		if !ok {
			return fmt.Errorf("config table ship_data_create_material in %s: no pool %d", regionDataDir(), id)
		}
		pool := buildPool{
			Gold: row.UseGold, Item: row.UseItem, ItemCount: row.Number1, ExchangeCount: row.ExchangeCount,
			Rates: map[string]float64{},
		}
		for _, tip := range row.RateTip {
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

	equips, err := readConfigTable[equipTemplateConfig]("equip_data_template")
	if err != nil {
		return err
	}
	equipUpgrades = map[uint32]equipUpgrade{}
	for id, row := range equips {
		if row.Next != 0 {
			equipUpgrades[id] = equipUpgrade{Next: row.Next, Gold: row.TransUseGold, Items: row.TransUseItem}
		}
	}

	recipes, err := readConfigTable[composeConfig]("compose_data_template")
	if err != nil {
		return err
	}
	composes = map[uint32]composeRecipe{}
	for id, row := range recipes {
		composes[id] = composeRecipe{Equip: row.EquipID, Gold: row.GoldNum, Material: row.MaterialID, MaterialCount: row.MaterialNum}
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
