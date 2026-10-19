package game

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Operation Siren's achievements (world_target_data, WorldAchievement):
// each entrance (world_chapter_colormask) has normal_target and
// cryptic_target achievements, each a list of {trigger, count}
// conditions; the stars of target_drop_show are earned by the number of
// them achieved and claimed with CS_33602 (WorldAchieveCommand). Built
// from the client's Lua and the tables alone (no capture of the mode).
//
// What a trigger counts is the server's: the tables only describe it
// (target_desc). The triggers are read by their description into what
// worldHappenings tells, counted on the active entrance (enter_map_id):
// sectors secured (OpReqPressingMap), victories (enemies beaten),
// meowfficer salvages ended, and the events of a world_event_data name
// started (their first effect run: resource nodes, merchants, switches,
// weather control devices, energy matrices, record files, data logging
// devices, promoted "Pawn" fleets). The others (reconnaissance in
// anomalous zones, the scanning facility, ambushes repelled, sources from
// enhanced fleets) aren't counted.

// worldTriggerKind is what a trigger counts.
type worldTriggerKind uint8

const (
	worldTriggerNone worldTriggerKind = iota
	worldTriggerPressing
	worldTriggerVictory
	worldTriggerSalvage
	worldTriggerEvent // worldTrigger.event started
)

// worldTriggerPatterns read a target_desc (EN) into what it counts; event
// is the world_event_data name of the events it counts.
var worldTriggerPatterns = []struct {
	desc  *regexp.Regexp
	kind  worldTriggerKind
	event string
}{
	{regexp.MustCompile(`(?i)^gain control of`), worldTriggerPressing, ""},
	{regexp.MustCompile(`(?i)\bvictor`), worldTriggerVictory, ""},
	{regexp.MustCompile(`(?i)meowfficer retrieval`), worldTriggerSalvage, ""},
	{regexp.MustCompile(`(?i)resource node`), worldTriggerEvent, "Resource Node"},
	{regexp.MustCompile(`(?i)mysterious merchant`), worldTriggerEvent, "Mysterious Merchant"},
	{regexp.MustCompile(`(?i)switch mechanism`), worldTriggerEvent, "Switch Mechanism"},
	{regexp.MustCompile(`(?i)weather control device`), worldTriggerEvent, "Siren Weather Control Device"},
	{regexp.MustCompile(`(?i)energy matri`), worldTriggerEvent, "Energy Matrix"},
	{regexp.MustCompile(`(?i)record file`), worldTriggerEvent, "Collection - File"},
	{regexp.MustCompile(`(?i)data logging`), worldTriggerEvent, "Siren Data Logging Device"},
	{regexp.MustCompile(`(?i)defeat .*promoted`), worldTriggerEvent, `Promoted "Pawn"`},
}

// worldTargetConfig is a world_target_data row.
type worldTargetConfig struct {
	Condition  [][2]uint32 `json:"condition"` // {trigger id, count}
	TargetDesc string      `json:"target_desc"`
}

// worldTargetEntrance is the achievements' part of a world_chapter_colormask
// row.
type worldTargetEntrance struct {
	NormalTarget  []uint32          `json:"normal_target"`
	CrypticTarget []uint32          `json:"cryptic_target"`
	DropShow      []json.RawMessage `json:"target_drop_show"` // {star, {type, id, count}}

	stars map[uint32][]uint32 // star -> its drop
}

// worldTrigger is a trigger of an achievement.
type worldTrigger struct {
	kind  worldTriggerKind
	event string
}

var (
	worldTargets         map[uint32]worldTargetConfig
	worldTargetEntrances map[uint32]*worldTargetEntrance
	worldTriggers        map[uint32]worldTrigger // by trigger id
	// worldEntranceTargetIDs are each entrance's achievements
	worldEntranceTargetIDs map[uint32][]uint32
)

func loadWorldAchieveData() error {
	var entrances map[uint32]worldTargetEntrance
	err := gamedata.LoadAll(
		gamedata.Load("world_target_data", &worldTargets),
		gamedata.Load("world_chapter_colormask", &entrances),
	)
	if err != nil {
		return err
	}
	worldTargetEntrances = map[uint32]*worldTargetEntrance{}
	worldEntranceTargetIDs = map[uint32][]uint32{}
	for id, entrance := range entrances {
		entrance.stars = map[uint32][]uint32{}
		for _, raw := range entrance.DropShow {
			var parts []json.RawMessage
			var star uint32
			var drop []uint32
			if json.Unmarshal(raw, &parts) != nil || len(parts) != 2 || json.Unmarshal(parts[0], &star) != nil || json.Unmarshal(parts[1], &drop) != nil || len(drop) != 3 {
				return gamedata.Missing("world_chapter_colormask", "readable target_drop_show for entrance %d", id)
			}
			entrance.stars[star] = drop
		}
		worldTargetEntrances[id] = &entrance
		worldEntranceTargetIDs[id] = append(slices.Clone(entrance.NormalTarget), entrance.CrypticTarget...)
	}
	worldTriggers = map[uint32]worldTrigger{}
	for _, target := range worldTargets {
		trigger := worldTrigger{}
		for _, pattern := range worldTriggerPatterns {
			if pattern.desc.MatchString(target.TargetDesc) {
				trigger = worldTrigger{pattern.kind, pattern.event}
				break
			}
		}
		for _, condition := range target.Condition {
			worldTriggers[condition[0]] = trigger
		}
	}
	return nil
}

// target is the achievement id's progress, nil when nothing was counted.
func (state *worldState) target(id uint32) *worldTarget {
	for i := range state.Targets {
		if state.Targets[i].ID == id {
			return &state.Targets[i]
		}
	}
	return nil
}

