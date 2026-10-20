package game

import (
	"context"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Tactical Research (META Lab -> a META ship -> its skills,
// view/technology/metacharacter/metacharactertacticslayer.lua) is how META
// ships (Ship:isMetaShipByConfigID, a ship_meta_breakout id) learn skills.
// Their research skills (MetaCharacterConst.getTacticsSkillIDListByShipConfigID)
// are the ship_data_template buff_list_display skills that have
// ship_meta_skilltask rows; they come locked (Ship:getMetaSkillLevelBySkillID
// is 0 for a skill missing from SHIPINFO.skill_id_list, and buff_list leaves
// them out) and are unlocked one by one with the skill_unlock materials of
// their level 1 row. One skill at a time is studied (MetaTacticsInfo.curSkillID,
// 0 for none: isAnyLearning is curSkillID > 0), its exp coming from the
// skill_levelup_task missions up to gameset meta_skill_exp_max a day.
//
// No official capture has any of it: the replies are built from the Lua
// alone. Belfast doesn't count the research missions yet, so the exp only
// ever stays at 0.

// metaSwitchesPerDay is how many times a day a ship may change the skill it
// studies: the client sets switchCountLeft to 1 when the first unlocked skill
// becomes the studied one (MetaCharacterTacticsLayer:tryLearnSkillAfterFirstUnlock)
// and, at 0, tells meta_switch_skill_disable "Unable to switch to learning a
// new skill today."
const metaSwitchesPerDay = 1

// metaSkillTask is a ship_meta_skilltask row: what a skill's level takes.
type metaSkillTask struct {
	SkillID uint32 `json:"skill_ID"`
	Level   uint32 `json:"level"`
	NeedExp uint32 `json:"need_exp"`
	// [drop type, id, count] per choice, picked by CS_63311.index (1-based)
	SkillUnlock [][]uint32 `json:"skill_unlock"`
}

// metaSkillTasksByLevel is ship_meta_skilltask by skill then level.
var metaSkillTasksByLevel map[uint32]map[uint32]metaSkillTask

var (
	// metaTacticsSkills are the research skills of each META ship template
	// (every ship_meta_breakout id), in buff_list_display order
	metaTacticsSkills map[uint32][]uint32
	// metaSkillUnlocks are the level 1 skill_unlock materials of each
	// research skill
	metaSkillUnlocks map[uint32][][]uint32
)

func loadMetaTacticsData() error {
	var templates map[uint32]struct {
		BuffListDisplay []uint32 `json:"buff_list_display"`
	}
	var tasks map[uint32]metaSkillTask
	err := gamedata.LoadAll(
		gamedata.Load("ship_meta_breakout", &metaBreakouts),
		gamedata.Load("ship_meta_repair", &metaRepairs),
		gamedata.Load("ship_data_template", &templates),
		gamedata.Load("ship_meta_skilltask", &tasks),
	)
	if err != nil {
		return err
	}
	metaSkillUnlocks = map[uint32][][]uint32{}
	metaSkillTasksByLevel = map[uint32]map[uint32]metaSkillTask{}
	for _, task := range tasks {
		if metaSkillTasksByLevel[task.SkillID] == nil {
			metaSkillTasksByLevel[task.SkillID] = map[uint32]metaSkillTask{}
		}
		metaSkillTasksByLevel[task.SkillID][task.Level] = task
		if _, ok := metaSkillUnlocks[task.SkillID]; !ok {
			metaSkillUnlocks[task.SkillID] = nil
		}
		if task.Level == 1 {
			metaSkillUnlocks[task.SkillID] = task.SkillUnlock
		}
	}
	metaTacticsSkills = make(map[uint32][]uint32, len(metaBreakouts))
	for id := range metaBreakouts {
		skills := []uint32{}
		for _, skill := range templates[id].BuffListDisplay {
			if _, ok := metaSkillUnlocks[skill]; ok {
				skills = append(skills, skill)
			}
		}
		metaTacticsSkills[id] = skills
	}
	return nil
}

// metaTactics is a META ship's research state on a given day.
type metaTactics struct {
	ship   db.Ship
	state  db.ShipMetaTactic
	skills []db.ShipSkill // its learnt skills, research or not
}

// loadMetaTactics returns the research state of those of ids that are the
// commander's META ships, today's (day) counters reset when the stored ones
// are from an earlier day.
func (tx *Tx) loadMetaTactics(ids []uint32, day int64) ([]metaTactics, error) {
	ships, err := tx.ListShipsByID(tx.ctx, db.ListShipsByIDParams{CommanderID: tx.commanderID, Ids: ids})
	if err != nil {
		return nil, err
	}
	states, err := tx.ListMetaTactics(tx.ctx, db.ListMetaTacticsParams{CommanderID: tx.commanderID, Ids: ids})
	if err != nil {
		return nil, err
	}
	skills, err := tx.ListShipSkillsByID(tx.ctx, db.ListShipSkillsByIDParams{CommanderID: tx.commanderID, Ids: ids})
	if err != nil {
		return nil, err
	}
	stateByShip := map[uint32]db.ShipMetaTactic{}
	for _, state := range states {
		if state.Day != day {
			state.Day, state.DayExp, state.DoubleExp, state.Switches = day, 0, 0, 0
		}
		stateByShip[state.ShipID] = state
	}
	skillsByShip := map[uint32][]db.ShipSkill{}
	for _, skill := range skills {
		skillsByShip[skill.ShipID] = append(skillsByShip[skill.ShipID], skill)
	}
	var out []metaTactics
	for _, ship := range ships {
		if _, meta := metaTacticsSkills[ship.TemplateID]; !meta {
			continue
		}
		state, ok := stateByShip[ship.ID]
		if !ok {
			state = db.ShipMetaTactic{ShipID: ship.ID, Day: day}
		}
		out = append(out, metaTactics{ship: ship, state: state, skills: skillsByShip[ship.ID]})
	}
	return out, nil
}

// loadOneMetaTactics is loadMetaTactics for one ship, errRefused when it
// isn't one of the commander's META ships.
func (tx *Tx) loadOneMetaTactics(shipID uint32, day int64) (metaTactics, error) {
	tactics, err := tx.loadMetaTactics([]uint32{shipID}, day)
	if err != nil {
		return metaTactics{}, err
	}
	if len(tactics) == 0 {
		return metaTactics{}, errRefused
	}
	return tactics[0], nil
}

func (tx *Tx) saveMetaTactics(state db.ShipMetaTactic) error {
	return tx.SetMetaTactics(tx.ctx, db.SetMetaTacticsParams(state))
}

// skillLevel is the level of one of the ship's skills, 0 when locked.
func (t metaTactics) skillLevel(skillID uint32) uint32 {
	for _, skill := range t.skills {
		if skill.SkillID == skillID {
			return skill.Level
		}
	}
	return 0
}

// isResearchSkill is whether skillID is one of the ship's research skills.
func (t metaTactics) isResearchSkill(skillID uint32) bool {
	return slices.Contains(metaTacticsSkills[t.ship.TemplateID], skillID)
}

// skillExp is the exp of each unlocked research skill (MetaTacticsInfo
// skillExpInfoTable, the SHIPSKILL skill_exp that SC_63316 also updates).
func (t metaTactics) skillExp() []*protobuf.SKILL_EXP {
	exps := []*protobuf.SKILL_EXP{}
	for _, skill := range t.skills {
		if skill.Level > 0 && t.isResearchSkill(skill.SkillID) {
			exps = append(exps, &protobuf.SKILL_EXP{SkillId: proto.Uint32(skill.SkillID), Exp: proto.Uint32(skill.Exp)})
		}
	}
	return exps
}

func metaDay(now time.Time) int64 {
	return startOfDay(now).Unix()
}

// handleMetaTacticsInfo answers MetaCharacterTacticsInfoRequestCommand
// (controller/command/technology/metacharacter/metacharactertacticsinforequestcommand.lua),
// sent at login for every META ship (LoadPlayerDataCommand ->
// MetaCharacterProxy:requestMetaTacticsInfo) and for each new one
// (BayProxy:addShip): MetaCharacterProxy:setMetaTacticsInfo makes a
// MetaTacticsInfo of each entry, curDayExp from exp, curSkillID from
// skill_id, skillExpInfoTable from skill_exp. Ships left out fall back to an
// empty MetaTacticsInfo (getMetaTacticsInfoByShipID), so ids that aren't the
// commander's META ships are skipped.
func (game *Game) handleMetaTacticsInfo(client *connection.Client, req *protobuf.CS_63317) error {
	day := metaDay(time.Now())
	reply := &protobuf.SC_63318{InfoList: []*protobuf.META_SKILL_SIMPLE_INFO{}}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		tactics, err := tx.loadMetaTactics(req.GetShipIdList(), day)
		for _, t := range tactics {
			reply.InfoList = append(reply.InfoList, &protobuf.META_SKILL_SIMPLE_INFO{
				ShipId:   proto.Uint32(t.ship.ID),
				Exp:      proto.Uint32(t.state.DayExp),
				SkillId:  proto.Uint32(t.state.SkillID),
				SkillExp: t.skillExp(),
			})
		}
		return err
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleMetaTactics answers MetaCharacterTacticsRequestCommand
// (metacharactertacticsrequestcommand.lua), sent when the research page of a
// META ship opens (MetaCharacterTacticsMediator:requestTacticsData): exp is
// today's research exp ("exp / meta_skill_exp_max"), double_exp the part of
// it earned at double rate (the double tag shows while it is below
// meta_skill_exp_double), switch_cnt the switches left today, tasks the
// finish counts of the studied skill's missions (none counted yet). The
// reply has no result: a ship that isn't the commander's META ship gets an
// empty state.
func (game *Game) handleMetaTactics(client *connection.Client, req *protobuf.CS_63313) error {
	day := metaDay(time.Now())
	reply := &protobuf.SC_63314{ShipId: proto.Uint32(req.GetShipId())}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		t, err := tx.loadOneMetaTactics(req.GetShipId(), day)
		if err != nil {
			return err
		}
		reply.Exp = proto.Uint32(t.state.DayExp)
		reply.DoubleExp = proto.Uint32(t.state.DoubleExp)
		reply.SkillId = proto.Uint32(t.state.SkillID)
		reply.SwitchCnt = proto.Uint32(metaSwitchesPerDay - min(t.state.Switches, metaSwitchesPerDay))
		reply.SkillExp = t.skillExp()
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(reply)
}

// handleMetaTacticsUnlock answers MetaCharacterTacticsUnlockCommand
// (metacharactertacticsunlockcommand.lua): unlocks a locked research skill
// at level 1 for the skill_unlock materials at index (1-based, the layer's
// pick among its level 1 row's). On result 0 the client raises the skill to
// level 1 (Ship:upSkillLevelForMeta) and takes the materials off itself.
// The first skill unlocked becomes the studied one without using a switch
// (MetaCharacterTacticsLayer:tryLearnSkillAfterFirstUnlock).
func (game *Game) handleMetaTacticsUnlock(client *connection.Client, req *protobuf.CS_63311) error {
	day := metaDay(time.Now())
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		t, err := tx.loadOneMetaTactics(req.GetShipId(), day)
		if err != nil {
			return err
		}
		skill := req.GetSkillId()
		if !t.isResearchSkill(skill) || t.skillLevel(skill) > 0 {
			return errRefused
		}
		unlocks := metaSkillUnlocks[skill]
		index := int(req.GetIndex())
		if index < 1 || index > len(unlocks) || len(unlocks[index-1]) < 3 || unlocks[index-1][0] != dropTypeItem {
			return errRefused
		}
		if err := tx.SpendItem(unlocks[index-1][1], unlocks[index-1][2]); err != nil {
			return err
		}
		if err := tx.AddShipSkill(tx.ctx, db.AddShipSkillParams{ShipID: t.ship.ID, SkillID: skill, Level: 1}); err != nil {
			return err
		}
		if t.state.SkillID == 0 {
			t.state.SkillID = skill
		}
		return tx.saveMetaTactics(t.state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63312{Result: result(err)})
}

// handleMetaTacticsSwitch answers MetaCharacterTacticsSwitchCommand
// (metacharactertacticsswitchcommand.lua): studies another unlocked research
// skill (the layer offers no switch to a skill at its max level), using one
// of the day's switches; switch_cnt is how many are left.
func (game *Game) handleMetaTacticsSwitch(client *connection.Client, req *protobuf.CS_63307) error {
	day := metaDay(time.Now())
	var left uint32
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		t, err := tx.loadOneMetaTactics(req.GetShipId(), day)
		if err != nil {
			return err
		}
		skill := req.GetSkillId()
		if !t.isResearchSkill(skill) || t.skillLevel(skill) == 0 || skill == t.state.SkillID ||
			t.state.Switches >= metaSwitchesPerDay {
			return errRefused
		}
		t.state.SkillID = skill
		t.state.Switches++
		left = metaSwitchesPerDay - t.state.Switches
		return tx.saveMetaTactics(t.state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63308{Result: result(err), SwitchCnt: proto.Uint32(left)})
}
