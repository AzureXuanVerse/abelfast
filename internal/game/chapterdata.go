package game

import (
	"encoding/json"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The campaign's game data, built from the config tables chapter_template,
// expedition_data_template, item_virtual_data_statistics, ship_level,
// energy_template and gameset.

// chapter_template.grids cell kinds (the 4th value of a grid)
const (
	gridBorn  = 1
	gridEnemy = 6 // an enemy spawns here (expedition_id_weight_list)
	gridBoss  = 8 // the boss spawns here (boss_expedition_id)
)

type chapterGrid struct {
	row, column uint32
	walkable    bool
	kind        uint32
}

type weightedExpedition struct {
	id, weight uint32
}

type chapterWinCondition struct {
	kind, param uint32
}

// ambushRatio is a chapter_template.ambush_ratio_extra row, ratio in 1/10000;
// a row without a cell applies everywhere.
type ambushRatio struct {
	row, column uint32
	anyCell     bool
	ratio       int
}

type chapterTemplate struct {
	id            uint32
	oil           uint32 // oil spent to sortie
	time          uint32 // seconds the sortie stays open
	ammoTotal     uint32
	progressBoss  uint32 // progress gained per boss kill
	grids         []chapterGrid
	enemies       []weightedExpedition // expedition_id_weight_list
	enemyRefresh  int                  // enemies spawned at start, enemy_refresh[0]
	bosses        []uint32             // boss_expedition_id
	bossRefresh   uint32               // kills before the boss appears
	isAmbush      bool
	ambushes      []uint32 // ambush_expedition_list
	ambushRatios  []ambushRatio
	avoidRatio    uint32 // how hard ambushes are to evade
	winConditions []chapterWinCondition
	awards        []uint32 // award virtual items, see dropPools
}

var chapterTemplates map[uint32]*chapterTemplate

// chapter_template row
type chapterConfig struct {
	Oil          uint32 `json:"oil"`
	Time         uint32 `json:"time"`
	AmmoTotal    uint32 `json:"ammo_total"`
	ProgressBoss uint32 `json:"progress_boss"`
	// [row, column, walkable, kind], walkable a boolean or 0/1
	Grids                  [][4]any   `json:"grids"`
	ExpeditionIDWeightList [][]uint32 `json:"expedition_id_weight_list"`
	EnemyRefresh           []int      `json:"enemy_refresh"`
	BossExpeditionID       []uint32   `json:"boss_expedition_id"`
	BossRefresh            uint32     `json:"boss_refresh"`
	IsAmbush               uint32     `json:"is_ambush"`
	AmbushExpeditionList   []uint32   `json:"ambush_expedition_list"`
	AmbushRatioExtra       [][]int    `json:"ambush_ratio_extra"`
	AvoidRatio             uint32     `json:"avoid_ratio"`
	// [kind, param], param a cell for the kinds that name one
	WinCondition [][2]any    `json:"win_condition"`
	Awards       [][2]uint32 `json:"awards"` // [type, virtual item]
}

// expedition_data_template.type
const (
	expeditionTypeMulBoss = 94
	expeditionTypeBoss    = 99
)

// expeditionTemplate is an expedition_data_template row.
type expeditionTemplate struct {
	Exp  uint32 `json:"exp"` // base ship exp
	Type uint32 `json:"type"`
}

var expeditionTemplates map[uint32]expeditionTemplate

func (expedition expeditionTemplate) isBoss() bool {
	return expedition.Type == expeditionTypeBoss || expedition.Type == expeditionTypeMulBoss
}

type dropRef struct {
	kind, id uint32
}

// dropPools are the "chance to receive" award items of
// item_virtual_data_statistics, display_icon listing what they can turn into.
var dropPools map[uint32][]dropRef

// item_virtual_data_statistics row: display_icon is a list of
// [type, id, count], or "" when there is none
type virtualItemConfig struct {
	DisplayIcon json.RawMessage `json:"display_icon"`
	LinkID      uint32          `json:"link_id"`
}

// Specialized Core: virtual item 59011 (virtual_type 20) is stored as the
// item that is its link_id, capped by gameset urpt_chapter_max.
const vitemSpecializedCore = 59011

var itemSpecializedCore uint32

// gameset values
var (
	flagshipExpPercent  uint32 // flagship_exp_percent
	mvpExpPercent       uint32 // mvp_exp_percent
	battleConsumeEnergy uint32 // battle_consume_energy
)

// gamesetValues are the gameset settings loadChapterData reads.
var gamesetValues = map[string]*uint32{
	"flagship_exp_percent":  &flagshipExpPercent,
	"mvp_exp_percent":       &mvpExpPercent,
	"battle_consume_energy": &battleConsumeEnergy,
}

// energyHappy is the energy_template row of the highest morale, which
// gives an exp bonus.
const energyHappy = 4

// happyEnergyFrom is the morale from which a ship is happy
// (energy_template[energyHappy].lower_bound).
var happyEnergyFrom uint32

// energy_template row
type energyConfig struct {
	LowerBound uint32 `json:"lower_bound"`
}

// shipLevelExp is ship_level exp by level: shipLevelExp[n-1] takes a ship
// from level n to n+1.
var shipLevelExp []uint32

// ship_level row
type shipLevelConfig struct {
	Level uint32 `json:"level"`
	Exp   uint32 `json:"exp"`
}

// loadChapterData builds every chapter of chapter_template, the expeditions
// fought in them, the award pools and the ship level curve.
func loadChapterData() error {
	var chapters map[uint32]chapterConfig
	var items map[uint32]virtualItemConfig
	var levels map[uint32]shipLevelConfig
	var energies map[uint32]energyConfig
	var gameset map[string]uint32
	err := gamedata.LoadAll(
		gamedata.Load("chapter_template", &chapters),
		gamedata.Load("expedition_data_template", &expeditionTemplates),
		gamedata.Load("item_virtual_data_statistics", &items),
		gamedata.Load("ship_level", &levels),
		gamedata.Load("energy_template", &energies),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
	)
	if err != nil {
		return err
	}
	for key, value := range gamesetValues {
		setting, ok := gameset[key]
		if !ok {
			return gamedata.Missing("gameset", "%s", key)
		}
		*value = setting
	}
	energy, ok := energies[energyHappy]
	if !ok {
		return gamedata.Missing("energy_template", "row %d", energyHappy)
	}
	happyEnergyFrom = energy.LowerBound
	if itemSpecializedCore = items[vitemSpecializedCore].LinkID; itemSpecializedCore == 0 {
		return gamedata.Missing("item_virtual_data_statistics", "link_id for %d", vitemSpecializedCore)
	}
	chapterTemplates = make(map[uint32]*chapterTemplate, len(chapters))
	for id, row := range chapters {
		template := &chapterTemplate{
			id:           id,
			oil:          row.Oil,
			time:         row.Time,
			ammoTotal:    row.AmmoTotal,
			progressBoss: row.ProgressBoss,
			bosses:       row.BossExpeditionID,
			bossRefresh:  row.BossRefresh,
			isAmbush:     row.IsAmbush != 0,
			ambushes:     row.AmbushExpeditionList,
			avoidRatio:   row.AvoidRatio,
		}
		for _, grid := range row.Grids {
			cellRow, _ := grid[0].(float64)
			cellColumn, _ := grid[1].(float64)
			kind, _ := grid[3].(float64)
			walkable, ok := grid[2].(bool)
			if !ok {
				number, _ := grid[2].(float64)
				walkable = number != 0
			}
			template.grids = append(template.grids, chapterGrid{uint32(cellRow), uint32(cellColumn), walkable, uint32(kind)})
		}
		for _, entry := range row.ExpeditionIDWeightList {
			if len(entry) >= 2 {
				template.enemies = append(template.enemies, weightedExpedition{entry[0], entry[1]})
			}
		}
		if len(row.EnemyRefresh) > 0 {
			template.enemyRefresh = row.EnemyRefresh[0]
		}
		for _, extra := range row.AmbushRatioExtra {
			switch len(extra) {
			case 1:
				template.ambushRatios = append(template.ambushRatios, ambushRatio{anyCell: true, ratio: extra[0]})
			case 3:
				template.ambushRatios = append(template.ambushRatios, ambushRatio{row: uint32(extra[0]), column: uint32(extra[1]), ratio: extra[2]})
			}
		}
		for _, condition := range row.WinCondition {
			kind, _ := condition[0].(float64)
			param, _ := condition[1].(float64)
			template.winConditions = append(template.winConditions, chapterWinCondition{uint32(kind), uint32(param)})
		}
		for _, award := range row.Awards {
			template.awards = append(template.awards, award[1])
		}
		chapterTemplates[id] = template
	}

	dropPools = map[uint32][]dropRef{}
	for id, row := range items {
		var icons [][]uint32
		if json.Unmarshal(row.DisplayIcon, &icons) != nil {
			continue
		}
		for _, icon := range icons {
			if len(icon) >= 2 {
				dropPools[id] = append(dropPools[id], dropRef{icon[0], icon[1]})
			}
		}
	}

	maxLevel := uint32(0)
	for _, row := range levels {
		maxLevel = max(maxLevel, row.Level)
	}
	shipLevelExp = make([]uint32, maxLevel)
	for _, row := range levels {
		if row.Level > 0 {
			shipLevelExp[row.Level-1] = row.Exp
		}
	}
	return nil
}