// targetInfo is the achievement as WORLDTARGET.
func (target *worldTarget) info() *protobuf.WORLDTARGET {
	info := &protobuf.WORLDTARGET{Id: proto.Uint32(target.ID)}
	for _, process := range target.Process {
		info.ProcessList = append(info.ProcessList, &protobuf.WORLDTARGET_PROCESS{TriggerId: proto.Uint32(process.ID), Count: proto.Uint32(process.Count)})
	}
	return info
}

// achieved is WorldAchievement:IsAchieved: every condition's count reached.
func (state *worldState) achieved(id uint32) bool {
	config, ok := worldTargets[id]
	if !ok || len(config.Condition) == 0 {
		return false
	}
	target := state.target(id)
	if target == nil {
		return false
	}
	for _, condition := range config.Condition {
		i := slices.IndexFunc(target.Process, func(p worldCount) bool { return p.ID == condition[0] })
		if i < 0 || target.Process[i].Count < condition[1] {
			return false
		}
	}
	return true
}

// worldTriggerCounts are the counts what happened adds, by trigger kind
// (and event name).
func worldTriggerCounts(happened *worldHappenings) (map[worldTriggerKind]uint32, map[string]uint32) {
	kinds := map[worldTriggerKind]uint32{
		worldTriggerPressing: uint32(len(happened.Pressed)),
		worldTriggerVictory:  uint32(len(happened.Kills)),
		worldTriggerSalvage:  uint32(len(happened.Salvaged)),
	}
	events := map[string]uint32{}
	var seen []uint32
	for _, id := range happened.Events {
		event, ok := worldEvents[id]
		if !ok || len(event.Effect) == 0 || slices.Contains(seen, id) || !slices.Contains(happened.Effects, event.Effect[0]) {
			continue
		}
		seen = append(seen, id)
		events[event.Name]++
	}
	return kinds, events
}

// worldTargetsUpdated is the achievements' hook, called once per map
// operation (its result is SC_33104.target_list) and world battle (pushed
// as SC_33601): what happened is counted on the active entrance's
// achievements, up to each condition's count; it returns the achievements
// that changed, whole (WorldAchievement:NetUpdate reads every trigger it
// is sent, which must be one of its conditions).
func (tx *Tx) worldTargetsUpdated(state *worldState, happened *worldHappenings) []*protobuf.WORLDTARGET {
	kinds, events := worldTriggerCounts(happened)
	var changed []*protobuf.WORLDTARGET
	for _, id := range worldEntranceTargetIDs[state.EnterMapID] {
		config, ok := worldTargets[id]
		if !ok {
			continue
		}
		target := state.target(id)
		dirty := false
		for _, condition := range config.Condition {
			trigger := worldTriggers[condition[0]]
			delta := kinds[trigger.kind]
			if trigger.kind == worldTriggerEvent {
				delta = events[trigger.event]
			}
			if trigger.kind == worldTriggerNone || delta == 0 {
				continue
			}
			if target == nil {
				state.Targets = append(state.Targets, worldTarget{ID: id})
				target = &state.Targets[len(state.Targets)-1]
			}
			i := slices.IndexFunc(target.Process, func(p worldCount) bool { return p.ID == condition[0] })
			if i < 0 {
				target.Process = append(target.Process, worldCount{ID: condition[0]})
				i = len(target.Process) - 1
			}
			if count := min(target.Process[i].Count+delta, condition[1]); count != target.Process[i].Count {
				target.Process[i].Count = count
				dirty = true
			}
		}
		if dirty {
			changed = append(changed, target.info())
		}
	}
	return changed
}

// handleWorldAchieve answers WorldAchieveCommand (CS_33602, the atlas'
// achievement panel or the collection's, World:GetFinishAchievements): the
// stars star_list of each entrance id are claimed, each earned once as
// many of the entrance's achievements are achieved as the star
// (CountAchievements, normal and cryptic) and not claimed before
// (target_fetch_list); drops are their target_drop_show rewards
// (PlayerConst.addTranDrop: world items into the world inventory). Errors
// show errorTip("world_achieve_error_", result).
func (game *Game) handleWorldAchieve(client *connection.Client, req *protobuf.CS_33602) error {
	ctx := context.Background()
	now := serverNow()
	reply := &protobuf.SC_33603{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		var drops []*protobuf.DROPINFO
		for _, claim := range req.GetList() {
			entrance := worldTargetEntrances[claim.GetId()]
			if entrance == nil || len(claim.GetStarList()) == 0 {
				return errRefused
			}
			achieved := uint32(0)
			for _, id := range worldEntranceTargetIDs[claim.GetId()] {
				if state.achieved(id) {
					achieved++
				}
			}
			i := slices.IndexFunc(state.TargetFetches, func(f worldTargetFetch) bool { return f.ID == claim.GetId() })
			if i < 0 {
				state.TargetFetches = append(state.TargetFetches, worldTargetFetch{ID: claim.GetId()})
				i = len(state.TargetFetches) - 1
			}
			fetch := &state.TargetFetches[i]
			for _, star := range claim.GetStarList() {
				drop, ok := entrance.stars[star]
				if !ok || star > achieved || slices.Contains(fetch.Stars, star) {
					return errRefused
				}
				fetch.Stars = append(fetch.Stars, star)
				drops = append(drops, dropInfo(drop[0], drop[1], drop[2]))
			}
		}
		if len(drops) == 0 {
			return errRefused
		}
		if err := tx.grantWorldDrops(state, drops); err != nil {
			return err
		}
		reply.Drops = drops
		return tx.saveWorldState(state)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_33603{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}
