package answer

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	shipyardResultOK           = uint32(0)
	shipyardResultFailed       = uint32(1)
	shipyardResultNoItems      = uint32(2)
	shipyardCooldownSeconds    = uint32(86400)
	shipyardStrengthRecordSlot = uint32(1)
)

func ensureCommanderLoadedForShipyard(clientCommander *orm.Commander) error {
	if clientCommander.OwnedShipsMap == nil || clientCommander.CommanderItemsMap == nil || clientCommander.OwnedResourcesMap == nil {
		return clientCommander.Load()
	}
	return nil
}

func listShipyardBlueprintProto(commanderID uint32) ([]*protobuf.BLUPRINTINFO, error) {
	entries, err := orm.ListCommanderShipyardBlueprints(commanderID)
	if err != nil {
		if db.IsNotFound(err) {
			return []*protobuf.BLUPRINTINFO{}, nil
		}
		return nil, err
	}
	out := make([]*protobuf.BLUPRINTINFO, 0, len(entries))
	for _, entry := range entries {
		copyEntry := entry
		out = append(out, &protobuf.BLUPRINTINFO{
			Id:             proto.Uint32(copyEntry.BlueprintID),
			ShipId:         proto.Uint32(copyEntry.ShipID),
			StartTime:      proto.Uint32(copyEntry.StartTime),
			BluePrintLevel: proto.Uint32(copyEntry.BluePrintLevel),
			Exp:            proto.Uint32(copyEntry.Exp),
			StartDuration:  proto.Uint32(copyEntry.StartDuration),
		})
	}
	return out, nil
}

func getShipyardStateOrDefault(commanderID uint32) (*orm.CommanderShipyardState, error) {
	state, err := orm.GetCommanderShipyardState(commanderID)
	if err != nil {
		if db.IsNotFound(err) {
			return &orm.CommanderShipyardState{CommanderID: commanderID}, nil
		}
		return nil, err
	}
	return state, nil
}

func getOrInitShipyardBlueprintTx(ctx context.Context, tx pgx.Tx, commanderID uint32, blueprintID uint32) (*orm.CommanderShipyardBlueprint, error) {
	entry, err := orm.GetCommanderShipyardBlueprintTx(ctx, tx, commanderID, blueprintID)
	if err == nil {
		return entry, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}
	entry = &orm.CommanderShipyardBlueprint{CommanderID: commanderID, BlueprintID: blueprintID}
	if err := orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func isShipyardTaskSatisfiedTx(ctx context.Context, tx pgx.Tx, commanderID uint32, taskID uint32) (bool, error) {
	task, err := orm.GetCommanderTaskTx(ctx, tx, commanderID, taskID)
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	template, err := orm.GetShipyardTaskTemplateConfig(taskID)
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if task.SubmitTime != 0 {
		return true, nil
	}
	if template.TargetNum == 0 {
		return true, nil
	}
	return task.Progress >= template.TargetNum, nil
}

func isBlueprintDevelopmentFinished(entry *orm.CommanderShipyardBlueprint, cfg *orm.ShipDataBlueprintConfig) (bool, error) {
	if entry == nil || cfg == nil {
		return false, nil
	}
	total, err := totalBlueprintLevels(cfg)
	if err != nil {
		return false, err
	}
	if entry.BluePrintLevel < total {
		return false, nil
	}
	return true, nil
}

func isShipyardBlueprintReadyToFinishTx(ctx context.Context, tx pgx.Tx, commanderID uint32, entry *orm.CommanderShipyardBlueprint, cfg *orm.ShipDataBlueprintConfig) (bool, error) {
	if entry == nil || cfg == nil {
		return false, nil
	}
	if entry.StartTime == 0 && entry.StartDuration == 0 {
		return false, nil
	}
	finished, err := isBlueprintDevelopmentFinished(entry, cfg)
	if err != nil {
		return false, err
	}
	if finished {
		return true, nil
	}

	seen := map[uint32]struct{}{}
	taskIDs := make([]uint32, 0, len(cfg.UnlockTaskOpenCondition)+len(cfg.UnlockTask))
	for _, taskID := range cfg.UnlockTaskOpenCondition {
		if taskID == 0 {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	for _, group := range cfg.UnlockTask {
		for _, taskID := range group {
			if taskID == 0 {
				continue
			}
			if _, ok := seen[taskID]; ok {
				continue
			}
			seen[taskID] = struct{}{}
			taskIDs = append(taskIDs, taskID)
		}
	}
	if len(taskIDs) == 0 {
		return false, nil
	}
	for _, taskID := range taskIDs {
		ok, err := isShipyardTaskSatisfiedTx(ctx, tx, commanderID, taskID)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func totalBlueprintLevels(cfg *orm.ShipDataBlueprintConfig) (uint32, error) {
	if cfg == nil {
		return 0, errors.New("nil blueprint config")
	}
	return uint32(len(cfg.StrengthenEffect) + len(cfg.FateStrengthen)), nil
}

func applyBlueprintExpGain(entry *orm.CommanderShipyardBlueprint, cfg *orm.ShipDataBlueprintConfig, shipLevel uint32, gain uint32) (bool, error) {
	if gain == 0 {
		return false, nil
	}
	totalLevels, err := totalBlueprintLevels(cfg)
	if err != nil {
		return false, err
	}
	if entry.BluePrintLevel >= totalLevels {
		return false, nil
	}

	remaining := gain
	level := entry.BluePrintLevel
	exp := entry.Exp

	for remaining > 0 && level < totalLevels {
		strengthID, ok := blueprintStrengthIDForLevel(cfg, level)
		if !ok {
			return false, nil
		}
		strengthCfg, err := orm.GetShipStrengthenBlueprintConfig(strengthID)
		if err != nil {
			return false, err
		}
		if shipLevel < strengthCfg.NeedLV {
			return false, nil
		}
		if strengthCfg.NeedExp == 0 {
			level++
			exp = 0
			continue
		}
		if exp >= strengthCfg.NeedExp {
			level++
			exp = 0
			continue
		}
		need := strengthCfg.NeedExp - exp
		if remaining >= need {
			remaining -= need
			level++
			exp = 0
			continue
		}
		exp += remaining
		remaining = 0
	}

	entry.BluePrintLevel = level
	if level >= totalLevels {
		entry.Exp = 0
	} else {
		entry.Exp = exp
	}
	return true, nil
}

func blueprintStrengthIDForLevel(cfg *orm.ShipDataBlueprintConfig, level uint32) (uint32, bool) {
	if level < uint32(len(cfg.StrengthenEffect)) {
		return cfg.StrengthenEffect[level], true
	}
	fateIndex := level - uint32(len(cfg.StrengthenEffect))
	if fateIndex < uint32(len(cfg.FateStrengthen)) {
		return cfg.FateStrengthen[fateIndex], true
	}
	return 0, false
}

func shipyardNowUnix() uint32 {
	return uint32(time.Now().UTC().Unix())
}
