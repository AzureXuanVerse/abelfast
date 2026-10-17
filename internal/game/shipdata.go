package game

import (
	"cmp"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strconv"

	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
)

// shipTemplate is a ship_data_template row, with its ship_data_statistics
// rarity, nationality and type.
type shipTemplate struct {
	Star     uint32   `json:"star"`
	Energy   uint32   `json:"energy"`
	MaxLevel uint32   `json:"max_level"`
	Equip1   uint32   `json:"equip_id_1"`
	Equip2   uint32   `json:"equip_id_2"`
	Equip3   uint32   `json:"equip_id_3"`
	Skills   []uint32 `json:"buff_list"`
	// 1 when the ship keeps exp past its max level (Ship:CanAccumulateExp)
	CanGetProficency uint32 `json:"can_get_proficency"`

	Rarity      uint32 `json:"-"`
	Nationality uint32 `json:"-"`
	Type        uint32 `json:"-"`
	// blueprint (Ship:isBluePrintShip, its ship_data_group handbook_type is
	// 2) or META (Ship:isMetaShip, a ship_meta_breakout id) ship, which
	// ShipStatus.canDestroyShip never lets retire
	unretirable bool
	// its ship_data_statistics skin_id, the skin it wears by default
	defaultSkin uint32
}

// equips are what the ship comes with, equip_id_1..3.
func (template shipTemplate) equips() [3]uint32 {
	return [3]uint32{template.Equip1, template.Equip2, template.Equip3}
}

// ship_data_statistics row: attrs by Ship.PROPERTIES (model/vo/ship.lua),
// their growths per level in 1/1000
type shipStatisticsConfig struct {
	Type             uint32    `json:"type"`
	Rarity           uint32    `json:"rarity"`
	Nationality      uint32    `json:"nationality"`
	SkinID           uint32    `json:"skin_id"`
	Attrs            []float64 `json:"attrs"`
	AttrsGrowth      []float64 `json:"attrs_growth"`
	AttrsGrowthExtra []float64 `json:"attrs_growth_extra"`
}

// AttributeType.Dodge, and its index in Ship.PROPERTIES (the 9th)
const (
	attributeDodge = "dodge"
	propertyDodge  = 8
)

// shipDodgeGrowth is a ship_data_statistics row's evasion: attrs,
// attrs_growth and attrs_growth_extra at propertyDodge.
type shipDodgeGrowth struct {
	base, growth, growthExtra float64
}

// intimacy_template row: an affinity range (1/100) and the stat bonus it
// gives (1/10000)
type intimacyConfig struct {
	LowerBound uint32 `json:"lower_bound"`
	UpperBound uint32 `json:"upper_bound"`
	AttrBonus  uint32 `json:"attr_bonus"`
}

// Ship.INTIMACY_PROPOSE, the intimacy_template row of an oathed ship
const intimacyPropose = 6

// equipStatisticsConfig is an equip_data_statistics row, the fields a
// ship's evasion reads: up to 3 [attribute_N, value_N] stats (value a number
// or a numeric string), property_rate {attribute: rate} and
// equip_parameters (avoid_extra, 1/10000). An upgraded equipment's row only
// lists what differs from its base row, the rest is read from it (the config
// tables' base lookup in include.lua).
type equipStatisticsConfig struct {
	Base            uint32          `json:"base"`
	Nationality     uint32          `json:"nationality"`
	Rarity          uint32          `json:"rarity"`
	Tech            uint32          `json:"tech"` // its T1-T3 grade, 0 for the gear no tech pack holds
	Attribute1      json.RawMessage `json:"attribute_1"`
	Attribute2      json.RawMessage `json:"attribute_2"`
	Attribute3      json.RawMessage `json:"attribute_3"`
	Value1          json.RawMessage `json:"value_1"`
	Value2          json.RawMessage `json:"value_2"`
	Value3          json.RawMessage `json:"value_3"`
	PropertyRate    json.RawMessage `json:"property_rate"`
	EquipParameters json.RawMessage `json:"equip_parameters"`
}

// equip_data_template row: what upgrading it costs and gives, and its
// equip_limit (its base row's when it has none)
type equipTemplateConfig struct {
	equipUpgrade
	equipReturn
	Base       uint32  `json:"base"`
	Prev       uint32  `json:"prev"` // 0 for a gear's +0
	EquipLimit *uint32 `json:"equip_limit"`
}

