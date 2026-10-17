package game

import (
	"cmp"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strconv"

	"github.com/ggmolly/belfast/internal/gamedata"
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

	// the ships it yields (ship_data_group descriptions linking to it) by
	// rarity, the rate_tip rate of the rarities that have ships (by rarity)
	// and the sum of those rates
	candidates map[uint32][]uint32
	rates      []rarityRate
	total      float64
}

type rarityRate struct {
	rarity uint32
	rate   float64
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

// item_data_statistics row: price is [resource, amount], or "" when the
// item doesn't sell
type itemConfig struct {
	Price json.RawMessage `json:"price"`
}

// medal_template row
type medalConfig struct {
	Start      uint32          `json:"start"` // 0 for a medal's first tier
	TargetType uint32          `json:"target_type"`
	TargetID   json.RawMessage `json:"target_id"`
}

// medal_template.target_type
const (
	medalTargetSubMedals   = 160  // the sub-medals in target_id are all won
	medalTargetNationExp   = 1041 // exp of the ships of a nationality
	medalTargetNationLevel = 1042 // ships of a nationality at a level
	medalTargetNationLove  = 1043 // affinity of the ships of a nationality
	medalTargetArchive     = 1044 // ships unlocked in the archives
)

// nationTrophy is a faction medal's sub-medals (medalTargetNation*), by
// ship_data_statistics nationality: ships at level or above, affinity, exp.
type nationTrophy struct {
	nationality, level      uint32
	level120, intimacy, exp uint32
}

var (
	// archiveTrophies are the medal_template ids CollectionProxy gets
	// progress for: the first tier of every medal, each followed by its
	// sub-medals (the captured order).
	archiveTrophies []uint32
	// archiveCountTrophy is the Naval F.L.E.X. Medal: ships unlocked in the
	// archives.
	archiveCountTrophy uint32
	// nationTrophies are the faction medals' sub-medals, nationalities in
	// descending order (as the official server sends them).
	nationTrophies []nationTrophy
)

// loadMedals builds archiveTrophies, archiveCountTrophy and nationTrophies.
func loadMedals(medals map[uint32]medalConfig) error {
	archiveTrophies = nil
	nations := map[uint32]*nationTrophy{}
	for _, id := range slices.Sorted(maps.Keys(medals)) {
		medal := medals[id]
		if medal.Start != 0 {
			continue
		}
		archiveTrophies = append(archiveTrophies, id)
		if medal.TargetType == medalTargetArchive {
			archiveCountTrophy = id
		}
		if medal.TargetType != medalTargetSubMedals {
			continue
		}
		for _, subID := range gamedata.IDs(medal.TargetID) {
			archiveTrophies = append(archiveTrophies, subID)
			sub := medals[subID]
			// target_id: [[nationality, level]], "nationality" or
			// [[nationality, 0]]
			target := gamedata.IDs(sub.TargetID)
			if len(target) == 0 {
				continue
			}
			nation := nations[target[0]]
			if nation == nil {
				nation = &nationTrophy{nationality: target[0]}
				nations[target[0]] = nation
			}
			switch sub.TargetType {
			case medalTargetNationLevel:
				nation.level120 = subID
				if len(target) > 1 {
					nation.level = target[1]
				}
			case medalTargetNationLove:
				nation.intimacy = subID
			case medalTargetNationExp:
				nation.exp = subID
			}
		}
	}
	if archiveCountTrophy == 0 {
		return gamedata.Missing("medal_template", "archive medal")
	}
	nationTrophies = nil
	for _, nationality := range slices.Backward(slices.Sorted(maps.Keys(nations))) {
		nationTrophies = append(nationTrophies, *nations[nationality])
	}
	return nil
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
	var templates map[uint32]shipTemplate
	var stats map[uint32]shipStatisticsConfig
	var pools map[uint32]buildPool
	var groups map[uint32]shipGroupConfig
	var items map[uint32]itemConfig
	var medals map[uint32]medalConfig
	err := gamedata.LoadAll(
		gamedata.Load("ship_data_template", &templates),
		gamedata.Load("ship_data_statistics", &stats),
		gamedata.Load("ship_data_create_material", &pools),
		gamedata.Load("ship_data_group", &groups),
		gamedata.Load("equip_data_template", &equipUpgrades),
		gamedata.Load("compose_data_template", &composes),
		gamedata.Load("item_data_statistics", &items),
		gamedata.Load("medal_template", &medals),
	)
	if err != nil {
		return err
	}
	if err := loadMedals(medals); err != nil {
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

	buildPools = map[uint32]buildPool{}
	for id, name := range buildPoolNames {
		pool, ok := pools[id]
		if !ok {
			return gamedata.Missing("ship_data_create_material", "pool %d", id)
		}
		var poolGroups []uint32
		for _, group := range groups {
			if obtainedFromPool(group.Description, name) {
				poolGroups = append(poolGroups, group.GroupType)
			}
		}
		slices.Sort(poolGroups)
		pool.candidates = map[uint32][]uint32{}
		for _, group := range poolGroups {
			if ship, ok := shipTemplates[group*10+1]; ok {
				pool.candidates[ship.Rarity] = append(pool.candidates[ship.Rarity], group*10+1)
			}
		}
		for _, tip := range pool.RateTip {
			match := rateTipPattern.FindStringSubmatch(rateTipTags.ReplaceAllString(tip, ""))
			if match == nil {
				continue
			}
			rate, err := strconv.ParseFloat(match[2], 64)
			if err != nil {
				continue
			}
			rarity := rarityByRateName[match[1]]
			if len(pool.candidates[rarity]) > 0 && !slices.ContainsFunc(pool.rates, func(r rarityRate) bool { return r.rarity == rarity }) {
				pool.rates = append(pool.rates, rarityRate{rarity, rate})
				pool.total += rate
			}
		}
		slices.SortFunc(pool.rates, func(a, b rarityRate) int { return cmp.Compare(a.rarity, b.rarity) })
		buildPools[id] = pool
	}

	// only the equipment that upgrades into another
	maps.DeleteFunc(equipUpgrades, func(_ uint32, upgrade equipUpgrade) bool { return upgrade.Next == 0 })

	itemPrices = map[uint32][2]uint32{}
	for id, row := range items {
		var price []uint32
		if json.Unmarshal(row.Price, &price) == nil && len(price) >= 2 {
			itemPrices[id] = [2]uint32{price[0], price[1]}
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
