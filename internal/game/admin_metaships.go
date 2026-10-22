package game

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// The back-office's view of a META ship's own progress (the META Lab's
// MetaCharacterScene): its Rigging Fortification repairs
// (MetaCharacterRepairLayer, stored as the ship_meta_repair rows done, as
// handleMetaRepair adds them) and its Tactical Research
// (MetaCharacterTacticsLayer, ship_meta_tactics and the research skills'
// ship_skills rows).

// metaRepairAttrs are MetaCharacterConst.REPAIR_ATTRS, in the order of
// metaShipConfig.repairs.
var metaRepairAttrs = [4]string{"cannon", "torpedo", "air", "reload"}

// metaLabTables are what the META Lab reads that the game handlers don't
// keep: ship_strengthen_meta's repair_total_exp and repair_effect
// (MetaCharacter), ship_meta_repair_effect's effect_attr (MetaRepairEffect),
// ship_meta_repair's effect_attr (MetaRepairItem.addition) and the gameset
// research limits (MetaTacticsInfo:isExpMaxPerDay, the double exp tag).
type metaLabTables struct {
	ships map[uint32]struct {
		RepairTotalExp uint32     `json:"repair_total_exp"`
		RepairEffect   [][]uint32 `json:"repair_effect"`
	}
	effects map[uint32]struct {
		EffectAttr [][]json.RawMessage `json:"effect_attr"`
	}
	steps map[uint32]struct {
		EffectAttr []json.RawMessage `json:"effect_attr"`
	}
	dayExpMax    uint32
	doubleExpMax uint32
}

var metaLab = sync.OnceValues(func() (*metaLabTables, error) {
	tables := &metaLabTables{}
	var err error
	if tables.ships, err = gamedata.Table[struct {
		RepairTotalExp uint32     `json:"repair_total_exp"`
		RepairEffect   [][]uint32 `json:"repair_effect"`
	}]("ship_strengthen_meta"); err != nil {
		return nil, err
	}
	if tables.effects, err = gamedata.Table[struct {
		EffectAttr [][]json.RawMessage `json:"effect_attr"`
	}]("ship_meta_repair_effect"); err != nil {
		return nil, err
	}
	if tables.steps, err = gamedata.Table[struct {
		EffectAttr []json.RawMessage `json:"effect_attr"`
	}]("ship_meta_repair"); err != nil {
		return nil, err
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return nil, err
	}
	tables.dayExpMax, tables.doubleExpMax = gameset["meta_skill_exp_max"], gameset["meta_skill_exp_double"]
	return tables, nil
})

// metaAttrValue reads an [attr, value] pair of effect_attr.
func metaAttrValue(pair []json.RawMessage) (string, float64, bool) {
	var attr string
	var value float64
	if len(pair) < 2 || json.Unmarshal(pair[0], &attr) != nil || json.Unmarshal(pair[1], &value) != nil {
		return "", 0, false
	}
	return attr, value, true
}

// MetaRepairAttr is one Rigging Fortification attribute (MetaCharacterAttr):
// the steps done of its ship_meta_repair list (its level - 1), the item each
// step takes, and the stat it adds so far and at the end
// (getAddition, getMaxAddition).
type MetaRepairAttr struct {
	Attr        string
	Done, Max   uint32
	ItemID      uint32
	ItemNum     uint32 // the next step's, 0 when done
	Addition    float64
	AdditionMax float64
}

// MetaRepairEffect is a ship_strengthen_meta repair_effect reward: its
// stats, given once the repair rate reaches progress percent
// (MetaCharacter:getPercentageAddition).
type MetaRepairEffect struct {
	ID       uint32
	Progress uint32
	Reached  bool
	Attrs    []MetaStat
}

type MetaStat struct {
	Attr  string
	Value float64
}

// MetaTacticsSkill is a research skill: level 0 while locked, its exp
// towards its ship_meta_skilltask need_exp (ExpMax, 0 at its max level).
type MetaTacticsSkill struct {
	ID       uint32
	Level    uint32
	MaxLevel uint32
	Exp      uint32
	ExpMax   uint32
	// NeedExp is the need_exp of each level from 1 below MaxLevel
	NeedExp []uint32
}

// MetaShipState is a META ship's repairs and research; Meta is false for
// any other ship.
type MetaShipState struct {
	Meta          bool
	RepairExp     uint32
	RepairExpMax  uint32
	Attrs         []MetaRepairAttr
	Effects       []MetaRepairEffect
	SkillID       uint32 // the skill studied, 0 for none
	DayExp        uint32
	DayExpMax     uint32
	DoubleExp     uint32
	DoubleExpMax  uint32
	Switches      uint32
	SwitchesMax   uint32
	Skills        []MetaTacticsSkill
	NextLevel     uint32 // Somatic Activation's next breakout: its level and repair exp, 0 at the last
	NextRepairExp uint32
}

// MetaShip is the META state of one of the commander's ships.
func (game *Game) MetaShip(ctx context.Context, commanderID, shipID uint32) (MetaShipState, error) {
	tx := newTx(ctx, game.DB.Queries, commanderID)
	ship, err := adminShip(ctx, tx.Queries, commanderID, shipID)
	if err != nil {
		return MetaShipState{}, err
	}
	return tx.metaShipState(ship)
}

