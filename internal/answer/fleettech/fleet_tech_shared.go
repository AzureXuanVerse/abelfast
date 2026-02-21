package fleettech

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	ResultFailure = uint32(1)
	ResultSuccess = uint32(0)

	OneStepClaimType = uint32(1)
)

var errFleetTechValidation = fmt.Errorf("fleet tech validation failed")

type fleetTechGroupConfig struct {
	ID    uint32   `json:"id"`
	Techs []uint32 `json:"techs"`
}

type fleetTechTemplateConfig struct {
	ID                uint32 `json:"id"`
	GroupID           uint32 `json:"groupid"`
	Cost              uint32 `json:"cost"`
	Time              uint32 `json:"time"`
	Add               []any  `json:"add"`
	LevelAwardDisplay []any  `json:"level_award_display"`
}

type fleetTechAttrKey struct {
	ShipType uint32
	AttrType uint32
}

func loadFleetTechConfigs() (map[uint32]fleetTechGroupConfig, map[uint32]fleetTechTemplateConfig, error) {
	groupEntries, err := orm.ListConfigEntries("ShareCfg/fleet_tech_group.json")
	if err != nil {
		return nil, nil, err
	}
	templateEntries, err := orm.ListConfigEntries("ShareCfg/fleet_tech_template.json")
	if err != nil {
		return nil, nil, err
	}

	groups := make(map[uint32]fleetTechGroupConfig, len(groupEntries))
	for _, entry := range groupEntries {
		var group fleetTechGroupConfig
		if err := json.Unmarshal(entry.Data, &group); err != nil {
			return nil, nil, err
		}
		if group.ID == 0 {
			if value, parseErr := strconv.ParseUint(entry.Key, 10, 32); parseErr == nil {
				group.ID = uint32(value)
			}
		}
		if group.ID == 0 {
			continue
		}
		groups[group.ID] = group
	}

	templates := make(map[uint32]fleetTechTemplateConfig, len(templateEntries))
	for _, entry := range templateEntries {
		var template fleetTechTemplateConfig
		if err := json.Unmarshal(entry.Data, &template); err != nil {
			return nil, nil, err
		}
		if template.ID == 0 {
			if value, parseErr := strconv.ParseUint(entry.Key, 10, 32); parseErr == nil {
				template.ID = uint32(value)
			}
		}
		if template.ID == 0 {
			continue
		}
		templates[template.ID] = template
	}

	return groups, templates, nil
}

func fleetTechIndexOfTech(group fleetTechGroupConfig, techID uint32) int {
	for index, id := range group.Techs {
		if id == techID {
			return index
		}
	}
	return -1
}

func fleetTechExpectedNextTech(group fleetTechGroupConfig, effectTechID uint32) (uint32, bool) {
	if len(group.Techs) == 0 {
		return 0, false
	}
	if effectTechID == 0 {
		return group.Techs[0], true
	}
	currentIndex := fleetTechIndexOfTech(group, effectTechID)
	if currentIndex < 0 {
		return 0, false
	}
	nextIndex := currentIndex + 1
	if nextIndex >= len(group.Techs) {
		return 0, false
	}
	return group.Techs[nextIndex], true
}

func fleetTechHasActiveStudy(state *orm.CommanderFleetTechState) bool {
	for i := range state.Groups {
		if state.Groups[i].StudyTechID != 0 {
			return true
		}
	}
	return false
}

func fleetTechApplyTemplateAdditions(max map[fleetTechAttrKey]uint32, rawAdd []any) {
	for _, rawEntry := range rawAdd {
		entry, ok := rawEntry.([]any)
		if !ok || len(entry) < 3 {
			continue
		}
		shipTypes, ok := entry[0].([]any)
		if !ok {
			continue
		}
		attrType, ok := parseUint32Value(entry[1])
		if !ok || attrType == 0 {
			continue
		}
		value, ok := parseUint32Value(entry[2])
		if !ok {
			continue
		}
		for _, rawShipType := range shipTypes {
			shipType, ok := parseUint32Value(rawShipType)
			if !ok || shipType == 0 {
				continue
			}
			key := fleetTechAttrKey{ShipType: shipType, AttrType: attrType}
			max[key] += value
		}
	}
}

func fleetTechBuildMaxAdditions(state *orm.CommanderFleetTechState, groups map[uint32]fleetTechGroupConfig, templates map[uint32]fleetTechTemplateConfig) map[fleetTechAttrKey]uint32 {
	max := make(map[fleetTechAttrKey]uint32)
	for _, groupState := range state.Groups {
		group, ok := groups[groupState.GroupID]
		if !ok {
			continue
		}
		completedIndex := fleetTechIndexOfTech(group, groupState.EffectTechID)
		if completedIndex < 0 {
			continue
		}
		for i := 0; i <= completedIndex; i++ {
			template, ok := templates[group.Techs[i]]
			if !ok {
				continue
			}
			fleetTechApplyTemplateAdditions(max, template.Add)
		}
	}
	return max
}

