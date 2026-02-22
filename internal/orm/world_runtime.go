package orm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/scheduler"
)

const (
	worldRuntimeCategory = "Runtime/world_runtime.json"

	worldGamesetCategory          = "ShareCfg/gameset.json"
	worldGamesetCategoryLowerCase = "sharecfgdata/gameset.json"
	worldMovePowerMaxKey          = "world_movepower_maxvalue"
	worldMovePowerRecoveryKey     = "world_movepower_recovery_interval"

	defaultWorldMovePowerMax      = 200
	defaultWorldMovePowerRecovery = 600
)

type WorldRuntime struct {
	CommanderID               uint32            `json:"commander_id"`
	Camp                      uint32            `json:"camp"`
	MapID                     uint32            `json:"map_id"`
	EnterMapID                uint32            `json:"enter_map_id"`
	ActionPower               uint32            `json:"action_power"`
	ActionPowerExtra          uint32            `json:"action_power_extra"`
	ActionPowerFetchCount     uint32            `json:"action_power_fetch_count"`
	LastRecoverTimestamp      uint32            `json:"last_recover_timestamp"`
	LastChangeGroupTimestamp  uint32            `json:"last_change_group_timestamp"`
	Progress                  uint32            `json:"progress"`
	TaskFinishCount           uint32            `json:"task_finish_count"`
	StaminaExchangeTimes      uint32            `json:"stamina_exchange_times"`
	Round                     uint32            `json:"round"`
	WeekStartUnix             uint32            `json:"week_start_unix,omitempty"`
	MonthKey                  uint32            `json:"month_key,omitempty"`
	SairenChapter             []uint32          `json:"sairen_chapter,omitempty"`
	MapTemplateByRandomID     map[string]uint32 `json:"map_template_by_random_id,omitempty"`
	FleetShipIDs              []uint32          `json:"fleet_ship_ids,omitempty"`
	CommanderIDs              []uint32          `json:"commander_ids,omitempty"`
	ResetAvailableAtTimestamp uint32            `json:"reset_available_at_timestamp"`
}

func LoadWorldRuntime(commanderID uint32) (*WorldRuntime, error) {
	entry, err := GetConfigEntry(worldRuntimeCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		return nil, err
	}
	var runtime WorldRuntime
	if err := json.Unmarshal(entry.Data, &runtime); err != nil {
		return nil, err
	}
	runtime.CommanderID = commanderID
	if runtime.MapTemplateByRandomID == nil {
		runtime.MapTemplateByRandomID = make(map[string]uint32)
	}
	return &runtime, nil
}

