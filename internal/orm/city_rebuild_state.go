package orm

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ggmolly/belfast/internal/db"
)

const cityRebuildStateCategory = "Runtime/city_rebuild_state"

type CityRebuildRecruit struct {
	ID        uint32 `json:"id"`
	StartTime uint32 `json:"start_time"`
}

type CityRebuildState struct {
	CommanderID uint32 `json:"commander_id"`
	ActID       uint32 `json:"act_id"`

	Pt       uint32               `json:"pt"`
	Builds   []uint32             `json:"builds"`
	Roles    []uint32             `json:"roles"`
	Recruits []CityRebuildRecruit `json:"recruits"`
	Buffs    map[uint32]uint32    `json:"buffs"`

	MaxLevel   uint32 `json:"max_level"`
	CurLevel   uint32 `json:"cur_level"`
	MaxDisplay uint32 `json:"max_display"`

	AdjustTime     uint32 `json:"adjust_time"`
	AdjustLeftHP   uint32 `json:"adjust_left_hp"`
	AdjustMaxLevel uint32 `json:"adjust_max_level"`

	SummaryPt    uint32 `json:"summary_pt"`
	SummaryReady bool   `json:"summary_ready"`
}

func GetOrCreateCityRebuildState(commanderID uint32, actID uint32) (*CityRebuildState, error) {
	entry, err := GetConfigEntry(cityRebuildStateCategory, cityRebuildStateKey(commanderID, actID))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := defaultCityRebuildState(commanderID, actID)
		if err := SaveCityRebuildState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	state := &CityRebuildState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	normalizeCityRebuildState(state, commanderID, actID)
	return state, nil
}

func SaveCityRebuildState(state *CityRebuildState) error {
	normalizeCityRebuildState(state, state.CommanderID, state.ActID)
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(cityRebuildStateCategory, cityRebuildStateKey(state.CommanderID, state.ActID), payload)
}

func defaultCityRebuildState(commanderID uint32, actID uint32) *CityRebuildState {
	return &CityRebuildState{
		CommanderID: commanderID,
		ActID:       actID,
		Builds:      []uint32{},
		Roles:       []uint32{},
		Recruits:    []CityRebuildRecruit{},
		Buffs:       map[uint32]uint32{},
		MaxLevel:    1,
		CurLevel:    1,
		MaxDisplay:  1,
	}
}

func normalizeCityRebuildState(state *CityRebuildState, commanderID uint32, actID uint32) {
	state.CommanderID = commanderID
	state.ActID = actID
	if state.Builds == nil {
		state.Builds = []uint32{}
	}
	if state.Roles == nil {
		state.Roles = []uint32{}
	}
	if state.Recruits == nil {
		state.Recruits = []CityRebuildRecruit{}
	}
	if state.Buffs == nil {
		state.Buffs = map[uint32]uint32{}
	}
	if state.MaxLevel == 0 {
		state.MaxLevel = 1
	}
	if state.CurLevel == 0 {
		state.CurLevel = 1
	}
	if state.CurLevel > state.MaxLevel {
		state.CurLevel = state.MaxLevel
	}
	if state.MaxDisplay < state.MaxLevel {
		state.MaxDisplay = state.MaxLevel
	}
	if state.AdjustMaxLevel == 0 {
		state.AdjustMaxLevel = state.MaxLevel
	}

	state.Builds = uniqueSortedUint32(state.Builds)
	state.Roles = uniqueSortedUint32(state.Roles)
	state.Recruits = uniqueSortedRecruits(state.Recruits)
}

func cityRebuildStateKey(commanderID uint32, actID uint32) string {
	return fmt.Sprintf("%d:%d", commanderID, actID)
}

func uniqueSortedUint32(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	set := make(map[uint32]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]uint32, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func uniqueSortedRecruits(values []CityRebuildRecruit) []CityRebuildRecruit {
	if len(values) == 0 {
		return []CityRebuildRecruit{}
	}
	index := make(map[uint32]CityRebuildRecruit, len(values))
	for _, recruit := range values {
		if recruit.ID == 0 {
			continue
		}
		current, ok := index[recruit.ID]
		if !ok || recruit.StartTime < current.StartTime {
			index[recruit.ID] = recruit
		}
	}
	result := make([]CityRebuildRecruit, 0, len(index))
	for _, recruit := range index {
		result = append(result, recruit)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}
