package game

import (
	"slices"
	"time"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The META Showdown (Operation Siren's world boss, WorldBossProxy /
// WorldBossConst) and the META Lab's Sync Points (MetaCharacterProxy).

// The virtual items (item_virtual_data_statistics, virtual_type 14) the
// bosses are summoned with: Drop's virtual_type 14 case (model/vo/drop.lua)
// adds them to WorldBossProxy's summon_pt / summon_pt_old.
const (
	vitemWorldBossPt     = 100000 // WorldBossConst.WORLD_BOSS_ITEM_ID, META Coordinates
	vitemWorldBossPastPt = 100002 // WORLD_PAST_BOSS_ITEM_ID, Dossier Coordinate Data
)

// worldBossMaxLevel is the highest level of a boss: each boss has that many
// world_boss_level rows from its boss_level_id, a Dossier boss is always
// fought at it (BattleGateWorldBossSimulate: boss_level_id + 14, level 15)
// and "the difficulty of subsequent META Showdowns will increase by 1 level,
// up to a maximum of 15" (gametip world_boss_help_meta).
const worldBossMaxLevel = 15

// worldBossKillPt is the Sync Points of a boss killed at level: no client
// table holds them (the boss's drop_show_self only names the Sync Point
// resource). The players' figure for a level 15 kill is 375, so 25 a level.
func worldBossKillPt(level uint32) uint32 {
	return 25 * level
}

// world_joint_boss_template row: a META ship's boss. state is the season of
// a current boss ("always" for the Dossier Analysis bosses).
type worldBossConfig struct {
	ID          uint32          `json:"id"`
	BossLevelID uint32          `json:"boss_level_id"`
	MetaID      uint32          `json:"meta_id"`
	State       gamedata.Window `json:"state"`
}

// world_boss_level row: the boss at one level.
type worldBossLevelConfig struct {
	HP           uint32 `json:"hp"`
	EnemyID      uint32 `json:"enemy_id"`
	ExpeditionID uint32 `json:"expedition_id"`
}

// worldBossTicket is a gameset curr_boss_ticket / past_joint_boss_ticket
// description: the points a summon costs, the most gained a day, the most
// stored (WorldBossConst.GetCurrBossConsume).
type worldBossTicket struct {
	consume, dailyMax, capacity uint32
}

var (
	worldBossTemplates map[uint32]worldBossConfig
	worldBossLevels    map[uint32]worldBossLevelConfig
	worldBossCurrent   worldBossTicket // curr_boss_ticket
	worldBossPast      worldBossTicket // past_joint_boss_ticket
	// joint_boss_oil_consume: the oil of the n-th battle against one's own
	// boss, the last value for any later one (GetBossOilConsume)
	worldBossOil []uint32
	// gameset values, in seconds
	worldBossLifetime    uint32 // joint_boss_bonus_time: a boss lasts 24 hours
	worldBossSupportTime uint32 // joint_boss_world_time: a support request
	worldBossAutoTime    uint32 // past_joint_boss_autofight_time: Dossier auto-deploy
)

func loadWorldBossData() error {
	var gameset map[string]uint32
	var current, past []uint32
	err := gamedata.LoadAll(
		gamedata.Load("world_joint_boss_template", &worldBossTemplates),
		gamedata.Load("world_boss_level", &worldBossLevels),
		gamedata.Load("ship_strengthen_meta", &metaShips),
		func() error { return gamedata.GamesetDescription("curr_boss_ticket", &current) },
		func() error { return gamedata.GamesetDescription("past_joint_boss_ticket", &past) },
		func() error { return gamedata.GamesetDescription("joint_boss_oil_consume", &worldBossOil) },
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
	)
	if err != nil {
		return err
	}
	if len(current) < 3 || len(past) < 3 || len(worldBossOil) == 0 {
		return gamedata.Missing("gameset", "META Showdown tickets / oil")
	}
	worldBossCurrent = worldBossTicket{current[0], current[1], current[2]}
	worldBossPast = worldBossTicket{past[0], past[1], past[2]}
	for key, value := range map[string]*uint32{
		"joint_boss_bonus_time":          &worldBossLifetime,
		"joint_boss_world_time":          &worldBossSupportTime,
		"past_joint_boss_autofight_time": &worldBossAutoTime,
	} {
		setting, ok := gameset[key]
		if !ok {
			return gamedata.Missing("gameset", "%s", key)
		}
		*value = setting
	}
	for id, row := range worldBossTemplates {
		if _, ok := metaShips[row.MetaID]; !ok {
			return gamedata.Missing("ship_strengthen_meta", "META ship %d of boss %d", row.MetaID, id)
		}
		for level := uint32(1); level <= worldBossMaxLevel; level++ {
			if _, ok := worldBossLevels[row.BossLevelID+level-1]; !ok {
				return gamedata.Missing("world_boss_level", "boss %d level %d", id, level)
			}
		}
	}
	return nil
}

// currentWorldBoss is the boss of the season running at now, 0 when none
// (WorldBossConst.GetCurrBossID: the last template whose state is a running
// timer).
func currentWorldBoss(now time.Time) uint32 {
	ids := make([]uint32, 0, len(worldBossTemplates))
	for id := range worldBossTemplates {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range slices.Backward(ids) {
		state := worldBossTemplates[id].State
		if !state.Always && state.Open(now) {
			return id
		}
	}
	return 0
}

// archivedWorldBoss is whether template is a Dossier Analysis boss
// (WorldBossConst.GetAchieveBossList: state "always").
func archivedWorldBoss(template uint32) bool {
	config, ok := worldBossTemplates[template]
	return ok && config.State.Always
}

// worldBossLevel is template's boss at level.
func worldBossLevel(template, level uint32) worldBossLevelConfig {
	return worldBossLevels[worldBossTemplates[template].BossLevelID+level-1]
}

// worldBossOilCost is the oil of the n-th battle (from 1) against one's own
// boss.
func worldBossOilCost(n uint32) uint32 {
	return worldBossOil[min(int(n), len(worldBossOil))-1]
}
