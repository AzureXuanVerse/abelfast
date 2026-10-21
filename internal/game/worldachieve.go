package game

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"unicode"

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
// (target_desc, translated in every region; condition_text is empty and
// the trigger id is the achievement's own). The triggers are read by
// their description (worldTriggerAnchors) into what worldHappenings tells,
// counted on the active entrance (enter_map_id):
// sectors secured (OpReqPressingMap), victories (enemies beaten),
// meowfficer salvages ended, and the events of a kind started (their first
// effect run: resource nodes, merchants, switches, weather control
// devices, energy matrices, record files, data logging devices, promoted
// "Pawn" fleets, weather anomaly nodes, the scanning facility). The others
// (ambushes repelled, sources from enhanced fleets) aren't counted: the
// wiki only says both follow a promoted "Pawn" (one of its random
// aftermaths), and nothing in the tables marks those fleets or sources.
// Nothing done in an Obscured or Abyssal zone, a Stronghold or the
// Arbiter's (hazard 7) counts.

// worldTriggerKind is what a trigger counts.
type worldTriggerKind uint8

const (
	worldTriggerNone worldTriggerKind = iota
	worldTriggerPressing
	worldTriggerVictory
	worldTriggerSalvage
	worldTriggerEvent // worldTrigger.event started
)

// worldTriggerAnchors are what the achievements count. target_desc is all
// the tables say of a trigger, translated in every region, so it is matched
// in the region's own language against achievements whose kind is known
// (their ids are the same in every region): an achievement counts what an
// anchor target does when its target_desc reads the same but for the
// numbers. The events a worldTriggerEvent counts are the ones its is
// recognizes from their config where the tables mark the kind, else the
// ones named (in the region's language) like one of its anchor events: no
// field tells a resource node, switch, weather control device, energy
// matrix, data logging device, promoted "Pawn" or anomaly from the other
// events (their icons, compass types and effects are shared). Several
// anchors where a region words or names one kind several ways (JP's 11403,
// the switches of the experiment zones...). The comments are EN's texts.
var worldTriggerAnchors = []struct {
	kind    worldTriggerKind
	targets []uint32 // world_target_data
	events  []uint32 // world_event_data, of the kind (is must hold for them)
	is      func(event worldAchieveEvent) bool
	// once counts a device once: the later states it turns into (the
	// events its effects spawn, the same kind) aren't counted again. The
	// wiki (Operation Siren, Siren devices): "Each Scanning Device only
	// counts once towards exploration objectives", the Data Logging
	// Tower's two options "either counts".
	once bool
}{
	{worldTriggerPressing, []uint32{1101}, nil, nil, false},                         // Gain control of N sector(s).
	{worldTriggerVictory, []uint32{1303}, nil, nil, false},                          // Obtain N victorie(s).
	{worldTriggerSalvage, []uint32{1204}, nil, nil, false},                          // Conduct N Meowfficer retrieval operation(s).
	{worldTriggerEvent, []uint32{1403}, []uint32{1, 5104, 5901, 20252}, nil, false}, // Collect N resource node(s).
	// Encounter the Mysterious Merchant N time(s): Akashi's icon and the
	// shop's dialog (EffectEventStoryOptionClient), not her story missions
	{worldTriggerEvent, []uint32{1205}, []uint32{1901}, func(event worldAchieveEvent) bool {
		return event.Icon == "mingshi" && slices.Contains(event.effectTypes, worldEffectStoryOptionClient)
	}, false},
	{worldTriggerEvent, []uint32{2405}, []uint32{5401, 24120}, nil, false},          // Activate N switch mechanism(s).
	{worldTriggerEvent, []uint32{2305}, []uint32{5101, 21360, 1000202}, nil, false}, // Destroy N Siren weather control device(s).
	{worldTriggerEvent, []uint32{1105}, []uint32{5501, 21421, 1001105}, nil, false}, // Restore N Energy Matrices to their correct position.
	// Recover N record file(s): the compass' file mark
	{worldTriggerEvent, []uint32{1304}, []uint32{100000}, func(event worldAchieveEvent) bool {
		return event.CompassIndex == worldCompassFile
	}, false},
	{worldTriggerEvent, []uint32{1104}, []uint32{1101}, nil, true},             // Use the Siren Data Logging Tower N time(s).
	{worldTriggerEvent, []uint32{1103, 11403}, []uint32{10110017}, nil, false}, // Defeat N Promoted "Pawns" fleet(s).
	{worldTriggerEvent, []uint32{1102}, []uint32{9000}, nil, false},            // Perform Reconnaissance N time(s) in anomalous zones.
	// Use the Siren Scanning Facility N time(s): its model
	{worldTriggerEvent, []uint32{1405}, []uint32{1401}, func(event worldAchieveEvent) bool {
		return event.Icon == "laohuji"
	}, true},
}

