package answer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const islandFormulaCategory = "ShareCfg/island_formula.json"

const (
	islandDelegationResultSuccess      = uint32(0)
	islandDelegationResultInvalid      = uint32(1)
	islandDelegationResultState        = uint32(2)
	islandDelegationResultNoReward     = uint32(3)
	islandDelegationResultPersistError = uint32(4)
)

type islandFormulaConfig struct {
	ID                uint32     `json:"id"`
	CommissionProduct [][]uint32 `json:"commission_product"`
	SecondProduct     [][]uint32 `json:"second_product"`
	PTAward           uint32     `json:"pt_award"`
}

func IslandGetDelegationAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21505
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21506, err
	}

	response := &protobuf.SC_21506{
		Result:    proto.Uint32(islandDelegationResultSuccess),
		GetTimes:  proto.Uint32(0),
		PtAward:   proto.Uint32(0),
		FormulaId: proto.Uint32(0),
	}

	claimType := payload.GetType()
	buildID := payload.GetBuildId()
	areaID := payload.GetAreaId()
	if buildID == 0 || areaID == 0 || (claimType != 1 && claimType != 2) {
		response.Result = proto.Uint32(islandDelegationResultInvalid)
		return client.SendMessage(21506, response)
	}

	if err := ensureCommanderLoaded(client, "Island/DelegationAward"); err != nil {
		response.Result = proto.Uint32(islandDelegationResultPersistError)
		return client.SendMessage(21506, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandDelegationForUpdateTx(context.Background(), tx, client.Commander.CommanderID, buildID, areaID)
		if err != nil {
			response.Result = proto.Uint32(islandDelegationResultState)
			return nil
		}

		if !slot.RewardReady {
			response.Result = proto.Uint32(islandDelegationResultNoReward)
			return nil
		}
		if claimType == 1 && !slot.HasRole {
			response.Result = proto.Uint32(islandDelegationResultState)
			return nil
		}
		if claimType == 2 && slot.HasRole {
			response.Result = proto.Uint32(islandDelegationResultState)
			return nil
		}

		formula, ok, err := loadIslandFormula(slot.FormulaID)
		if err != nil {
			response.Result = proto.Uint32(islandDelegationResultPersistError)
			return nil
		}
		if !ok {
			response.Result = proto.Uint32(islandDelegationResultState)
			return nil
		}

		drops := buildIslandDelegationDrops(slot, formula)
		ptAward := slot.PTAward
		if ptAward == 0 {
			ptAward = formula.PTAward
		}
		if len(drops) == 0 && ptAward == 0 {
			response.Result = proto.Uint32(islandDelegationResultNoReward)
			return nil
		}

		for i := range drops {
			if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, drops[i].GetId(), drops[i].GetNumber()); err != nil {
				response.Result = proto.Uint32(islandDelegationResultPersistError)
				return err
			}
		}
		if err := orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, ptAward); err != nil {
			response.Result = proto.Uint32(islandDelegationResultPersistError)
			return err
		}

		getTimes, err := orm.ApplyIslandDelegationClaimTx(context.Background(), tx, client.Commander.CommanderID, buildID, areaID, claimType)
		if err != nil {
			response.Result = proto.Uint32(islandDelegationResultPersistError)
			return err
		}

		response.Result = proto.Uint32(islandDelegationResultSuccess)
		response.DropList = drops
		response.GetTimes = proto.Uint32(getTimes)
		response.PtAward = proto.Uint32(ptAward)
		response.FormulaId = proto.Uint32(slot.FormulaID)
		return nil
	})
	if err != nil {
		return client.SendMessage(21506, response)
	}

	return client.SendMessage(21506, response)
}

func buildIslandDelegationDrops(slot *orm.IslandDelegation, formula *islandFormulaConfig) []*protobuf.DROPINFO {
	drops := make([]*protobuf.DROPINFO, 0, 2)

	mainItemID, mainBase := islandFormulaProduct(formula.CommissionProduct)
	mainCount := mainBase*slot.MainNum + slot.ExtraMainNum
	if mainCount > 0 {
		drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, mainItemID, mainCount))
	}

	otherItemID, otherBase := islandFormulaProduct(formula.SecondProduct)
	otherCount := otherBase*slot.OtherNum + slot.ExtraOtherNum
	if otherCount > 0 {
		drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, otherItemID, otherCount))
	}

	return drops
}

func islandFormulaProduct(list [][]uint32) (uint32, uint32) {
	if len(list) == 0 || len(list[0]) < 2 {
		return 0, 0
	}
	return list[0][0], list[0][1]
}

func loadIslandFormula(formulaID uint32) (*islandFormulaConfig, bool, error) {
	key := fmt.Sprintf("%d", formulaID)
	if entry, err := orm.GetConfigEntry(islandFormulaCategory, key); err == nil {
		var direct islandFormulaConfig
		if err := json.Unmarshal(entry.Data, &direct); err == nil {
			if direct.ID == 0 {
				direct.ID = formulaID
			}
			return &direct, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandFormulaCategory)
	if err != nil {
		return nil, false, err
	}
	for i := range entries {
		var single islandFormulaConfig
		if err := json.Unmarshal(entries[i].Data, &single); err == nil {
			if single.ID == formulaID {
				return &single, true, nil
			}
		}
		var list []islandFormulaConfig
		if err := json.Unmarshal(entries[i].Data, &list); err == nil {
			for j := range list {
				if list[j].ID == formulaID {
					return &list[j], true, nil
				}
			}
		}
	}

	return nil, false, nil
}
