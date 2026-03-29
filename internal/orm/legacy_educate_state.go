package orm

import (
	"encoding/json"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
)

const legacyEducateStateCategory = "Runtime/legacy_educate_state"

type LegacyEducateState struct {
	CommanderID   uint32            `json:"commander_id"`
	CallName      string            `json:"call_name"`
	TargetID      uint32            `json:"target_id"`
	FavorLv       uint32            `json:"favor_lv"`
	FavorExp      uint32            `json:"favor_exp"`
	HadAdjustment bool              `json:"had_adjustment"`
	Attrs         map[uint32]uint32 `json:"attrs"`
	TaskProgress  map[uint32]uint32 `json:"task_progress"`
	OptionRecords map[uint32]uint32 `json:"option_records"`
	Resources     map[uint32]int32  `json:"resources"`
	Endings       []uint32          `json:"endings"`
	Qualifieds    []uint32          `json:"qualifieds"`
}

func GetOrCreateLegacyEducateState(commanderID uint32) (*LegacyEducateState, error) {
	entry, err := GetConfigEntry(legacyEducateStateCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := defaultLegacyEducateState(commanderID)
		if err := SaveLegacyEducateState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	state := &LegacyEducateState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	normalizeLegacyEducateState(state, commanderID)
	return state, nil
}

func SaveLegacyEducateState(state *LegacyEducateState) error {
	normalizeLegacyEducateState(state, state.CommanderID)
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(legacyEducateStateCategory, strconv.FormatUint(uint64(state.CommanderID), 10), payload)
}

func defaultLegacyEducateState(commanderID uint32) *LegacyEducateState {
	return &LegacyEducateState{
		CommanderID:   commanderID,
		CallName:      "CHILD_USERNAME_SC_27001",
		FavorLv:       1,
		FavorExp:      0,
		Attrs:         map[uint32]uint32{201: 0, 202: 0, 203: 0},
		TaskProgress:  map[uint32]uint32{},
		OptionRecords: map[uint32]uint32{},
		Resources:     map[uint32]int32{3: 10},
		Endings:       []uint32{},
		Qualifieds:    []uint32{},
	}
}

func normalizeLegacyEducateState(state *LegacyEducateState, commanderID uint32) {
	state.CommanderID = commanderID
	if state.CallName == "" {
		state.CallName = "CHILD_USERNAME_SC_27001"
	}
	if state.FavorLv == 0 {
		state.FavorLv = 1
	}
	if state.Attrs == nil {
		state.Attrs = map[uint32]uint32{}
	}
	if _, ok := state.Attrs[201]; !ok {
		state.Attrs[201] = 0
	}
	if _, ok := state.Attrs[202]; !ok {
		state.Attrs[202] = 0
	}
	if _, ok := state.Attrs[203]; !ok {
		state.Attrs[203] = 0
	}
	if state.TaskProgress == nil {
		state.TaskProgress = map[uint32]uint32{}
	}
	if state.OptionRecords == nil {
		state.OptionRecords = map[uint32]uint32{}
	}
	if state.Resources == nil {
		state.Resources = map[uint32]int32{}
	}
	if _, ok := state.Resources[3]; !ok {
		state.Resources[3] = 10
	}
	if state.Endings == nil {
		state.Endings = []uint32{}
	}
	if state.Qualifieds == nil {
		state.Qualifieds = []uint32{}
	}
}