const (
	// WorldMapAttachment.CompassTypeFile, the record files' compass_index
	worldCompassFile = 9
	// WorldMapAttachment.EffectEventStoryOptionClient, a dialog the client
	// runs (the merchant's shop)
	worldEffectStoryOptionClient = 1009
)

// worldAchieveEvent is what loadWorldAchieveData reads of a
// world_event_data row.
type worldAchieveEvent struct {
	Name         string   `json:"name"`
	Icon         string   `json:"icon"`
	CompassIndex uint32   `json:"compass_index"`
	Effect       []uint32 `json:"effect"`

	effectTypes []uint32 // its effects' world_effect_data effect_type
}

// worldTargetDescForm is a target_desc without its numbers, what the
// achievements of a kind share.
func worldTargetDescForm(desc string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return -1
		}
		return r
	}, desc)
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
	event int // worldTriggerEvent's: the worldTriggerAnchors entry
}

var (
	worldTargets         map[uint32]worldTargetConfig
	worldTargetEntrances map[uint32]*worldTargetEntrance
	worldTriggers        map[uint32]worldTrigger // by trigger id
	// worldEventTriggers are the worldTriggerAnchors entries counting the
	// events, by world_event_data id
	worldEventTriggers map[uint32]int
	// worldEntranceTargetIDs are each entrance's achievements
	worldEntranceTargetIDs map[uint32][]uint32
)

