package island

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandTechResultSuccess = uint32(0)
	islandTechResultFailed  = uint32(1)
)

const (
	islandTechCategory      = "ShareCfg/island_technology_template.json"
	islandTechCategoryLC    = "sharecfgdata/island_technology_template.json"
	islandFormulaCategoryLC = "sharecfgdata/island_formula.json"
)

type islandTechnologyTemplate struct {
	ID          uint32     `json:"id"`
	FormulaID   uint32     `json:"formula_id"`
	IslandLevel uint32     `json:"island_level"`
	SysUnlock   [][]uint32 `json:"sys_unlock"`
	TechRepeat  uint32     `json:"tech_repeat"`
}

type islandFormulaTemplate struct {
	ID                uint32     `json:"id"`
	UnlockType        uint32     `json:"unlock_type"`
	CommissionProduct [][]uint32 `json:"commission_product"`
	DropList          [][]uint32 `json:"drop_list"`
}

func IslandUnlockTech(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21520
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21521, err
	}

	response := &protobuf.SC_21521{Result: proto.Uint32(islandTechResultFailed)}
	if client.Commander == nil {
		return client.SendMessage(21521, response)
	}

	techID := payload.GetTechId()
	if techID == 0 {
		return client.SendMessage(21521, response)
	}

	techTemplate, ok := loadIslandTechnologyTemplate(techID)
	if !ok {
		return client.SendMessage(21521, response)
	}

	state, err := orm.GetIslandTechnologyState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21521, response)
		}
		state = orm.NewIslandTechnologyState(client.Commander.CommanderID)
	}

	if containsIslandUint32(state.UnlockedTechIDs, techID) {
		return client.SendMessage(21521, response)
	}

	requiredIslandLevel := maxUint32(techTemplate.IslandLevel, 1)
	islandLevel := uint32(1)
	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21521, response)
		}
	} else {
		islandLevel = maxUint32(snapshot.Level, 1)
	}
	if islandLevel < requiredIslandLevel {
		return client.SendMessage(21521, response)
	}

	if !isIslandTechUnlockedByConditions(state, techTemplate) {
		return client.SendMessage(21521, response)
	}

	formulaTemplate, _ := loadIslandFormulaTemplate(techTemplate.FormulaID)
	state.UnlockedTechIDs = append(state.UnlockedTechIDs, techID)
	if formulaTemplate != nil && formulaTemplate.UnlockType > 0 && !containsIslandUint32(state.AbilityIDs, formulaTemplate.UnlockType) {
		state.AbilityIDs = append(state.AbilityIDs, formulaTemplate.UnlockType)
	}
	sort.Slice(state.UnlockedTechIDs, func(i, j int) bool { return state.UnlockedTechIDs[i] < state.UnlockedTechIDs[j] })
	sort.Slice(state.AbilityIDs, func(i, j int) bool { return state.AbilityIDs[i] < state.AbilityIDs[j] })

	if err := orm.UpsertIslandTechnologyState(state); err != nil {
		return client.SendMessage(21521, response)
	}

	response.Result = proto.Uint32(islandTechResultSuccess)
	return client.SendMessage(21521, response)
}

func IslandFinishTechImmediate(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21522
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21523, err
	}

	response := &protobuf.SC_21523{Result: proto.Uint32(islandTechResultFailed), DropList: []*protobuf.DROPINFO{}}
	if client.Commander == nil {
		return client.SendMessage(21523, response)
	}

	techID := payload.GetTechId()
	if techID == 0 {
		return client.SendMessage(21523, response)
	}

	techTemplate, ok := loadIslandTechnologyTemplate(techID)
	if !ok {
		return client.SendMessage(21523, response)
	}

	state, err := orm.GetIslandTechnologyState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21523, response)
		}
		state = orm.NewIslandTechnologyState(client.Commander.CommanderID)
	}

	if !containsIslandUint32(state.UnlockedTechIDs, techID) {
		return client.SendMessage(21523, response)
	}

	finishedCount := state.FinishCounts[techID]
	if techTemplate.TechRepeat > 0 && finishedCount >= techTemplate.TechRepeat {
		return client.SendMessage(21523, response)
	}

	formulaTemplate, ok := loadIslandFormulaTemplate(techTemplate.FormulaID)
	if !ok {
		return client.SendMessage(21523, response)
	}

	drops := buildIslandTechDrops(formulaTemplate)
	for _, drop := range drops {
		ok, err := applyDrop(client, drop.GetType(), drop.GetId(), drop.GetNumber())
		if err != nil || !ok {
			return client.SendMessage(21523, response)
		}
	}

	state.FinishCounts[techID] = finishedCount + 1
	if err := orm.UpsertIslandTechnologyState(state); err != nil {
		return client.SendMessage(21523, response)
	}

	response.Result = proto.Uint32(islandTechResultSuccess)
	response.DropList = drops
	return client.SendMessage(21523, response)
}

