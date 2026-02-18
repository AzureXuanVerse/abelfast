package answer

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	cityRebuildResultSuccess = uint32(0)
	cityRebuildResultFailed  = uint32(1)

	cityRebuildBuildingCategory = "ShareCfg/activity_ninja_building.json"
	cityRebuildBuffCategory     = "ShareCfg/activity_ninja_buff.json"

	cityRebuildBuildingTypeRebuild = uint32(1)
)

type cityRebuildBuildingConfig struct {
	ID        uint32   `json:"id"`
	Type      uint32   `json:"type"`
	PtCost    []uint32 `json:"pt_cost"`
	RoleID    uint32   `json:"role_id"`
	NeedLevel uint32   `json:"need_level"`
}

type cityRebuildBuffConfig struct {
	Group     uint32 `json:"group"`
	Level     uint32 `json:"level"`
	BasicCost uint32 `json:"basic_cost"`
}

func CityRebuildGetData(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26060{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26061, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26061, client, &protobuf.SC_26061{Result: proto.Uint32(cityRebuildResultFailed)})
	}

	response := &protobuf.SC_26061{
		Result: proto.Uint32(cityRebuildResultSuccess),
		Info:   buildCityRebuildInfo(state),
	}
	return connection.SendProtoMessage(26061, client, response)
}

func CityRebuildEndRecruit(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26062{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26063, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26063, client, buildCityRebuildAdjustResponse26063(cityRebuildResultFailed, nil))
	}

	roles := cityRebuildDedupeUint32(req.GetRoles())
	if len(roles) == 0 {
		return connection.SendProtoMessage(26063, client, buildCityRebuildAdjustResponse26063(cityRebuildResultFailed, nil))
	}

	recruitByID := make(map[uint32]orm.CityRebuildRecruit, len(state.Recruits))
	for _, recruit := range state.Recruits {
		recruitByID[recruit.ID] = recruit
	}
	for _, roleID := range roles {
		if _, ok := recruitByID[roleID]; !ok {
			return connection.SendProtoMessage(26063, client, buildCityRebuildAdjustResponse26063(cityRebuildResultFailed, nil))
		}
	}
	roleSet := make(map[uint32]struct{}, len(roles))
	for _, roleID := range roles {
		roleSet[roleID] = struct{}{}
	}

	nextRecruits := make([]orm.CityRebuildRecruit, 0, len(state.Recruits))
	for _, recruit := range state.Recruits {
		if _, done := roleSet[recruit.ID]; done {
			continue
		}
		nextRecruits = append(nextRecruits, recruit)
	}
	state.Recruits = nextRecruits
	state.Roles = append(state.Roles, roles...)
	state.SummaryReady = true
	if state.SummaryPt == 0 {
		state.SummaryPt = state.Pt / 10
	}
	if err := orm.SaveCityRebuildState(state); err != nil {
		return connection.SendProtoMessage(26063, client, buildCityRebuildAdjustResponse26063(cityRebuildResultFailed, nil))
	}

	return connection.SendProtoMessage(26063, client, buildCityRebuildAdjustResponse26063(cityRebuildResultSuccess, state))
}

func CityRebuildBuildingAction(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26064{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26065, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
	}

	buildings, err := loadCityRebuildBuildingConfigs()
	if err != nil {
		return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
	}

	building, ok := buildings[req.GetBuildingId()]
	if !ok {
		return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
	}
	if building.NeedLevel > 0 && state.CurLevel < building.NeedLevel {
		return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
	}

	cost := cityRebuildPtCost(building.PtCost)
	if state.Pt < cost {
		return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
	}

	if building.Type == cityRebuildBuildingTypeRebuild {
		if cityRebuildContainsUint32(state.Builds, building.ID) {
			return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
		}
		state.Builds = append(state.Builds, building.ID)
	} else {
		roleID := building.RoleID
		if roleID == 0 {
			roleID = building.ID
		}
		if cityRebuildContainsUint32(state.Roles, roleID) || cityRebuildContainsRecruit(state.Recruits, roleID) {
			return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
		}
		state.Recruits = append(state.Recruits, orm.CityRebuildRecruit{ID: roleID, StartTime: uint32(time.Now().Unix())})
	}

	state.Pt -= cost
	state.SummaryReady = true
	if state.SummaryPt == 0 {
		state.SummaryPt = state.Pt / 10
	}
	if err := orm.SaveCityRebuildState(state); err != nil {
		return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultFailed, nil))
	}

	return connection.SendProtoMessage(26065, client, buildCityRebuildAdjustResponse26065(cityRebuildResultSuccess, state))
}

