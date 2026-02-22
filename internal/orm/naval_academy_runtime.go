package orm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
)

const navalAcademyRuntimeCategory = "Runtime/naval_academy_runtime.json"

type NavalAcademyRuntime struct {
	CommanderID             uint32 `json:"commander_id"`
	OilWellLevel            uint32 `json:"oil_well_level"`
	GoldWellLevel           uint32 `json:"gold_well_level"`
	OilCollectTimestamp     uint32 `json:"oil_collect_timestamp"`
	GoldCollectTimestamp    uint32 `json:"gold_collect_timestamp"`
	OilUpgradeStartTime     uint32 `json:"oil_upgrade_start_time"`
	OilUpgradeCompleteTime  uint32 `json:"oil_upgrade_complete_time"`
	GoldUpgradeStartTime    uint32 `json:"gold_upgrade_start_time"`
	GoldUpgradeCompleteTime uint32 `json:"gold_upgrade_complete_time"`
}

func LoadNavalAcademyRuntime(commanderID uint32) (*NavalAcademyRuntime, error) {
	entry, err := GetConfigEntry(navalAcademyRuntimeCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		return nil, err
	}

	var runtime NavalAcademyRuntime
	if err := json.Unmarshal(entry.Data, &runtime); err != nil {
		return nil, err
	}
	runtime.CommanderID = commanderID
	return &runtime, nil
}

func LoadOrCreateNavalAcademyRuntime(commanderID uint32) (*NavalAcademyRuntime, error) {
	runtime, err := LoadNavalAcademyRuntime(commanderID)
	if err == nil {
		return runtime, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	return &NavalAcademyRuntime{
		CommanderID:   commanderID,
		OilWellLevel:  1,
		GoldWellLevel: 1,
	}, nil
}

func SaveNavalAcademyRuntime(runtime *NavalAcademyRuntime) error {
	if runtime == nil {
		return fmt.Errorf("naval academy runtime is nil")
	}

	payload, err := json.Marshal(runtime)
	if err != nil {
		return err
	}

	return UpsertConfigEntry(navalAcademyRuntimeCategory, strconv.FormatUint(uint64(runtime.CommanderID), 10), payload)
}
