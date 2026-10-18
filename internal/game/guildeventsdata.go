package game

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// guild_operation_template row: an operation the guild starts for consume
// guild funds once at unlock_guild_level (GuildEvent)
type guildOperationConfig struct {
	ID               uint32 `json:"id"`
	Consume          uint32 `json:"consume"`
	UnlockGuildLevel uint32 `json:"unlock_guild_level"`
}

// guild_base_event row: a mission of an operation (GuildMission). Its id's
// thousands are the operation; type 1 is common, 2 elite; sub_type 1 a main
// mission (the tree's spine), 2 a side one. workload is the work to do
// (guildMissionSpeed), node_position where its nodes sit (percent).
// event_attr_acc_effect is [[attribute, op, goal, score]] (the attribute's
// total over the dispatched ships), event_attr_count_effect [[attribute,
// value, goal, score]] (ships with the attribute at value or above).
type guildBaseEventConfig struct {
	ID              uint32     `json:"id"`
	Type            uint32     `json:"type"`
	SubType         uint32     `json:"sub_type"`
	Workload        uint32     `json:"workload"`
	NodePosition    []uint32   `json:"node_position"`
	BasePoint       uint32     `json:"base_point"`
	ResultPoint     []uint32   `json:"result_point"`
	AwardListReport [][]uint32 `json:"award_list_report"`
	AttrAcc         [][]uint32 `json:"event_attr_acc_effect"`
	AttrCount       [][]uint32 `json:"event_attr_count_effect"`
}

// guild_event_node row: what a node gives (GuildMissionNode); item
// "sairendanchuan" is a battle, "box" a supply box
type guildEventNodeConfig struct {
	ID           uint32     `json:"id"`
	Item         string     `json:"item"`
	SuccessAward [][]uint32 `json:"success_award"`
	FailAward    [][]uint32 `json:"fail_award"`
}

// guild_boss_event row: an operation's boss (GuildBossMission);
// expedition_id is [stage, [enemy]]
type guildBossEventConfig struct {
	ID           uint32          `json:"id"`
	ExpeditionID json.RawMessage `json:"expedition_id"`
	AwardReport  [][]uint32      `json:"award_report"`
}

// guildBoss is a boss as the operations use it: the stage fought
// (GuildBossMission:GetStageID), the guild-wide hp, the report's rewards.
type guildBoss struct {
	stage, hp uint32
	awards    [][]uint32
}

// guildOperationContent is what an operation's tree is drawn from: its
// common and elite main missions, side missions, bosses and nodes.
type guildOperationContent struct {
	common, elite, side []uint32
	bosses              []uint32
	battleNodes         []uint32
	boxNodes            []uint32
}

// what the enemy_data_statistics rows hold that the guild boss needs
type enemyDurabilityConfig struct {
	Durability uint32 `json:"durability"`
}

// the GuildMission.MISSION_* and guildset values the operations use
const (
	guildMissionMaxFleets = 4 // GuildConst.MISSION_MAX_FLEET_CNT
	guildMissionMaxShips  = 4 // GuildConst.MISSION_MAX_SHIP_CNT
	// guildOperationPositions is how many main missions an operation's
	// tree has before its boss. The client takes the tree from the server
	// and no table gives a count: 5 is belfast's choice.
	guildOperationPositions = 5
)

var (
	guildOperations      map[uint32]guildOperationConfig
	guildBaseEvents      map[uint32]guildBaseEventConfig
	guildEventNodes      map[uint32]guildEventNodeConfig
	guildBosses          map[uint32]guildBoss
	guildOperationPlans  map[uint32]*guildOperationContent // by operation id
	guildShipAttrs       map[uint32]shipStatisticsConfig   // ship_data_statistics attrs
	guildOperationLength uint32                            // operation_duration_time
	// the speed a mission's efficiency gives (guildMissionSpeed)
	guildEfficiencyBase, guildEfficiencyNormal, guildEfficiencyElite, guildEfficiencyMax uint32
	guildJoinsPerMonth                                                                   uint32 // efficiency_param_times
	guildJoinLiveness, guildBossLiveness                                                 uint32
	guildBossDailyCount                                                                  uint32 // operation_daily_boss_count
	guildBossOil                                                                         uint32 // use_oil
	guildDamageResource                                                                  uint32 // guild_damage_resource: the Damage Rating resource
	guildReportMax                                                                       uint32 // operation_report_max
	guildDispatchResets                                                                  []uint32
)