func LoadOrCreateWorldRuntime(commanderID uint32) (*WorldRuntime, error) {
	runtime, err := LoadWorldRuntime(commanderID)
	if err == nil {
		return runtime, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	return &WorldRuntime{
		CommanderID:              commanderID,
		ActionPower:              200,
		ActionPowerExtra:         0,
		ActionPowerFetchCount:    0,
		LastRecoverTimestamp:     0,
		LastChangeGroupTimestamp: 0,
		Progress:                 0,
		TaskFinishCount:          0,
		StaminaExchangeTimes:     0,
		Round:                    0,
		SairenChapter:            []uint32{},
		MapTemplateByRandomID:    make(map[string]uint32),
		FleetShipIDs:             []uint32{},
		CommanderIDs:             []uint32{},
	}, nil
}

func SaveWorldRuntime(runtime *WorldRuntime) error {
	if runtime == nil {
		return fmt.Errorf("world runtime is nil")
	}
	if runtime.MapTemplateByRandomID == nil {
		runtime.MapTemplateByRandomID = make(map[string]uint32)
	}
	if runtime.SairenChapter == nil {
		runtime.SairenChapter = []uint32{}
	}
	payload, err := json.Marshal(runtime)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(worldRuntimeCategory, strconv.FormatUint(uint64(runtime.CommanderID), 10), payload)
}

func (runtime *WorldRuntime) SetMapTemplate(randomID uint32, templateID uint32) {
	if runtime.MapTemplateByRandomID == nil {
		runtime.MapTemplateByRandomID = make(map[string]uint32)
	}
	runtime.MapTemplateByRandomID[strconv.FormatUint(uint64(randomID), 10)] = templateID
}

func (runtime *WorldRuntime) MapTemplate(randomID uint32) uint32 {
	if runtime.MapTemplateByRandomID == nil {
		return 0
	}
	return runtime.MapTemplateByRandomID[strconv.FormatUint(uint64(randomID), 10)]
}

func LoadWorldMovePowerSettings() (uint32, uint32, error) {
	maxValue, err := loadWorldGamesetKeyValue(worldMovePowerMaxKey)
	if err != nil {
		return 0, 0, err
	}
	recoverInterval, err := loadWorldGamesetKeyValue(worldMovePowerRecoveryKey)
	if err != nil {
		return 0, 0, err
	}
	if maxValue == 0 {
		maxValue = defaultWorldMovePowerMax
	}
	if recoverInterval == 0 {
		recoverInterval = defaultWorldMovePowerRecovery
	}
	return maxValue, recoverInterval, nil
}

func SyncWorldRuntime(runtime *WorldRuntime, now time.Time) (bool, bool, error) {
	if runtime == nil {
		return false, false, fmt.Errorf("world runtime is nil")
	}

	clock, err := scheduler.NewCurrentRegionResetClock()
	if err != nil {
		return false, false, err
	}

	now = now.UTC()
	weekStartUnix := uint32(clock.CurrentWeeklyReset(now).Unix())
	monthKey := clock.CurrentMonthKey(now)
	changed := false
	monthReset := false

	if runtime.WeekStartUnix == 0 {
		runtime.WeekStartUnix = weekStartUnix
		changed = true
	} else if runtime.WeekStartUnix != weekStartUnix {
		runtime.WeekStartUnix = weekStartUnix
		runtime.StaminaExchangeTimes = 0
		changed = true
	}

	if runtime.MonthKey == 0 {
		runtime.MonthKey = monthKey
		changed = true
	} else if runtime.MonthKey != monthKey {
		runtime.MonthKey = monthKey
		runtime.StaminaExchangeTimes = 0
		changed = true
		monthReset = true
	}

	maxActionPower, recoverInterval, err := LoadWorldMovePowerSettings()
	if err != nil {
		return false, false, err
	}
	if applyWorldActionPowerRegeneration(runtime, uint32(now.Unix()), maxActionPower, recoverInterval) {
		changed = true
	}

	return changed, monthReset, nil
}

func applyWorldActionPowerRegeneration(runtime *WorldRuntime, nowUnix uint32, maxActionPower uint32, recoverInterval uint32) bool {
	if maxActionPower == 0 || recoverInterval == 0 {
		return false
	}

	if runtime.LastRecoverTimestamp == 0 || runtime.LastRecoverTimestamp > nowUnix {
		runtime.LastRecoverTimestamp = nowUnix
		return true
	}

	if runtime.ActionPower >= maxActionPower {
		if runtime.LastRecoverTimestamp == nowUnix {
			return false
		}
		runtime.LastRecoverTimestamp = nowUnix
		return true
	}

	elapsed := nowUnix - runtime.LastRecoverTimestamp
	ticks := elapsed / recoverInterval
	if ticks == 0 {
		return false
	}

	missing := maxActionPower - runtime.ActionPower
	gain := ticks
	if gain > missing {
		gain = missing
	}
	runtime.ActionPower += gain
	runtime.LastRecoverTimestamp += gain * recoverInterval
	if runtime.ActionPower >= maxActionPower {
		runtime.LastRecoverTimestamp = nowUnix
	}
	return true
}

func loadWorldGamesetKeyValue(key string) (uint32, error) {
	entry, err := GetConfigEntry(worldGamesetCategory, key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			entry, err = GetConfigEntry(worldGamesetCategoryLowerCase, key)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					return 0, nil
				}
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	var payload struct {
		KeyValue uint32 `json:"key_value"`
	}
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return 0, err
	}
	return payload.KeyValue, nil
}