func fleetTechBuildTechSetList(state *orm.CommanderFleetTechState, max map[fleetTechAttrKey]uint32) []*protobuf.TECHSET {
	result := make([]*protobuf.TECHSET, 0, len(state.AttrOverrides))
	for _, override := range state.AttrOverrides {
		key := fleetTechAttrKey{ShipType: override.ShipType, AttrType: override.AttrType}
		maxValue, ok := max[key]
		if !ok {
			continue
		}
		if override.SetValue > maxValue {
			continue
		}
		if override.SetValue == maxValue {
			continue
		}
		result = append(result, &protobuf.TECHSET{
			ShipType: proto.Uint32(override.ShipType),
			AttrType: proto.Uint32(override.AttrType),
			SetValue: proto.Uint32(override.SetValue),
		})
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].GetShipType() != result[j].GetShipType() {
			return result[i].GetShipType() < result[j].GetShipType()
		}
		return result[i].GetAttrType() < result[j].GetAttrType()
	})
	return result
}

func fleetTechNormalizeOverrides(payload []*protobuf.TECHSET, max map[fleetTechAttrKey]uint32) ([]orm.FleetTechAttrOverride, bool) {
	if len(payload) == 0 {
		return []orm.FleetTechAttrOverride{}, true
	}
	index := map[fleetTechAttrKey]int{}
	overrides := make([]orm.FleetTechAttrOverride, 0, len(payload))
	for _, set := range payload {
		if set == nil {
			return nil, false
		}
		shipType := set.GetShipType()
		attrType := set.GetAttrType()
		setValue := set.GetSetValue()
		if shipType == 0 || attrType == 0 {
			return nil, false
		}
		key := fleetTechAttrKey{ShipType: shipType, AttrType: attrType}
		maxValue, ok := max[key]
		if !ok {
			return nil, false
		}
		if setValue > maxValue {
			return nil, false
		}
		normalized := orm.FleetTechAttrOverride{ShipType: shipType, AttrType: attrType, SetValue: setValue}
		if existing, ok := index[key]; ok {
			overrides[existing] = normalized
			continue
		}
		index[key] = len(overrides)
		overrides = append(overrides, normalized)
	}
	effective := make([]orm.FleetTechAttrOverride, 0, len(overrides))
	for _, override := range overrides {
		maxValue := max[fleetTechAttrKey{ShipType: override.ShipType, AttrType: override.AttrType}]
		if override.SetValue == maxValue {
			continue
		}
		effective = append(effective, override)
	}
	sort.Slice(effective, func(i int, j int) bool {
		if effective[i].ShipType != effective[j].ShipType {
			return effective[i].ShipType < effective[j].ShipType
		}
		return effective[i].AttrType < effective[j].AttrType
	})
	return effective, true
}

func fleetTechClaimDrops(template fleetTechTemplateConfig) []*protobuf.DROPINFO {
	merged := map[string]*protobuf.DROPINFO{}
	for _, rawReward := range template.LevelAwardDisplay {
		row, ok := rawReward.([]any)
		if !ok || len(row) < 3 {
			continue
		}
		dropType, ok := parseUint32Value(row[0])
		if !ok {
			continue
		}
		dropID, ok := parseUint32Value(row[1])
		if !ok {
			continue
		}
		count, ok := parseUint32Value(row[2])
		if !ok || count == 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d", dropType, dropID)
		if existing, ok := merged[key]; ok {
			existing.Number = proto.Uint32(existing.GetNumber() + count)
			continue
		}
		merged[key] = &protobuf.DROPINFO{Type: proto.Uint32(dropType), Id: proto.Uint32(dropID), Number: proto.Uint32(count)}
	}
	return fleetTechDropMapToSlice(merged)
}

func fleetTechMergeDropList(merged map[string]*protobuf.DROPINFO, drops []*protobuf.DROPINFO) {
	for _, drop := range drops {
		key := fmt.Sprintf("%d:%d", drop.GetType(), drop.GetId())
		if existing, ok := merged[key]; ok {
			existing.Number = proto.Uint32(existing.GetNumber() + drop.GetNumber())
			continue
		}
		merged[key] = &protobuf.DROPINFO{Type: proto.Uint32(drop.GetType()), Id: proto.Uint32(drop.GetId()), Number: proto.Uint32(drop.GetNumber())}
	}
}

func fleetTechDropMapToSlice(merged map[string]*protobuf.DROPINFO) []*protobuf.DROPINFO {
	result := make([]*protobuf.DROPINFO, 0, len(merged))
	for _, drop := range merged {
		result = append(result, drop)
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].GetType() != result[j].GetType() {
			return result[i].GetType() < result[j].GetType()
		}
		return result[i].GetId() < result[j].GetId()
	})
	return result
}

func parseUint32Value(value any) (uint32, bool) {
	switch v := value.(type) {
	case float64:
		return uint32(v), true
	case float32:
		return uint32(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case uint32:
		return v, true
	case uint64:
		return uint32(v), true
	case json.Number:
		n, err := strconv.ParseUint(v.String(), 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(n), true
	default:
		return 0, false
	}
}

func withFleetTechStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, fn func(*orm.CommanderFleetTechState) error) error {
	state, err := orm.GetOrCreateCommanderFleetTechStateTx(ctx, tx, commanderID)
	if err != nil {
		return err
	}
	if err := fn(state); err != nil {
		return err
	}
	return orm.SaveCommanderFleetTechStateTx(ctx, tx, state)
}
