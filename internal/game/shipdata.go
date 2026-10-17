package game

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
)

// The few client config tables the ship, construction and equipment handlers
// need, cut down by shipdata/gen.sh (see there for the source fields).
//
//go:embed shipdata/*.json
var shipDataFiles embed.FS

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
	Gold          uint32             `json:"gold"`
	Item          uint32             `json:"item"`
	ItemCount     uint32             `json:"item_count"`
	ExchangeCount uint32             `json:"exchange_count"`
	Rates         map[string]float64 `json:"rates"` // rate_tip, by rarity name
	Groups        []uint32           `json:"groups"`
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
	shipTemplates = map[uint32]shipTemplate{}
	buildPools    = map[uint32]buildPool{}
	equipUpgrades = map[uint32]equipUpgrade{}
	composes      = map[uint32]composeRecipe{}
	itemPrices    = map[uint32][2]uint32{} // item_data_statistics.price: [resource, amount]
)

func init() {
	var ships map[string][]json.RawMessage
	mustLoad("ships.json", &ships)
	for key, row := range ships {
		var t shipTemplate
		mustFields(row, &t.Star, &t.Energy, &t.MaxLevel, &t.Rarity, &t.Nationality, &t.Equips, &t.Skills)
		shipTemplates[mustID(key)] = t
	}
	var pools map[string]buildPool
	mustLoad("pools.json", &pools)
	for key, pool := range pools {
		buildPools[mustID(key)] = pool
	}
	var upgrades map[string][]json.RawMessage
	mustLoad("equip_upgrades.json", &upgrades)
	for key, row := range upgrades {
		var u equipUpgrade
		mustFields(row, &u.Next, &u.Gold, &u.Items)
		equipUpgrades[mustID(key)] = u
	}
	var recipes map[string][4]uint32
	mustLoad("compose.json", &recipes)
	for key, r := range recipes {
		composes[mustID(key)] = composeRecipe{Equip: r[0], Gold: r[1], Material: r[2], MaterialCount: r[3]}
	}
	var prices map[string][2]uint32
	mustLoad("item_prices.json", &prices)
	for key, price := range prices {
		itemPrices[mustID(key)] = price
	}
}

func mustLoad(name string, v any) {
	raw, err := shipDataFiles.ReadFile("shipdata/" + name)
	if err == nil {
		err = json.Unmarshal(raw, v)
	}
	if err != nil {
		panic(fmt.Sprintf("shipdata/%s: %v", name, err))
	}
}

func mustFields(row []json.RawMessage, fields ...any) {
	if len(row) != len(fields) {
		panic(fmt.Sprintf("shipdata: row %s has %d fields, want %d", row, len(row), len(fields)))
	}
	for i, field := range fields {
		if err := json.Unmarshal(row[i], field); err != nil {
			panic(fmt.Sprintf("shipdata: row %s: %v", row, err))
		}
	}
}

func mustID(key string) uint32 {
	id, err := strconv.ParseUint(key, 10, 32)
	if err != nil {
		panic(fmt.Sprintf("shipdata: id %q: %v", key, err))
	}
	return uint32(id)
}
