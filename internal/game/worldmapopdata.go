package game

import (
	"encoding/json"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The tables the map operations (CS_33103) and the world's battles read
// beyond loadWorldData's, loaded by loadWorldOpData.

// worldTemplateReset is a world_chapter_template_reset row: a map the
// fleets can jump out of (OpReqJumpOut), back to an entrance.
type worldTemplateReset struct {
	ResetItem          []uint32 `json:"reset_item"`          // world items lost on the way out
	TransportColormask uint32   `json:"transport_colormask"` // the entrance jumped out to
}

// worldEventCompleteCount is world_event_complete's chapter_num: how many
// entrances an activation gives that zone securing reward.
type worldEventCompleteCount struct {
	ChapterNum uint32 `json:"chapter_num"`
}

var (
	worldEventCompleteCounts map[uint32]uint32
	worldTemplateResets      map[uint32]worldTemplateReset

	worldOpSettings struct {
		deathHPFix  uint32 // world_death_hpfix: a sunk ship's hp_rant after the battle
		damageLevel uint32 // len(world_buff_morale): the highest fleet damage level
		moveStep    uint32 // world_move_initial_step: a fleet's moves per turn
		// world_instruction_submarine / _detect: {cost, cooldown}, the
		// cost falling with the time since the last use (World.CalcCDTimeCost)
		submarineCost, visionCost [2]uint32
		// world_instruction_maintenance: {cost, cooldown, ...}
		maintenanceCost []uint32
		rookieMaps      []uint32 // world_guide_map_list (WorldConst.IsRookieMap)
	}
)

// loadWorldOpData reads the map operations' tables.
func loadWorldOpData() error {
	var gameset map[string]uint32
	var completes map[uint32]worldEventCompleteCount
	var morale []json.RawMessage
	err := gamedata.LoadAll(
		gamedata.Load("world_event_complete", &completes),
		gamedata.Load("world_chapter_template_reset", &worldTemplateResets),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
		func() error { return gamedata.GamesetDescription("world_buff_morale", &morale) },
		func() error {
			return gamedata.GamesetDescription("world_instruction_submarine", &worldOpSettings.submarineCost)
		},
		func() error {
			return gamedata.GamesetDescription("world_instruction_detect", &worldOpSettings.visionCost)
		},
		func() error {
			return gamedata.GamesetDescription("world_instruction_maintenance", &worldOpSettings.maintenanceCost)
		},
		func() error {
			return gamedata.GamesetDescription("world_guide_map_list", &worldOpSettings.rookieMaps)
		},
	)
	if err != nil {
		return err
	}
	worldEventCompleteCounts = make(map[uint32]uint32, len(completes))
	for id, row := range completes {
		worldEventCompleteCounts[id] = row.ChapterNum
	}
	worldOpSettings.damageLevel = uint32(len(morale))
	worldOpSettings.deathHPFix = gameset["world_death_hpfix"]
	worldOpSettings.moveStep = gameset["world_move_initial_step"]
	if worldOpSettings.deathHPFix == 0 || worldOpSettings.moveStep == 0 || len(worldOpSettings.maintenanceCost) == 0 {
		return gamedata.Missing("gameset", "world_death_hpfix / world_move_initial_step / world_instruction_maintenance")
	}
	return nil
}

// worldCDCost is World.CalcCDTimeCost: cost[0] points, less in proportion
// to the part of the cooldown cost[1] elapsed since last.
func worldCDCost(cost [2]uint32, last, now int64) uint32 {
	if cost[1] == 0 {
		return cost[0]
	}
	left := max(int64(cost[1])-max(now-last, 0), 0)
	return uint32(int64(cost[0]) * left / int64(cost[1]))
}
