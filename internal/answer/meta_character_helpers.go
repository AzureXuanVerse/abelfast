package answer

import (
	"context"
	"errors"
	"sort"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func ensureCommanderMetaLoaded(commander *orm.Commander) error {
	if commander.OwnedShipsMap != nil && commander.CommanderItemsMap != nil && commander.MiscItemsMap != nil && commander.OwnedResourcesMap != nil {
		return nil
	}
	return commander.Load()
}

func metaSkillSlots(ship *orm.OwnedShip) ([]orm.MetaTacticsSkillSlot, map[uint32]uint32, error) {
	slots, err := orm.GetMetaTacticsSkillSlotsByShipTemplate(ship.ShipID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	index := make(map[uint32]uint32, len(slots))
	for _, slot := range slots {
		index[slot.SkillID] = slot.Pos
	}
	return slots, index, nil
}

func metaSkillStateBySkill(states []orm.CommanderMetaTacticsSkillState) map[uint32]orm.CommanderMetaTacticsSkillState {
	indexed := make(map[uint32]orm.CommanderMetaTacticsSkillState, len(states))
	for _, state := range states {
		indexed[state.SkillID] = state
	}
	return indexed
}

func getMetaTacticsSnapshot(commanderID uint32, shipID uint32) (*orm.CommanderMetaTacticsState, []orm.CommanderMetaTacticsSkillState, []orm.CommanderMetaTacticsTaskProgress, error) {
	state, err := orm.GetCommanderMetaTacticsState(commanderID, shipID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, nil, nil, err
	}
	if errors.Is(err, db.ErrNotFound) {
		state = &orm.CommanderMetaTacticsState{CommanderID: commanderID, ShipID: shipID, SwitchCnt: orm.DefaultMetaTacticsSwitchCount}
	}
	skillStates, err := orm.ListCommanderMetaTacticsSkillStates(commanderID, shipID)
	if err != nil {
		return nil, nil, nil, err
	}
	tasks, err := orm.ListCommanderMetaTacticsTaskProgress(commanderID, shipID)
	if err != nil {
		return nil, nil, nil, err
	}
	return state, skillStates, tasks, nil
}

func ensureMetaSkillStatesTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32, slots []orm.MetaTacticsSkillSlot) ([]orm.CommanderMetaTacticsSkillState, error) {
	result := make([]orm.CommanderMetaTacticsSkillState, 0, len(slots))
	for _, slot := range slots {
		state, err := orm.GetOrCreateCommanderMetaTacticsSkillStateTx(ctx, tx, commanderID, shipID, slot.SkillID, slot.Pos)
		if err != nil {
			return nil, err
		}
		result = append(result, *state)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SkillPos < result[j].SkillPos
	})
	return result, nil
}

func buildMetaSkillExpPayload(states []orm.CommanderMetaTacticsSkillState) []*protobuf.SKILL_EXP {
	result := make([]*protobuf.SKILL_EXP, 0, len(states))
	for _, state := range states {
		if state.Level == 0 {
			continue
		}
		result = append(result, &protobuf.SKILL_EXP{SkillId: proto.Uint32(state.SkillID), Exp: proto.Uint32(state.Exp)})
	}
	return result
}