func CityRebuildUpgradeBuff(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26066{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26067, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}
	if req.GetGroup() == 0 || req.GetCount() == 0 {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}

	costs, maxLevels, err := loadCityRebuildBuffUpgradeCosts()
	if err != nil {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}
	groupCosts := costs[req.GetGroup()]
	maxLevel := maxLevels[req.GetGroup()]
	if len(groupCosts) == 0 || maxLevel == 0 {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}

	currentLevel := state.Buffs[req.GetGroup()]
	targetLevel := currentLevel + req.GetCount()
	if targetLevel > maxLevel {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}

	var totalCost uint32
	for level := currentLevel + 1; level <= targetLevel; level++ {
		cost, ok := groupCosts[level]
		if !ok {
			return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
		}
		totalCost += cost
	}
	if state.Pt < totalCost {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}

	state.Pt -= totalCost
	state.Buffs[req.GetGroup()] = targetLevel
	state.SummaryReady = true
	if state.SummaryPt == 0 {
		state.SummaryPt = state.Pt / 10
	}
	if err := orm.SaveCityRebuildState(state); err != nil {
		return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultFailed, nil))
	}

	return connection.SendProtoMessage(26067, client, buildCityRebuildAdjustResponse26067(cityRebuildResultSuccess, state))
}

func CityRebuildResultSummary(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26068{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26069, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26069, client, buildCityRebuildSummaryResponse(cityRebuildResultFailed, nil))
	}
	if !state.SummaryReady {
		return connection.SendProtoMessage(26069, client, buildCityRebuildSummaryResponse(cityRebuildResultFailed, state))
	}

	summary := &protobuf.NINJA_SUMMARY{
		SummaryPt: buildNinjaPT(state.SummaryPt),
		AwardList: []*protobuf.DROPINFO{},
		Adjust:    buildCityRebuildAdjust(state),
	}
	state.SummaryReady = false
	state.SummaryPt = 0
	if err := orm.SaveCityRebuildState(state); err != nil {
		return connection.SendProtoMessage(26069, client, buildCityRebuildSummaryResponse(cityRebuildResultFailed, state))
	}

	response := &protobuf.SC_26069{
		Result:  proto.Uint32(cityRebuildResultSuccess),
		Summary: summary,
	}
	return connection.SendProtoMessage(26069, client, response)
}

func CityRebuildChooseLevel(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26070{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26071, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26071, client, buildCityRebuildAdjustResponse26071(cityRebuildResultFailed, nil))
	}
	if req.GetLevel() == 0 || req.GetLevel() > state.MaxLevel {
		return connection.SendProtoMessage(26071, client, buildCityRebuildAdjustResponse26071(cityRebuildResultFailed, nil))
	}

	state.CurLevel = req.GetLevel()
	state.SummaryReady = true
	if err := orm.SaveCityRebuildState(state); err != nil {
		return connection.SendProtoMessage(26071, client, buildCityRebuildAdjustResponse26071(cityRebuildResultFailed, nil))
	}
	return connection.SendProtoMessage(26071, client, buildCityRebuildAdjustResponse26071(cityRebuildResultSuccess, state))
}