func isIslandTechUnlockedByConditions(state *orm.IslandTechnologyState, template *islandTechnologyTemplate) bool {
	for _, condition := range template.SysUnlock {
		if len(condition) < 2 {
			continue
		}
		switch condition[0] {
		case 2:
			if !containsIslandUint32(state.UnlockedTechIDs, condition[1]) {
				return false
			}
		case 4:
			if !containsIslandUint32(state.AbilityIDs, condition[1]) {
				return false
			}
		}
	}
	return true
}

func buildIslandTechDrops(formula *islandFormulaTemplate) []*protobuf.DROPINFO {
	if formula == nil {
		return []*protobuf.DROPINFO{}
	}
	if len(formula.DropList) > 0 {
		drops := make([]*protobuf.DROPINFO, 0, len(formula.DropList))
		for _, entry := range formula.DropList {
			if len(entry) < 3 {
				continue
			}
			drops = append(drops, newDropInfo(entry[0], entry[1], entry[2]))
		}
		return drops
	}
	if len(formula.CommissionProduct) > 0 && len(formula.CommissionProduct[0]) >= 2 {
		return []*protobuf.DROPINFO{newDropInfo(consts.DROP_TYPE_ITEM, formula.CommissionProduct[0][0], formula.CommissionProduct[0][1])}
	}
	return []*protobuf.DROPINFO{}
}

func loadIslandTechnologyTemplate(techID uint32) (*islandTechnologyTemplate, bool) {
	if template, ok := loadIslandTechnologyTemplateFromCategory(islandTechCategory, techID); ok {
		return template, true
	}
	return loadIslandTechnologyTemplateFromCategory(islandTechCategoryLC, techID)
}

func loadIslandTechnologyTemplateFromCategory(category string, techID uint32) (*islandTechnologyTemplate, bool) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(techID), 10))
	if err == nil {
		var template islandTechnologyTemplate
		if json.Unmarshal(entry.Data, &template) == nil {
			if template.ID == 0 {
				template.ID = techID
			}
			return &template, true
		}
	}
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return nil, false
	}
	for _, row := range entries {
		var template islandTechnologyTemplate
		if json.Unmarshal(row.Data, &template) == nil {
			if template.ID == techID {
				return &template, true
			}
		}
	}
	return nil, false
}

func loadIslandFormulaTemplate(formulaID uint32) (*islandFormulaTemplate, bool) {
	if formula, ok := loadIslandFormulaTemplateFromCategory(islandFormulaCategory, formulaID); ok {
		return formula, true
	}
	return loadIslandFormulaTemplateFromCategory(islandFormulaCategoryLC, formulaID)
}

func loadIslandFormulaTemplateFromCategory(category string, formulaID uint32) (*islandFormulaTemplate, bool) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(formulaID), 10))
	if err == nil {
		var template islandFormulaTemplate
		if json.Unmarshal(entry.Data, &template) == nil {
			if template.ID == 0 {
				template.ID = formulaID
			}
			return &template, true
		}
	}
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return nil, false
	}
	for _, row := range entries {
		var template islandFormulaTemplate
		if json.Unmarshal(row.Data, &template) == nil {
			if template.ID == formulaID {
				return &template, true
			}
		}
	}
	return nil, false
}

func containsIslandUint32(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