// equipEvasion is what a piece of equipment adds to evasion: a flat dodge
// (GetAttributes), a dodge rate (GetPropertyRate), the fleet's ambush
// evasion (equip_parameters.avoid_extra, 1/10000), and its equip_limit (two
// pieces sharing a non-zero one don't stack, Ship:getActiveEquipments).
type equipEvasion struct {
	dodge, dodgeRate float64
	avoidExtra       float64
	limit            uint32
}

// ship_data_group row: description lists where the ship can be obtained,
// [text, [action, params], ...]
type shipGroupConfig struct {
	GroupType    uint32          `json:"group_type"`
	HandbookType uint32          `json:"handbook_type"`
	Description  json.RawMessage `json:"description"`
	// the other groups whose skins the group can wear (ShipSkin.IsShareSkin)
	ShareGroupID []uint32 `json:"share_group_id"`
}

// handbookBlueprint is ship_data_group handbook_type of the research
// (blueprint) ships (Ship:Ctor's bluePrintFlag).
const handbookBlueprint = 2

// ship_data_by_type row: the gold a retired ship of the type gives
// (Ship:calReturnRes; its distory_resource_oil_ratio isn't returned)
type shipTypeConfig struct {
	RetireGold uint32 `json:"distory_resource_gold_ratio"`
	TeamType   string `json:"team_type"` // teamVanguard, teamMain or teamSubmarine
}

