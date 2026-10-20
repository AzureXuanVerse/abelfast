package game

import (
	"encoding/json"
	"slices"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// Research ships (Priority / Decisive, the Dev. Dock: view/technology/
// shipblueprint*, TechnologyProxy, model/vo/shipblueprint.lua). Each
// ship_data_blueprint row is one research ship, its id the ship's group.
// Its research opens once its unlock_task_open_condition tasks are claimed,
// then runs its unlock_task tasks, each opening its delay after the start;
// once they are all claimed the ship is built. A built ship is developed
// with its blueprints (or coins, the catch-up) through the
// ship_strengthen_blueprint levels of strengthen_effect, then the fate
// simulation levels of fate_strengthen.

// blueprintConfig is a ship_data_blueprint row.
type blueprintConfig struct {
	ID      uint32 `json:"id"`
	Version uint32 `json:"blueprint_version"`
	// the catch-up: coins can buy development (is_pursuing 1) at price
	// coins a blueprint, discounted the first ones a day
	IsPursuing uint32 `json:"is_pursuing"`
	Price      uint32 `json:"price"`
	// the ship's own blueprint, what develops it
	StrengthenItem uint32 `json:"strengthen_item"`
	// the PR vouchers that build the ship at once (CS_63214)
	GainItemID []uint32 `json:"gain_item_id"`
	// [task id, seconds after the start it opens at]
	UnlockTask    [][]uint32 `json:"unlock_task"`
	OpenCondition []uint32   `json:"unlock_task_open_condition"`
	Strengthen    []uint32   `json:"strengthen_effect"`
	Fate          []uint32   `json:"fate_strengthen"`

	levels, fates []blueprintLevel
}

// blueprintLevel is a ship_strengthen_blueprint row: the exp the level
// needs, the ship level it needs, and its special effects (special 1) the
// server keeps: skills learnt, a skin given, a limit break, a skill
// upgraded ([old, new], fate simulation).
type blueprintLevel struct {
	Lv             uint32          `json:"lv"`
	NeedExp        uint32          `json:"need_exp"`
	NeedLv         uint32          `json:"need_lv"`
	Special        uint32          `json:"special"`
	EffectSkill    json.RawMessage `json:"effect_skill"`
	EffectSkin     uint32          `json:"effect_skin"`
	EffectBreakout uint32          `json:"effect_breakout"`
	ChangeSkill    json.RawMessage `json:"change_skill"`

	skills, changeSkill []uint32
}

var (
	blueprintConfigs map[uint32]blueprintConfig
	// blueprintOfTask is the research ship each unlock_task belongs to
	blueprintOfTask map[uint32]uint32
	// blueprintOpenTasks are every unlock_task_open_condition task, sorted
	blueprintOpenTasks []uint32
	// gameset blueprint_pursue_discount_ssr / _ur: [n, percent], the price
	// of the nth catch-up blueprint of the day from the last entry n reaches
	blueprintDiscountSSR, blueprintDiscountUR [][]uint32
	// gameset technology_catchup_itemid: by blueprint_version, [item, exp]
	// the combat data pack of the series and the sortie exp one gives
	blueprintSpeedups [][]uint32
)

func loadBlueprintData() error {
	var levels map[uint32]blueprintLevel
	err := gamedata.LoadAll(
		gamedata.Load("ship_data_blueprint", &blueprintConfigs),
		gamedata.Load("ship_strengthen_blueprint", &levels),
		func() error {
			return gamedata.GamesetDescription("blueprint_pursue_discount_ssr", &blueprintDiscountSSR)
		},
		func() error { return gamedata.GamesetDescription("blueprint_pursue_discount_ur", &blueprintDiscountUR) },
		func() error { return gamedata.GamesetDescription("technology_catchup_itemid", &blueprintSpeedups) },
	)
	if err != nil {
		return err
	}
	if len(blueprintDiscountSSR) == 0 || len(blueprintDiscountUR) == 0 {
		return gamedata.Missing("gameset", "blueprint_pursue_discount")
	}
	for id, level := range levels {
		level.skills = gamedata.IDs(level.EffectSkill)
		level.changeSkill = gamedata.IDs(level.ChangeSkill)
		levels[id] = level
	}
	blueprintOfTask = map[uint32]uint32{}
	blueprintOpenTasks = nil
	for id, config := range blueprintConfigs {
		config.levels, config.fates = nil, nil
		for _, level := range config.Strengthen {
			row, ok := levels[level]
			if !ok {
				return gamedata.Missing("ship_strengthen_blueprint", "%d of blueprint %d", level, id)
			}
			config.levels = append(config.levels, row)
		}
		for _, level := range config.Fate {
			row, ok := levels[level]
			if !ok {
				return gamedata.Missing("ship_strengthen_blueprint", "%d of blueprint %d", level, id)
			}
			config.fates = append(config.fates, row)
		}
		if len(config.levels) == 0 {
			return gamedata.Missing("ship_data_blueprint", "strengthen_effect of %d", id)
		}
		for _, task := range config.UnlockTask {
			if len(task) < 2 {
				return gamedata.Missing("ship_data_blueprint", "unlock_task of %d", id)
			}
			blueprintOfTask[task[0]] = id
		}
		blueprintOpenTasks = append(blueprintOpenTasks, config.OpenCondition...)
		blueprintConfigs[id] = config
	}
	slices.Sort(blueprintOpenTasks)
	blueprintOpenTasks = slices.Compact(blueprintOpenTasks)
	return nil
}

// blueprintTask tells whether a task is a research ship's: one of its tasks
// or of the tasks that open it.
func blueprintTask(id uint32) bool {
	_, ok := blueprintOfTask[id]
	return ok || slices.Contains(blueprintOpenTasks, id)
}

// taskIDs are the research's unlock_task ids, in order (getTaskIds).
func (config blueprintConfig) taskIDs() []uint32 {
	ids := make([]uint32, len(config.UnlockTask))
	for i, task := range config.UnlockTask {
		ids[i] = task[0]
	}
	return ids
}

// taskOpensAt is when the research's task opens for a research started at
// start (getTaskOpenTimeStamp), false when it isn't one of its tasks.
func (config blueprintConfig) taskOpensAt(task uint32, start int64) (int64, bool) {
	for _, entry := range config.UnlockTask {
		if entry[0] == task {
			return start + int64(entry[1]) + 1, true
		}
	}
	return 0, false
}

// shipTemplate is the research ship's base template (getShipVO).
func (config blueprintConfig) shipTemplate() uint32 {
	return config.ID*10 + 1
}

// ur is ShipBluePrint:isRarityUR: a Decisive (ultra rare) ship, whose
// catch-up has its own prices and daily count.
func (config blueprintConfig) ur() bool {
	return ownableShips[config.shipTemplate()].Rarity >= shipRarityUR
}

// maxLevel is getMaxLevel, the last development level.
func (config blueprintConfig) maxLevel() uint32 {
	return config.levels[len(config.levels)-1].Lv
}

// maxFate is getMaxFateLevel, the last fate simulation phase (0 when the
// ship has none).
func (config blueprintConfig) maxFate() int {
	if len(config.fates) == 0 {
		return 0
	}
	return int(config.fates[len(config.fates)-1].Lv) - int(config.maxLevel())
}

// itemExp is the development exp one of the ship's blueprints gives.
func (config blueprintConfig) itemExp() uint32 {
	return itemUses[config.StrengthenItem].blueprintExp
}

// blueprintProgress is a research ship's development (ShipBluePrint level,
// fateLevel, exp): fate is -1 until the last level, then the fate
// simulation phase.
type blueprintProgress struct {
	level uint32
	fate  int
	exp   uint32
}

// readBlueprintProgress is ShipBluePrint:updateInfo of the stored level
// (blue_print_level, the strength_list id: the level, plus the fate phase
// once at the last level) and exp.
func readBlueprintProgress(config blueprintConfig, stored, exp uint32) blueprintProgress {
	progress := blueprintProgress{level: min(stored, config.maxLevel()), fate: -1, exp: exp}
	if progress.level == config.maxLevel() {
		progress.fate = int(stored - progress.level)
	}
	return progress
}

// stored is the level as blue_print_level and strength_list send it.
func (progress blueprintProgress) stored() uint32 {
	return progress.level + uint32(max(progress.fate, 0))
}

// maxed is the client's "isMaxLevel and isMaxFateLevel": nothing left to
// develop.
func (progress blueprintProgress) maxed(config blueprintConfig) bool {
	return progress.level == config.maxLevel() && progress.fate >= config.maxFate()
}

// addExp is ShipBluePrint:addExp: exp levels the ship up while it reaches
// the next level's need_exp, then, past the last level, the fate
// simulation phases. What is left stays as exp.
func (progress blueprintProgress) addExp(config blueprintConfig, exp uint32) blueprintProgress {
	progress.exp += exp
	maxLevel := config.maxLevel()
	if progress.level < maxLevel {
		for progress.level < maxLevel && config.levels[progress.level].NeedExp <= progress.exp {
			progress.exp -= config.levels[progress.level].NeedExp
			progress.level++
		}
		if progress.level == maxLevel {
			progress.fate = 0
		}
	}
	if len(config.fates) > 0 && progress.fate >= 0 {
		for progress.fate < config.maxFate() && config.fates[progress.fate].NeedExp <= progress.exp {
			progress.exp -= config.fates[progress.fate].NeedExp
			progress.fate++
		}
	}
	return progress
}

// needLevel is the ship level the development progress needs: the
// current fate phase's need_lv, or the current level's
// (ModBluePrintCommand). fate false is PursuingBluePrintCommand's check,
// which only looks at the level.
func (progress blueprintProgress) needLevel(config blueprintConfig, fate bool) uint32 {
	if fate && progress.fate > 0 {
		return config.fates[progress.fate-1].NeedLv
	}
	return config.levels[max(progress.level, 1)-1].NeedLv
}

// pursueDiscount is TechnologyProxy.getPursuingDiscount: the percent of the
// price the nth catch-up blueprint of the day costs.
func pursueDiscount(table [][]uint32, n uint32) uint32 {
	i := len(table) - 1
	for i > 0 && n < table[i][0] {
		i--
	}
	return table[i][1]
}

// pursueCost is TechnologyProxy:calcPursuingCost: what count catch-up
// blueprints cost after the pursued ones bought today, the discounted ones
// first, the rest at full price.
func (config blueprintConfig) pursueCost(pursued, count uint32) uint32 {
	table := blueprintDiscountSSR
	if config.ur() {
		table = blueprintDiscountUR
	}
	var cost uint32
	last := table[len(table)-1][0]
	for n := pursued + 1; n < last && count > 0; n++ {
		cost += config.Price * pursueDiscount(table, n) / 100
		count--
	}
	return cost + count*config.Price
}