func loadWorldAchieveData() error {
	var entrances map[uint32]worldTargetEntrance
	// read here: gamedata.LoadAll runs loadWorldData (worldEvents,
	// worldEffects) at the same time
	var events map[uint32]worldAchieveEvent
	var effects map[uint32]worldEffect
	err := gamedata.LoadAll(
		gamedata.Load("world_target_data", &worldTargets),
		gamedata.Load("world_chapter_colormask", &entrances),
		gamedata.Load("world_event_data", &events),
		gamedata.Load("world_effect_data", &effects),
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
	forms := map[string]worldTrigger{}
	for i, anchor := range worldTriggerAnchors {
		for _, id := range anchor.targets {
			target, ok := worldTargets[id]
			if !ok {
				return gamedata.Missing("world_target_data", "achievement %d", id)
			}
			forms[worldTargetDescForm(target.TargetDesc)] = worldTrigger{anchor.kind, i}
		}
	}
	if err := readWorldEventTriggers(events, effects); err != nil {
		return err
	}
	worldTriggers = map[uint32]worldTrigger{}
	for _, target := range worldTargets {
		trigger := forms[worldTargetDescForm(target.TargetDesc)]
		for _, condition := range target.Condition {
			worldTriggers[condition[0]] = trigger
		}
	}
	return nil
}

// readWorldEventTriggers fills worldEventTriggers: each event's
// worldTriggerAnchors entry, by its is, else by its name.
func readWorldEventTriggers(events map[uint32]worldAchieveEvent, effects map[uint32]worldEffect) error {
	for id, event := range events {
		for _, effect := range event.Effect {
			event.effectTypes = append(event.effectTypes, effects[effect].EffectType)
		}
		events[id] = event
	}
	named := map[string]int{}
	for i, anchor := range worldTriggerAnchors {
		for _, id := range anchor.events {
			event, ok := events[id]
			switch {
			case !ok:
				return gamedata.Missing("world_event_data", "event %d", id)
			case anchor.is != nil && !anchor.is(event):
				return gamedata.Missing("world_event_data", "event %d of its achievement kind", id)
			case anchor.is != nil:
				continue
			}
			if other, seen := named[event.Name]; event.Name == "" || seen && other != i {
				return gamedata.Missing("world_event_data", "event %d named for one achievement kind", id)
			}
			named[event.Name] = i
		}
	}
	worldEventTriggers = map[uint32]int{}
	for id, event := range events {
		found := -1
		for i, anchor := range worldTriggerAnchors {
			if anchor.is != nil && anchor.is(event) {
				if found >= 0 {
					return gamedata.Missing("world_event_data", "event %d of one achievement kind", id)
				}
				found = i
			}
		}
		if i, ok := named[event.Name]; ok && found < 0 {
			found = i
		}
		if found >= 0 {
			worldEventTriggers[id] = found
		}
	}
	// a device counted once: the events its kind's events turn into
	later := map[uint32]bool{}
	for id, i := range worldEventTriggers {
		if !worldTriggerAnchors[i].once {
			continue
		}
		for _, spawned := range worldEventSpawns(events[id].Effect, effects) {
			if other, ok := worldEventTriggers[spawned]; ok && other == i && spawned != id {
				later[spawned] = true
			}
		}
	}
	for id := range later {
		delete(worldEventTriggers, id)
	}
	return nil
}

// worldEventSpawns are the events an event's effects can add to the map
// (EffectEventSpawn* types 9 and 10), its story options' (27), conditions'
// (35) and random picks' (36) effects followed.
func worldEventSpawns(start []uint32, effects map[uint32]worldEffect) []uint32 {
	var spawned []uint32
	seen := map[uint32]bool{}
	pending := slices.Clone(start)
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		effect, ok := effects[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		var params []json.RawMessage
		_ = json.Unmarshal(effect.Params, &params)
		switch effect.EffectType {
		case worldEffectSpawnEvents, worldEffectSpawnRelative:
			spawned = append(spawned, paramUints(params)...)
		case worldEffectStoryOption:
			var options [][]uint32
			if len(params) >= 2 && json.Unmarshal(params[1], &options) == nil {
				for _, option := range options {
					if len(option) == 2 {
						pending = append(pending, option[1])
					}
				}
			}
		case worldEffectCondition:
			for i := 1; i < len(params); i += 2 {
				pending = append(pending, rawUint(params[i]))
			}
			if len(params)%2 == 1 {
				pending = append(pending, rawUint(params[len(params)-1]))
			}
		case worldEffectRandom:
			for _, raw := range params {
				var option []float64
				if json.Unmarshal(raw, &option) == nil && len(option) == 2 {
					pending = append(pending, uint32(option[0]))
				}
			}
		}
	}
	return spawned
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
// (and worldTriggerAnchors entry for the events).
func worldTriggerCounts(happened *worldHappenings) (map[worldTriggerKind]uint32, map[int]uint32) {
	kinds := map[worldTriggerKind]uint32{
		worldTriggerPressing: uint32(len(happened.Pressed)),
		worldTriggerVictory:  uint32(len(happened.Kills)),
		worldTriggerSalvage:  uint32(len(happened.Salvaged)),
	}
	events := map[int]uint32{}
	var seen []uint32
	for _, id := range happened.Events {
		event, ok := worldEvents[id]
		if !ok || len(event.Effect) == 0 || slices.Contains(seen, id) || !slices.Contains(happened.Effects, event.Effect[0]) {
			continue
		}
		seen = append(seen, id)
		if anchor, counted := worldEventTriggers[id]; counted {
			events[anchor]++
		}
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
	// the wiki: only the Explorable and Secured zones count, not the
	// Obscured, Abyssal ones or the Strongholds
	if zone := worldZones[state.MapID]; zone == worldZoneObscured || zone == worldZoneAbyssal || worldRandomMaps[state.MapID].HazardLevel == 7 {
		return nil
	}
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