// ship_data_by_star row: what a retired ship of the rarity gives,
// [[type, id, count], ...]
type shipStarConfig struct {
	RetireItems [][]uint32 `json:"destory_item"`
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

// equipReturn is an equip_data_template row: what disassembling one piece
// gives back.
type equipReturn struct {
	Gold  uint32      `json:"destory_gold"`
	Items [][2]uint32 `json:"destory_item"` // [item, count]
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
	itemUseConfig
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

// shipLeveling is how a ship_data_template levels (Ship:getLevelExpConfig,
// Ship:CanAccumulateExp): on ship_level exp_ur when its
// ship_data_statistics rarity is UR, keeping exp past its max level when
// can_get_proficency.
type shipLeveling struct {
	ur, accumulate bool
}

var (
	shipTemplates map[uint32]shipTemplate
	// every ship_data_template's, limit broken and retrofit ones included
	shipLevelings map[uint32]shipLeveling
	buildPools    map[uint32]buildPool
	equipUpgrades map[uint32]equipUpgrade
	composes      map[uint32]composeRecipe
	itemPrices    map[uint32][2]uint32 // item_data_statistics.price: [resource, amount]
	equipIDs      map[uint32]bool      // every equip_data_template id
	equipReturns  map[uint32]equipReturn
	// what retiring a ship gives (Ship:calReturnRes): gold by
	// ship_data_statistics type, drops by rarity
	retireGold  map[uint32]uint32
	retireItems map[uint32][][]uint32
	// ship_data_by_type team_type by ship type (Ship:getTeamType)
	shipTeams map[uint32]string
	// ship_data_group share_group_id by group_type
	shareGroups map[uint32][]uint32
	// gameset urpt_chapter_max: [limit counter id, cap] of the Specialized
	// Core (virtual_type 20)
	specializedCoreCap [2]uint32
)

// what a ship's evasion is computed from (shipDodge)
var (
	shipDodges    map[uint32]shipDodgeGrowth // every ship_data_statistics id
	intimacies    []intimacyConfig           // intimacy_template, by id from 1
	equipEvasions map[uint32]equipEvasion    // the equipment that has any
	// gameset extra_attr_level_limit: the level above which
	// attrs_growth_extra applies
	extraAttrLevelLimit uint32
)

// buildPoolNames are the GETBOAT projectName of the ship_data_create_material
// pools, how ship_data_group descriptions link a ship to them: the client's
// pool marks (BuildShipProxy:GetPools in model/proxy/buildshipproxy.lua,
// BuildShipPool.BUILD_POOL_MARK_* in model/vo/buildshippool.lua).
var buildPoolNames = map[uint32]string{1: "special", 2: "light", 3: "heavy"}

// rateTipPattern reads a rate_tip line once its color tags are stripped:
// "Super Rare: 7%".
var (
	rateTipTags    = regexp.MustCompile(`<[^>]*>`)
	rateTipPattern = regexp.MustCompile(`^([A-Za-z ]+): ([0-9.]+)%`)
)

// loadShipData builds the ship, construction and equipment data: ships as
// obtained (base templates, id ending in 1, that have statistics), the
// construction pools, equipment upgrades, crafting recipes, item prices and
// uses.
func loadShipData() error {
	var templates map[uint32]shipTemplate
	var stats map[uint32]shipStatisticsConfig
	var pools map[uint32]buildPool
	var groups map[uint32]shipGroupConfig
	var items map[uint32]itemConfig
	var medals map[uint32]medalConfig
	var intimacyRows map[uint32]intimacyConfig
	var equips map[uint32]equipTemplateConfig
	var equipStats map[uint32]equipStatisticsConfig
	var gameset map[string]uint32
	var gametip map[string]string
	var shipTypes map[uint32]shipTypeConfig
	var shipStars map[uint32]shipStarConfig
	var metas map[uint32]json.RawMessage
	err := gamedata.LoadAll(
		gamedata.Load("ship_data_template", &templates),
		gamedata.Load("ship_data_by_type", &shipTypes),
		gamedata.Load("ship_data_by_star", &shipStars),
		gamedata.Load("ship_meta_breakout", &metas),
		func() error { return gamedata.GamesetDescription("urpt_chapter_max", &specializedCoreCap) },
		gamedata.Load("ship_data_statistics", &stats),
		gamedata.Load("ship_data_create_material", &pools),
		gamedata.Load("ship_data_group", &groups),
		gamedata.Load("equip_data_template", &equips),
		gamedata.Load("equip_data_statistics", &equipStats),
		gamedata.Load("compose_data_template", &composes),
		gamedata.Load("item_data_statistics", &items),
		gamedata.Load("medal_template", &medals),
		gamedata.Load("intimacy_template", &intimacyRows),
		loadBuildTimes,
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
		func() (err error) {
			gametip, err = gamedata.Gametip()
			return err
		},
	)
	if err != nil {
		return err
	}
	if err := loadMedals(medals); err != nil {
		return err
	}
	limit, ok := gameset["extra_attr_level_limit"]
	if !ok {
		return gamedata.Missing("gameset", "extra_attr_level_limit")
	}
	extraAttrLevelLimit = limit

	shipDodges = make(map[uint32]shipDodgeGrowth, len(stats))
	for id, stat := range stats {
		if len(stat.Attrs) <= propertyDodge || len(stat.AttrsGrowth) <= propertyDodge {
			continue
		}
		growth := shipDodgeGrowth{base: stat.Attrs[propertyDodge], growth: stat.AttrsGrowth[propertyDodge]}
		if len(stat.AttrsGrowthExtra) > propertyDodge {
			growth.growthExtra = stat.AttrsGrowthExtra[propertyDodge]
		}
		shipDodges[id] = growth
	}
	intimacies = make([]intimacyConfig, max(len(intimacyRows), intimacyPropose))
	for i := range intimacies {
		row, ok := intimacyRows[uint32(i+1)]
		if !ok {
			return gamedata.Missing("intimacy_template", "row %d", i+1)
		}
		intimacies[i] = row
	}
	loadEquipEvasions(equipStats, equips)

	blueprints := map[uint32]bool{}
	shareGroups = map[uint32][]uint32{}
	for _, group := range groups {
		if group.HandbookType == handbookBlueprint {
			blueprints[group.GroupType] = true
		}
		if len(group.ShareGroupID) > 0 {
			shareGroups[group.GroupType] = group.ShareGroupID
		}
	}
	shipTemplates = map[uint32]shipTemplate{}
	shipLevelings = map[uint32]shipLeveling{}
	for id, row := range templates {
		stat, ok := stats[id]
		if ok {
			shipLevelings[id] = shipLeveling{ur: stat.Rarity == shipRarityUR, accumulate: row.CanGetProficency == 1}
		}
		if id%10 != 1 || !ok {
			continue
		}
		row.Rarity, row.Nationality, row.Type = stat.Rarity, stat.Nationality, stat.Type
		_, meta := metas[id]
		row.unretirable = meta || blueprints[id/10]
		row.defaultSkin = stat.SkinID
		shipTemplates[id] = row
	}
	retireGold = make(map[uint32]uint32, len(shipTypes))
	shipTeams = make(map[uint32]string, len(shipTypes))
	for id, row := range shipTypes {
		retireGold[id] = row.RetireGold
		shipTeams[id] = row.TeamType
	}
	retireItems = make(map[uint32][][]uint32, len(shipStars))
	for id, row := range shipStars {
		retireItems[id] = row.RetireItems
	}

	// ships without a construction time (buildTimes) are left out
	untimed := 0
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
			ship, ok := shipTemplates[group*10+1]
			if !ok {
				continue
			}
			if _, timed := buildTimes[group*10+1]; !timed {
				untimed++
				continue
			}
			pool.candidates[ship.Rarity] = append(pool.candidates[ship.Rarity], group*10+1)
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
	if untimed > 0 {
		logger.WithFields("Build", logger.FieldValue("left out", untimed)).Warn("pool ships without a construction time in server/build_times.json")
	}

	// only the equipment that upgrades into another
	equipUpgrades = map[uint32]equipUpgrade{}
	for id, equip := range equips {
		if equip.Next != 0 {
			equipUpgrades[id] = equip.equipUpgrade
		}
	}

	itemPrices = map[uint32][2]uint32{}
	for id, row := range items {
		var price []uint32
		if json.Unmarshal(row.Price, &price) == nil && len(price) >= 2 {
			itemPrices[id] = [2]uint32{price[0], price[1]}
		}
	}
	readItemUses(items, techPackGear(equips, equipStats), gametip)
	equipIDs = make(map[uint32]bool, len(equips))
	equipReturns = make(map[uint32]equipReturn, len(equips))
	for id, equip := range equips {
		equipIDs[id] = true
		equipReturns[id] = equip.equipReturn
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

// stat is the row's [attribute_N, value_N], i from 0.
func (row equipStatisticsConfig) stat(i int) (attribute, value json.RawMessage) {
	return [3]json.RawMessage{row.Attribute1, row.Attribute2, row.Attribute3}[i],
		[3]json.RawMessage{row.Value1, row.Value2, row.Value3}[i]
}

// loadEquipEvasions builds equipEvasions from every piece of equipment's
// dodge stats (Equipment:GetAttributes and GetPropertyRate, equip_parameters,
// model/vo/equipment.lua) and equip_limit.
func loadEquipEvasions(stats map[uint32]equipStatisticsConfig, templates map[uint32]equipTemplateConfig) {
	// inherited reads a row's field, else its base row's
	inherited := func(id uint32, field func(equipStatisticsConfig) json.RawMessage) json.RawMessage {
		row := stats[id]
		if value := field(row); value != nil || row.Base == 0 {
			return value
		}
		return field(stats[row.Base])
	}
	equipEvasions = map[uint32]equipEvasion{}
	for id := range stats {
		var evasion equipEvasion
		for i := range 3 {
			var attribute string
			raw := inherited(id, func(row equipStatisticsConfig) json.RawMessage { attribute, _ := row.stat(i); return attribute })
			if json.Unmarshal(raw, &attribute) != nil || attribute != attributeDodge {
				continue
			}
			// a number, or a string tonumber reads
			raw = inherited(id, func(row equipStatisticsConfig) json.RawMessage { _, value := row.stat(i); return value })
			var value float64
			if json.Unmarshal(raw, &value) != nil {
				var text string
				if json.Unmarshal(raw, &text) == nil {
					value, _ = strconv.ParseFloat(text, 64)
				}
			}
			evasion.dodge += value
		}
		// a {attribute: rate} table, or an empty list
		var rates map[string]float64
		if json.Unmarshal(inherited(id, func(row equipStatisticsConfig) json.RawMessage { return row.PropertyRate }), &rates) == nil {
			evasion.dodgeRate = rates[attributeDodge]
		}
		var parameters struct {
			AvoidExtra float64 `json:"avoid_extra"`
		}
		if json.Unmarshal(inherited(id, func(row equipStatisticsConfig) json.RawMessage { return row.EquipParameters }), &parameters) == nil {
			evasion.avoidExtra = parameters.AvoidExtra
		}
		if template, ok := templates[id]; ok {
			if template.EquipLimit == nil {
				template = templates[template.Base]
			}
			if template.EquipLimit != nil {
				evasion.limit = *template.EquipLimit
			}
		}
		if evasion != (equipEvasion{}) {
			equipEvasions[id] = evasion
		}
	}
}