// loadGuildEventData reads the operation tables and guildset values, after
// loadGuildData (guildSet).
func loadGuildEventData() error {
	var bosses map[uint32]guildBossEventConfig
	var enemies map[uint32]enemyDurabilityConfig
	err := gamedata.LoadAll(
		gamedata.Load("guild_operation_template", &guildOperations),
		gamedata.Load("guild_base_event", &guildBaseEvents),
		gamedata.Load("guild_event_node", &guildEventNodes),
		gamedata.Load("guild_boss_event", &bosses),
		gamedata.Load("enemy_data_statistics", &enemies),
		gamedata.Load("ship_data_statistics", &guildShipAttrs),
	)
	if err != nil {
		return err
	}
	for key, value := range map[string]*uint32{
		"operation_duration_time":      &guildOperationLength,
		"efficiency_param_base":        &guildEfficiencyBase,
		"efficiency_param_normal":      &guildEfficiencyNormal,
		"efficiency_param_elite":       &guildEfficiencyElite,
		"efficiency_param_max":         &guildEfficiencyMax,
		"efficiency_param_times":       &guildJoinsPerMonth,
		"operation_event_guild_active": &guildJoinLiveness,
		"operation_boss_guild_active":  &guildBossLiveness,
		"operation_daily_boss_count":   &guildBossDailyCount,
		"guild_damage_resource":        &guildDamageResource,
		"use_oil":                      &guildBossOil,
		"operation_report_max":         &guildReportMax,
	} {
		if err := guildSetting(key, value); err != nil {
			return err
		}
	}
	guildDispatchResets = gamedata.IDs(guildSet["operation_member_dispatch_reset"].KeyArgs)
	slices.Sort(guildDispatchResets)
	if len(guildDispatchResets) == 0 {
		return gamedata.Missing("guildset", "operation_member_dispatch_reset key_args")
	}

	guildOperationPlans = map[uint32]*guildOperationContent{}
	plan := func(operation uint32) *guildOperationContent {
		content := guildOperationPlans[operation]
		if content == nil {
			content = &guildOperationContent{}
			guildOperationPlans[operation] = content
		}
		return content
	}
	for _, id := range slices.Sorted(maps.Keys(guildBaseEvents)) {
		event := guildBaseEvents[id]
		content := plan(id / 1000)
		switch {
		case event.SubType != 1:
			content.side = append(content.side, id)
		case event.Type == 2:
			content.elite = append(content.elite, id)
		default:
			content.common = append(content.common, id)
		}
	}
	// nodes come in four tiers of 20 by reward, battles from 1, boxes
	// from 1001: operation n draws from tier n
	for _, id := range slices.Sorted(maps.Keys(guildEventNodes)) {
		content := plan((id%1000-1)/20 + 1)
		if guildEventNodes[id].Item == "box" {
			content.boxNodes = append(content.boxNodes, id)
		} else {
			content.battleNodes = append(content.battleNodes, id)
		}
	}
	guildBosses = map[uint32]guildBoss{}
	for _, id := range slices.Sorted(maps.Keys(bosses)) {
		expedition := gamedata.IDs(bosses[id].ExpeditionID)
		if len(expedition) < 2 {
			return gamedata.Missing("guild_boss_event", "%d expedition_id", id)
		}
		// the boss's hp is its enemy's durability (the battle's), shared
		// by the guild
		enemy, ok := enemies[expedition[1]]
		if !ok {
			return gamedata.Missing("enemy_data_statistics", "%d", expedition[1])
		}
		guildBosses[id] = guildBoss{stage: expedition[0], hp: max(enemy.Durability, 1), awards: bosses[id].AwardReport}
		// stages 7000x belong to operation 1, 7001x to 2...
		operation := expedition[0]/10%10 + 1
		plan(operation).bosses = append(plan(operation).bosses, id)
	}
	for id := range guildOperations {
		content := guildOperationPlans[id]
		if content == nil || len(content.common)+len(content.elite) == 0 || len(content.bosses) == 0 {
			return gamedata.Missing("guild_base_event", "missions or boss of operation %d", id)
		}
	}
	return nil
}
