package game

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Island Technologies (IslandTechnologyAgency, IslandTechnology): each
// technology (island_technology_template) is researched as a delegation of
// its formula on place 702 once unlocked, unlocking being the formula's
// ability (island_formula.unlock_type, a TYPE_FORMULA ability). What a
// finished technology grants is the official server's alone: belfast gives
// what <data dir>/<region>/server/island_tech_awards.json lists for it
// ({"<tech id>": [[type, id, count], ...]}), nothing without the file.

// IslandTechnology.UNLOCK_TYPE: a sys_unlock condition
const (
	techUnlockLevel   = 0
	techUnlockTask    = 1
	techUnlockAbility = 2
	techUnlockTech    = 3
)

// productionTechConfig is an island_technology_template row.
type productionTechConfig struct {
	FormulaID   uint32          `json:"formula_id"`
	IslandLevel uint32          `json:"island_level"`
	SysUnlock   json.RawMessage `json:"sys_unlock"`
	TechRepeat  []uint32        `json:"tech_repeat"`
	AutoFinish  uint32          `json:"auto_finish"`
}

var (
	productionTechs map[uint32]productionTechConfig
	// a technology by its formula (IslandTechnologyAgency.formula2Id)
	productionTechByFormula map[uint32]uint32
	// what a finished technology grants (server content)
	productionTechAwards map[uint32][][3]uint32
)

func loadProductionTechData() error {
	if err := gamedata.Load("island_technology_template", &productionTechs)(); err != nil {
		return err
	}
	productionTechByFormula = map[uint32]uint32{}
	for id, tech := range productionTechs {
		productionTechByFormula[tech.FormulaID] = id
	}
	productionTechAwards = map[uint32][][3]uint32{}
	raw, err := gamedata.ServerFile("island_tech_awards.json")
	if err != nil || raw == nil {
		return err
	}
	if err := json.Unmarshal(raw, &productionTechAwards); err != nil {
		return fmt.Errorf("server content island_tech_awards.json: %w", err)
	}
	return nil
}

// once tells whether a technology is researched once (tech_repeat[1] 0).
func (tech productionTechConfig) once() bool {
	return len(tech.TechRepeat) == 0 || tech.TechRepeat[0] == 0
}

// techFinished is how many times a technology was researched
// (PB_ISLAND_TECH).
func (state *islandState) techFinished(id uint32) uint32 {
	tech := state.public().GetTech()
	if slices.Contains(tech.GetFinishList(), id) {
		return 1
	}
	for _, entry := range tech.GetRepeatFinishList() {
		if entry.GetId() == id {
			return entry.GetNum()
		}
	}
	return 0
}

// finishTech counts a technology researched once more
// (IslandTechnologyAgency.AddFinishCntByFormulatId) and is what it grants.
func (tx *Tx) finishTech(state *islandState, id uint32) ([]*protobuf.DROPINFO, error) {
	config, ok := productionTechs[id]
	if !ok {
		return nil, nil
	}
	public := state.public()
	if public.Tech == nil {
		public.Tech = &protobuf.PB_ISLAND_TECH{}
	}
	tech := public.Tech
	if config.once() {
		if slices.Contains(tech.FinishList, id) {
			return nil, nil
		}
		tech.FinishList = append(tech.FinishList, id)
	} else {
		found := false
		for _, entry := range tech.RepeatFinishList {
			if entry.GetId() == id {
				entry.Num = proto.Uint32(entry.GetNum() + 1)
				found = true
			}
		}
		if !found {
			tech.RepeatFinishList = append(tech.RepeatFinishList, &protobuf.PB_REPEAT_FINISH{Id: proto.Uint32(id), Num: proto.Uint32(1)})
		}
	}
	var drops []*protobuf.DROPINFO
	for _, award := range productionTechAwards[id] {
		drops = append(drops, &protobuf.DROPINFO{Type: proto.Uint32(award[0]), Id: proto.Uint32(award[1]), Number: proto.Uint32(award[2])})
	}
	return drops, nil
}

// techUnlockable tells whether a technology's conditions are met
// (IslandTechnology.CanUnlock): the island's level and every sys_unlock.
func (state *islandState) techUnlockable(config productionTechConfig) bool {
	level := state.public().GetLevel()
	if level < config.IslandLevel {
		return false
	}
	var conditions [][]uint32
	_ = json.Unmarshal(config.SysUnlock, &conditions)
	for _, condition := range conditions {
		if len(condition) < 2 {
			return false
		}
		value := condition[1]
		switch condition[0] {
		case techUnlockLevel:
			if level < config.IslandLevel {
				return false
			}
		case techUnlockTask:
			if !slices.Contains(state.public().GetTaskInfo().GetTaskIdListFinish(), value) {
				return false
			}
		case techUnlockAbility:
			if !state.productionHasAbility(value) {
				return false
			}
		case techUnlockTech:
			if state.techFinished(value) == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// handleIslandUnlockTech answers IslandUnlockTechCommand (CS_21520, sent by
// IslandTechnologyAgency.TryAutoUnlock as soon as a technology's conditions
// are met): the island gains the technology's formula ability, which the
// client adds itself. Refused (ret 1) when it has it already or the
// conditions aren't met.
func (game *Game) handleIslandUnlockTech(client *connection.Client, req *protobuf.CS_21520) error {
	reply := &protobuf.SC_21521{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		config, ok := productionTechs[req.GetTechId()]
		ability := productionFormulas[config.FormulaID].UnlockType
		if !ok || ability == 0 || state.productionHasAbility(ability) || !state.techUnlockable(config) {
			reply.Result = result(errRefused)
			return nil
		}
		reply.Result = result(nil)
		return tx.grantDrops([]*protobuf.DROPINFO{{Type: proto.Uint32(dropTypeIslandAbility), Id: proto.Uint32(ability), Number: proto.Uint32(1)}})
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandFinishTech answers IslandFinishTechImmdCommand (CS_21522): a
// technology finished without research (auto_finish 0, sent for each by
// GetAutoFinishList once it is unlocked), counted as researched with what
// it grants in drop_list. None of the current technologies is one.
func (game *Game) handleIslandFinishTech(client *connection.Client, req *protobuf.CS_21522) error {
	reply := &protobuf.SC_21523{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		id := req.GetTechId()
		config, ok := productionTechs[id]
		if !ok || config.AutoFinish != 0 || !state.formulaUnlocked(config.FormulaID) || state.techFinished(id) > 0 {
			reply.Result = result(errRefused)
			return nil
		}
		drops, err := tx.finishTech(state, id)
		if err != nil {
			return err
		}
		reply.Result = result(nil)
		reply.DropList = drops
		return tx.grantDrops(drops)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