func CityRebuildInitTime(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26072{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26073, err
	}

	state, err := loadCityRebuildStateForRequest(client, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26073, client, &protobuf.SC_26073{Result: proto.Uint32(cityRebuildResultFailed)})
	}

	state.AdjustTime = uint32(time.Now().Unix())
	if state.AdjustLeftHP == 0 {
		state.AdjustLeftHP = 100
	}
	state.AdjustMaxLevel = state.MaxLevel
	if err := orm.SaveCityRebuildState(state); err != nil {
		return connection.SendProtoMessage(26073, client, &protobuf.SC_26073{Result: proto.Uint32(cityRebuildResultFailed)})
	}

	response := &protobuf.SC_26073{
		Result: proto.Uint32(cityRebuildResultSuccess),
		Adjust: buildCityRebuildAdjust(state),
	}
	return connection.SendProtoMessage(26073, client, response)
}

func loadCityRebuildStateForRequest(client *connection.Client, actID uint32) (*orm.CityRebuildState, error) {
	if actID == 0 {
		return nil, fmt.Errorf("missing act_id")
	}
	template, err := loadActivityTemplate(actID)
	if err != nil {
		return nil, err
	}
	if !isActiveActivity(template.Time) {
		return nil, fmt.Errorf("inactive activity")
	}
	return orm.GetOrCreateCityRebuildState(client.Commander.CommanderID, actID)
}

func isActiveActivity(raw json.RawMessage) bool {
	var status string
	if err := json.Unmarshal(raw, &status); err == nil {
		return status != "stop"
	}
	return true
}

func loadCityRebuildBuildingConfigs() (map[uint32]cityRebuildBuildingConfig, error) {
	entries, err := orm.ListConfigEntries(cityRebuildBuildingCategory)
	if err != nil {
		return nil, err
	}
	configs := make(map[uint32]cityRebuildBuildingConfig, len(entries))
	for _, entry := range entries {
		if len(entry.Data) == 0 || entry.Data[0] != '{' {
			continue
		}
		cfg := cityRebuildBuildingConfig{}
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, err
		}
		if cfg.ID == 0 {
			continue
		}
		configs[cfg.ID] = cfg
	}
	return configs, nil
}

func loadCityRebuildBuffUpgradeCosts() (map[uint32]map[uint32]uint32, map[uint32]uint32, error) {
	entries, err := orm.ListConfigEntries(cityRebuildBuffCategory)
	if err != nil {
		return nil, nil, err
	}
	costs := make(map[uint32]map[uint32]uint32)
	maxLevels := make(map[uint32]uint32)
	for _, entry := range entries {
		if len(entry.Data) == 0 || entry.Data[0] != '{' {
			continue
		}
		cfg := cityRebuildBuffConfig{}
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, nil, err
		}
		if cfg.Group == 0 || cfg.Level == 0 {
			continue
		}
		if costs[cfg.Group] == nil {
			costs[cfg.Group] = make(map[uint32]uint32)
		}
		costs[cfg.Group][cfg.Level] = cfg.BasicCost
		if cfg.Level > maxLevels[cfg.Group] {
			maxLevels[cfg.Group] = cfg.Level
		}
	}
	return costs, maxLevels, nil
}

func cityRebuildPtCost(values []uint32) uint32 {
	if len(values) < 3 {
		return 0
	}
	return values[2]
}