func (tx *Tx) metaShipState(ship db.Ship) (MetaShipState, error) {
	breakout, meta := metaBreakouts[ship.TemplateID]
	config, known := metaShips[ship.TemplateID/10]
	if !meta || !known {
		return MetaShipState{}, nil
	}
	lab, err := metaLab()
	if err != nil {
		return MetaShipState{}, err
	}
	done, err := tx.ListMetaRepairs(tx.ctx, ship.ID)
	if err != nil {
		return MetaShipState{}, err
	}
	levels, exp := metaRepairLevels(config, done)
	state := MetaShipState{
		Meta: true, RepairExp: exp, RepairExpMax: lab.ships[ship.TemplateID/10].RepairTotalExp,
		Attrs: []MetaRepairAttr{}, Effects: []MetaRepairEffect{}, Skills: []MetaTacticsSkill{},
		DayExpMax: lab.dayExpMax, DoubleExpMax: lab.doubleExpMax, SwitchesMax: metaSwitchesPerDay,
	}
	if breakout.BreakoutID != 0 {
		state.NextLevel, state.NextRepairExp = breakout.Level, breakout.Repair
	}
	for i, list := range config.repairs() {
		// an attribute without rows is locked (MetaCharacterAttr:isLock)
		if len(list) == 0 {
			continue
		}
		attr := MetaRepairAttr{Attr: metaRepairAttrs[i], Done: levels[i] - 1, Max: uint32(len(list)), ItemID: metaRepairs[list[0]].ItemID}
		if attr.Done < attr.Max {
			next := metaRepairs[list[attr.Done]]
			attr.ItemID, attr.ItemNum = next.ItemID, next.ItemNum
		}
		for j, id := range list {
			_, value, _ := metaAttrValue(lab.steps[id].EffectAttr)
			attr.AdditionMax += value
			if uint32(j) < attr.Done {
				attr.Addition += value
			}
		}
		state.Attrs = append(state.Attrs, attr)
	}
	for _, effect := range lab.ships[ship.TemplateID/10].RepairEffect {
		if len(effect) < 2 {
			continue
		}
		// getRepairRate * 100 >= progress
		row := MetaRepairEffect{ID: effect[1], Progress: effect[0], Reached: state.RepairExpMax > 0 && exp*100 >= effect[0]*state.RepairExpMax, Attrs: []MetaStat{}}
		for _, pair := range lab.effects[effect[1]].EffectAttr {
			if attr, value, ok := metaAttrValue(pair); ok && value != 0 {
				row.Attrs = append(row.Attrs, MetaStat{Attr: attr, Value: value})
			}
		}
		state.Effects = append(state.Effects, row)
	}

	t, err := tx.loadOneMetaTactics(ship.ID, metaDay(time.Now()))
	if err != nil {
		if refused(err) {
			return state, nil
		}
		return MetaShipState{}, err
	}
	state.SkillID, state.DayExp, state.DoubleExp, state.Switches = t.state.SkillID, t.state.DayExp, t.state.DoubleExp, t.state.Switches
	for _, id := range metaTacticsSkills[ship.TemplateID] {
		skill := MetaTacticsSkill{ID: id, Level: t.skillLevel(id), MaxLevel: metaSkillMaxLevel(id)}
		for _, row := range t.skills {
			if row.SkillID == id {
				skill.Exp = row.Exp
			}
		}
		skill.ExpMax = metaSkillExpMax(id, skill.Level)
		for level := uint32(1); level < skill.MaxLevel; level++ {
			skill.NeedExp = append(skill.NeedExp, metaSkillExpMax(id, level))
		}
		state.Skills = append(state.Skills, skill)
	}
	return state, nil
}

// metaSkillExpMax is the most exp a research skill holds at level: its
// ship_meta_skilltask need_exp, where the layer offers its level up
// (MetaCharacterTacticsLayer:updateTacticsRedTag), 0 while locked or at its
// max level (the layer shows "exp/Max").
func metaSkillExpMax(skillID, level uint32) uint32 {
	if level == 0 || level >= metaSkillMaxLevel(skillID) {
		return 0
	}
	return metaSkillTasksByLevel[skillID][level].NeedExp
}

// MetaShipEdit is a change of a META ship's progress; nil fields stay.
type MetaShipEdit struct {
	// Repairs sets the steps done of each attribute, by REPAIR_ATTRS name
	Repairs map[string]uint32
	// RepairFull does every step of every attribute
	RepairFull bool
	// SkillID changes the skill studied (0 for none)
	SkillID *uint32
	// Skills sets research skills' level (0 locks it) and exp
	Skills map[uint32]MetaSkillEdit
	// ResetDay clears today's research exp, double exp and switches
	ResetDay bool
}

type MetaSkillEdit struct {
	Level uint32
	Exp   uint32
}

