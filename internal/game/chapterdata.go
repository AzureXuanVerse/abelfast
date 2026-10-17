package game

// The slices of the game data the campaign needs, copied from the EN config
// tables (sharecfgdata/chapter_template, expedition_data_template,
// item_virtual_data_statistics, ShareCfg/ship_level, gameset). Only the
// chapters listed here can be played; add more by copying their rows.

// chapter_template.grids cell kinds (the 4th value of a grid)
const (
	gridNone  = 0
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
	winConditions []chapterWinCondition
	awards        []uint32 // award virtual items, see dropPools
}

var chapterTemplates = map[uint32]*chapterTemplate{
	// 1-1 Offshore Exercises
	101: {
		id:           101,
		oil:          10,
		time:         99999999,
		ammoTotal:    5,
		progressBoss: 100,
		grids: []chapterGrid{
			{4, 7, true, gridBoss}, {4, 6, true, gridEnemy}, {4, 5, true, gridNone}, {4, 4, true, gridNone},
			{4, 3, true, gridNone}, {4, 2, true, gridNone}, {4, 1, true, gridBorn},
		},
		enemies:       []weightedExpedition{{101010, 160}, {101011, 160}, {101012, 160}},
		enemyRefresh:  1,
		bosses:        []uint32{101000},
		bossRefresh:   1,
		isAmbush:      true,
		ambushes:      []uint32{101210},
		ambushRatios:  []ambushRatio{{anyCell: true, ratio: -20000}, {row: 4, column: 3, ratio: 30000}},
		winConditions: []chapterWinCondition{{1, 1}},
		awards:        []uint32{56001, 54011, 54021, 59001},
	},
}

// expedition_data_template.type
const (
	expeditionTypeMulBoss = 94
	expeditionTypeBoss    = 99
)

type expeditionTemplate struct {
	exp   uint32 // base ship exp
	kind  uint32
	level uint32
}

var expeditionTemplates = map[uint32]expeditionTemplate{
	101010: {80, 1, 1},
	101011: {80, 1, 1},
	101012: {80, 1, 1},
	101210: {80, 1, 1},
	101000: {150, expeditionTypeBoss, 2},
}

func (expedition expeditionTemplate) isBoss() bool {
	return expedition.kind == expeditionTypeBoss || expedition.kind == expeditionTypeMulBoss
}

type dropRef struct {
	kind, id uint32
}

// dropPools are the "chance to receive" award items of
// item_virtual_data_statistics, display_icon listing what they can turn into.
var dropPools = map[uint32][]dropRef{
	56001: {{dropTypeShip, 101061}, {dropTypeShip, 101071}, {dropTypeShip, 201081}, {dropTypeShip, 201091}, {dropTypeShip, 201101}}, // Mystery Ship
	54011: {{dropTypeItem, 17001}, {dropTypeItem, 17011}, {dropTypeItem, 17021}, {dropTypeItem, 17031}, {dropTypeItem, 17041}},      // T1 Mystery Gear Part
	54021: {{dropTypeItem, 30011}, {dropTypeItem, 30021}, {dropTypeItem, 30031}, {dropTypeItem, 30041}},                             // Mystery T1 Tech Pack
}

// Specialized Core: virtual item 59011 (virtual_type 20) is stored as item
// 59010 (its link_id), capped by gameset urpt_chapter_max.
const (
	vitemSpecializedCore = 59011
	itemSpecializedCore  = 59010
)

// gameset values
const (
	flagshipExpPercent  = 150 // flagship_exp_percent
	mvpExpPercent       = 200 // mvp_exp_percent
	battleConsumeEnergy = 2   // battle_consume_energy
)

// shipLevelExp is ShareCfg/ship_level exp by level: shipLevelExp[n-1] takes a
// ship from level n to n+1.
var shipLevelExp = []uint32{
	100, 200, 300, 400, 500, 600, 700, 800, 900, 1000,
	1100, 1200, 1300, 1400, 1500, 1600, 1700, 1800, 1900, 2000,
	2100, 2200, 2300, 2400, 2500, 2600, 2700, 2800, 2900, 3000,
	3100, 3200, 3300, 3400, 3500, 3600, 3700, 3800, 3900, 4000,
	4200, 4400, 4600, 4800, 5000, 5200, 5400, 5600, 5800, 6000,
	6200, 6400, 6600, 6800, 7000, 7200, 7400, 7600, 7800, 8000,
	8300, 8600, 8900, 9200, 9500, 9800, 10100, 10400, 10700, 12100,
	12540, 12980, 13420, 13860, 14300, 14740, 15180, 15620, 16060, 17250,
	17825, 18400, 18975, 19550, 20125, 20700, 21275, 21850, 22425, 24000,
	25200, 26400, 28800, 31200, 36000, 42000, 48000, 72000, 158400, 50000,
	53000, 56000, 59000, 62000, 65000, 71000, 77000, 83000, 89000, 95000,
	105000, 115000, 125000, 135000, 145000, 160000, 175000, 190000, 205000, 220000,
	241000, 262000, 283000, 304000, 0,
}
