package admin

import (
	"net/http"

	"github.com/ggmolly/belfast/internal/game"
)

// registerMetaShips adds the back-office's META ships repair and tactics routes.
func (server *Server) registerMetaShips(api apiFunc) {
	api("GET /api/commanders/{id}/dock/{ship}/meta", server.handleMetaShip)
	api("POST /api/commanders/{id}/dock/{ship}/meta", server.handleEditMetaShip)
}

type metaRepairAttrJSON struct {
	Attr        string  `json:"attr"`
	Done        uint32  `json:"done"`
	Max         uint32  `json:"max"`
	Item        named   `json:"item"`
	ItemID      uint32  `json:"item_id"`
	ItemNum     uint32  `json:"item_num"`
	Addition    float64 `json:"addition"`
	AdditionMax float64 `json:"addition_max"`
}

type metaStatJSON struct {
	Attr  string  `json:"attr"`
	Value float64 `json:"value"`
}

type metaEffectJSON struct {
	ID       uint32         `json:"id"`
	Progress uint32         `json:"progress"`
	Reached  bool           `json:"reached"`
	Attrs    []metaStatJSON `json:"attrs"`
}

type metaSkillJSON struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Level  uint32 `json:"level"`
	Max    uint32 `json:"max"`
	Exp    uint32 `json:"exp"`
	ExpMax uint32 `json:"exp_max"`
	// the exp each level holds, from level 1 below max
	NeedExp []uint32 `json:"need_exp"`
}

type metaRepairJSON struct {
	Exp     uint32               `json:"exp"`
	ExpMax  uint32               `json:"exp_max"`
	Attrs   []metaRepairAttrJSON `json:"attrs"`
	Effects []metaEffectJSON     `json:"effects"`
}

// metaActivationJSON is Somatic Activation's next breakout: the level and
// repair exp it takes.
type metaActivationJSON struct {
	Level     uint32 `json:"level"`
	RepairExp uint32 `json:"repair_exp"`
}

type metaTacticsJSON struct {
	SkillID      uint32          `json:"skill_id"`
	DayExp       uint32          `json:"day_exp"`
	DayExpMax    uint32          `json:"day_exp_max"`
	DoubleExp    uint32          `json:"double_exp"`
	DoubleExpMax uint32          `json:"double_exp_max"`
	Switches     uint32          `json:"switches"`
	SwitchesMax  uint32          `json:"switches_max"`
	Skills       []metaSkillJSON `json:"skills"`
}

// metaShipJSON is a META ship's repairs and research; {"meta": false} for
// any other ship.
type metaShipJSON struct {
	Meta       bool                `json:"meta"`
	Repair     *metaRepairJSON     `json:"repair,omitempty"`
	Activation *metaActivationJSON `json:"activation,omitempty"`
	Tactics    *metaTacticsJSON    `json:"tactics,omitempty"`
}

func (server *Server) handleMetaShip(r *http.Request) (any, error) {
	commanderID, shipID, err := server.shipPath(r)
	if err != nil {
		return nil, err
	}
	return server.metaShip(r, commanderID, shipID)
}

func (server *Server) metaShip(r *http.Request, commanderID, shipID uint32) (any, error) {
	tables, err := dockNames()
	if err != nil {
		return nil, err
	}
	state, err := server.Game.MetaShip(r.Context(), commanderID, shipID)
	if err != nil {
		return nil, err
	}
	out := metaShipJSON{Meta: state.Meta}
	if !state.Meta {
		return out, nil
	}
	out.Repair = &metaRepairJSON{Exp: state.RepairExp, ExpMax: state.RepairExpMax, Attrs: []metaRepairAttrJSON{}, Effects: []metaEffectJSON{}}
	for _, attr := range state.Attrs {
		out.Repair.Attrs = append(out.Repair.Attrs, metaRepairAttrJSON{
			Attr: attr.Attr, Done: attr.Done, Max: attr.Max, Item: server.names[dropItem][attr.ItemID], ItemID: attr.ItemID, ItemNum: attr.ItemNum,
			Addition: attr.Addition, AdditionMax: attr.AdditionMax,
		})
	}
	for _, effect := range state.Effects {
		row := metaEffectJSON{ID: effect.ID, Progress: effect.Progress, Reached: effect.Reached, Attrs: []metaStatJSON{}}
		for _, stat := range effect.Attrs {
			row.Attrs = append(row.Attrs, metaStatJSON(stat))
		}
		out.Repair.Effects = append(out.Repair.Effects, row)
	}
	if state.NextLevel != 0 {
		out.Activation = &metaActivationJSON{state.NextLevel, state.NextRepairExp}
	}
	out.Tactics = &metaTacticsJSON{
		SkillID: state.SkillID, DayExp: state.DayExp, DayExpMax: state.DayExpMax, DoubleExp: state.DoubleExp, DoubleExpMax: state.DoubleExpMax,
		Switches: state.Switches, SwitchesMax: state.SwitchesMax, Skills: []metaSkillJSON{},
	}
	for _, skill := range state.Skills {
		out.Tactics.Skills = append(out.Tactics.Skills, metaSkillJSON{ID: skill.ID, Name: tables.skills[skill.ID], Level: skill.Level, Max: skill.MaxLevel, Exp: skill.Exp, ExpMax: skill.ExpMax, NeedExp: append([]uint32{}, skill.NeedExp...)})
	}
	return out, nil
}

// handleEditMetaShip changes a META ship's repairs (steps done by attribute,
// or every one), research skills (level, 0 to lock, and exp), the skill it
// studies and today's research counts; the ship's META state as it is now.
func (server *Server) handleEditMetaShip(r *http.Request) (any, error) {
	commanderID, shipID, err := server.shipPath(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Repairs    map[string]uint32 `json:"repairs"`
		RepairFull bool              `json:"repair_full"`
		SkillID    *uint32           `json:"skill_id"`
		Skills     map[uint32]struct {
			Level uint32 `json:"level"`
			Exp   uint32 `json:"exp"`
		} `json:"skills"`
		ResetDay bool `json:"reset_day"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	edit := game.MetaShipEdit{Repairs: body.Repairs, RepairFull: body.RepairFull, SkillID: body.SkillID, ResetDay: body.ResetDay, Skills: map[uint32]game.MetaSkillEdit{}}
	for id, skill := range body.Skills {
		edit.Skills[id] = game.MetaSkillEdit(skill)
	}
	if _, err := server.Game.EditMetaShip(r.Context(), commanderID, shipID, edit); err != nil {
		return nil, err
	}
	return server.metaShip(r, commanderID, shipID)
}