func buildCityRebuildInfo(state *orm.CityRebuildState) *protobuf.NINJA_INFO {
	recruits := make([]*protobuf.NINJA_ROLE_RECRUIT, 0, len(state.Recruits))
	for _, recruit := range state.Recruits {
		recruits = append(recruits, &protobuf.NINJA_ROLE_RECRUIT{
			Id:        proto.Uint32(recruit.ID),
			StartTime: proto.Uint32(recruit.StartTime),
		})
	}
	buffs := make([]uint32, 0, len(state.Buffs))
	for group, level := range state.Buffs {
		if level == 0 {
			continue
		}
		buffs = append(buffs, group*1000+level)
	}
	sort.Slice(buffs, func(i int, j int) bool {
		return buffs[i] < buffs[j]
	})

	return &protobuf.NINJA_INFO{
		Pt:         buildNinjaPT(state.Pt),
		Builds:     append([]uint32{}, state.Builds...),
		Roles:      append([]uint32{}, state.Roles...),
		Recruits:   recruits,
		Buffs:      buffs,
		MaxLevel:   proto.Uint32(state.MaxLevel),
		CurLevel:   proto.Uint32(state.CurLevel),
		MaxDisplay: proto.Uint32(state.MaxDisplay),
		Adjust:     buildCityRebuildAdjust(state),
		SummaryPt:  buildNinjaPT(state.SummaryPt),
	}
}

func buildCityRebuildAdjust(state *orm.CityRebuildState) *protobuf.NINJA_ADJUST {
	timeValue := uint32(0)
	leftHP := uint32(0)
	maxLevel := uint32(1)
	if state != nil {
		timeValue = state.AdjustTime
		leftHP = state.AdjustLeftHP
		maxLevel = state.AdjustMaxLevel
		if maxLevel == 0 {
			maxLevel = state.MaxLevel
		}
		if maxLevel == 0 {
			maxLevel = 1
		}
	}
	return &protobuf.NINJA_ADJUST{
		Time:     proto.Uint32(timeValue),
		LeftHp:   buildNinjaPT(leftHP),
		MaxLevel: proto.Uint32(maxLevel),
	}
}

func buildNinjaPT(value uint32) *protobuf.NINJA_PT {
	return &protobuf.NINJA_PT{
		B: proto.Uint32(value),
		M: proto.Uint32(0),
		K: proto.Uint32(0),
	}
}

func buildCityRebuildAdjustResponse26063(result uint32, state *orm.CityRebuildState) *protobuf.SC_26063 {
	return &protobuf.SC_26063{Result: proto.Uint32(result), Adjust: buildCityRebuildAdjust(state)}
}

func buildCityRebuildAdjustResponse26065(result uint32, state *orm.CityRebuildState) *protobuf.SC_26065 {
	return &protobuf.SC_26065{Result: proto.Uint32(result), Adjust: buildCityRebuildAdjust(state)}
}

func buildCityRebuildAdjustResponse26067(result uint32, state *orm.CityRebuildState) *protobuf.SC_26067 {
	return &protobuf.SC_26067{Result: proto.Uint32(result), Adjust: buildCityRebuildAdjust(state)}
}

func buildCityRebuildAdjustResponse26071(result uint32, state *orm.CityRebuildState) *protobuf.SC_26071 {
	return &protobuf.SC_26071{Result: proto.Uint32(result), Adjust: buildCityRebuildAdjust(state)}
}

func buildCityRebuildSummaryResponse(result uint32, state *orm.CityRebuildState) *protobuf.SC_26069 {
	summaryPt := uint32(0)
	if state != nil {
		summaryPt = state.SummaryPt
	}
	return &protobuf.SC_26069{
		Result: proto.Uint32(result),
		Summary: &protobuf.NINJA_SUMMARY{
			SummaryPt: buildNinjaPT(summaryPt),
			AwardList: []*protobuf.DROPINFO{},
			Adjust:    buildCityRebuildAdjust(state),
		},
	}
}

func cityRebuildDedupeUint32(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	set := make(map[uint32]struct{}, len(values))
	result := make([]uint32, 0, len(values))
	for _, value := range values {
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cityRebuildContainsUint32(values []uint32, value uint32) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func cityRebuildContainsRecruit(values []orm.CityRebuildRecruit, roleID uint32) bool {
	for _, recruit := range values {
		if recruit.ID == roleID {
			return true
		}
	}
	return false
}