// EditMetaShip changes a META ship's repairs and research, the commander
// kicked first.
func (game *Game) EditMetaShip(ctx context.Context, commanderID, shipID uint32, edit MetaShipEdit) (bool, error) {
	if _, err := adminShip(ctx, game.DB.Queries, commanderID, shipID); err != nil {
		return false, err
	}
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		ship, err := adminShip(ctx, tx.Queries, commanderID, shipID)
		if err != nil {
			return err
		}
		config, known := metaShips[ship.TemplateID/10]
		if _, meta := metaBreakouts[ship.TemplateID]; !meta || !known {
			return adminRefused("%d isn't a META ship", shipID)
		}
		if err := tx.editMetaRepairs(ship, config, edit); err != nil {
			return err
		}
		return tx.editMetaTactics(ship, edit)
	})
}

// editMetaRepairs keeps the rows the game would have added for those steps:
// each attribute's first ones, in order (MetaCharacterRepairCommand only
// takes an attribute's next row).
func (tx *Tx) editMetaRepairs(ship db.Ship, config metaShipConfig, edit MetaShipEdit) error {
	if len(edit.Repairs) == 0 && !edit.RepairFull {
		return nil
	}
	done, err := tx.ListMetaRepairs(tx.ctx, ship.ID)
	if err != nil {
		return err
	}
	levels, _ := metaRepairLevels(config, done)
	lists := config.repairs()
	steps := [4]uint32{}
	for i := range lists {
		steps[i] = levels[i] - 1
		if edit.RepairFull {
			steps[i] = uint32(len(lists[i]))
		}
	}
	for attr, n := range edit.Repairs {
		i := -1
		for j, name := range metaRepairAttrs {
			if name == attr {
				i = j
			}
		}
		if i < 0 || len(lists[i]) == 0 {
			return adminRefused("the ship has no %q repairs", attr)
		}
		if n > uint32(len(lists[i])) {
			return adminRefused("%s repairs go from 0 to %d", attr, len(lists[i]))
		}
		if !edit.RepairFull {
			steps[i] = n
		}
	}
	if err := tx.AdminClearMetaRepairs(tx.ctx, ship.ID); err != nil {
		return err
	}
	for i, list := range lists {
		for _, id := range list[:steps[i]] {
			if err := tx.AddMetaRepair(tx.ctx, db.AddMetaRepairParams{ShipID: ship.ID, RepairID: id}); err != nil {
				return err
			}
		}
	}
	return nil
}

// editMetaTactics sets research skills, the skill studied and today's
// counts. As MetaCharacterTacticsLayer:tryLearnSkillAfterFirstUnlock does,
// the first skill unlocked becomes the studied one when none is.
func (tx *Tx) editMetaTactics(ship db.Ship, edit MetaShipEdit) error {
	if len(edit.Skills) == 0 && edit.SkillID == nil && !edit.ResetDay {
		return nil
	}
	t, err := tx.loadOneMetaTactics(ship.ID, metaDay(time.Now()))
	if refused(err) {
		return adminRefused("%d isn't a META ship", ship.ID)
	} else if err != nil {
		return err
	}
	for id := range edit.Skills {
		if !t.isResearchSkill(id) {
			return adminRefused("skill %d isn't one of the ship's research skills", id)
		}
	}
	unlocked := uint32(0)
	for _, id := range metaTacticsSkills[ship.TemplateID] {
		change, ok := edit.Skills[id]
		if !ok {
			continue
		}
		top := metaSkillMaxLevel(id)
		if change.Level > top {
			return adminRefused("skill %d goes from level 0 to %d", id, top)
		}
		if most := metaSkillExpMax(id, change.Level); change.Exp > most {
			return adminRefused("skill %d holds up to %d exp at level %d", id, most, change.Level)
		}
		before := t.skillLevel(id)
		if change.Level == 0 {
			if before > 0 {
				if err := tx.DeleteShipSkill(tx.ctx, db.DeleteShipSkillParams{ShipID: ship.ID, SkillID: id}); err != nil {
					return err
				}
			}
			if t.state.SkillID == id {
				t.state.SkillID = 0
			}
			continue
		}
		if before == 0 {
			if err := tx.AddShipSkill(tx.ctx, db.AddShipSkillParams{ShipID: ship.ID, SkillID: id, Level: change.Level}); err != nil {
				return err
			}
			if unlocked == 0 {
				unlocked = id
			}
		}
		if err := tx.SetShipSkillExp(tx.ctx, db.SetShipSkillExpParams{Level: change.Level, Exp: change.Exp, ShipID: ship.ID, SkillID: id}); err != nil {
			return err
		}
	}
	level := func(id uint32) uint32 {
		if change, ok := edit.Skills[id]; ok {
			return change.Level
		}
		return t.skillLevel(id)
	}
	if edit.SkillID != nil {
		id := *edit.SkillID
		if id != 0 && (!t.isResearchSkill(id) || level(id) == 0) {
			return adminRefused("skill %d isn't an unlocked research skill", id)
		}
		t.state.SkillID = id
	} else if t.state.SkillID == 0 && unlocked != 0 {
		t.state.SkillID = unlocked
	}
	if edit.ResetDay {
		t.state.DayExp, t.state.DoubleExp, t.state.Switches = 0, 0, 0
	}
	return tx.saveMetaTactics(t.state)
}
