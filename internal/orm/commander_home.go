package orm

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	CommanderCatteryOpClean = uint32(1)
	CommanderCatteryOpFeed  = uint32(2)
	CommanderCatteryOpPlay  = uint32(3)

	defaultCommanderHomeSlots = uint32(4)
	defaultCommanderOpFlag    = uint32(7)
)

type CommanderHome struct {
	CommanderID uint32
	Level       uint32
	Exp         uint32
	Clean       uint32
	SceneOpen   bool
}

type CommanderHomeSlot struct {
	CommanderID         uint32
	SlotID              uint32
	OpFlag              uint32
	ExpTime             uint32
	AssignedCommanderID uint32
	Style               uint32
	CacheExp            uint32
}

type commanderHomeLevelConfig struct {
	FeedLevel      []uint32 `json:"feed_level"`
	NestAppearance []uint32 `json:"nest_appearance"`
}

func CommanderCatteryOpBit(opType uint32) uint32 {
	switch opType {
	case CommanderCatteryOpClean:
		return 1
	case CommanderCatteryOpFeed:
		return 2
	case CommanderCatteryOpPlay:
		return 4
	default:
		return 0
	}
}

func CommanderHasCatteryOpFlag(opFlag uint32, opType uint32) bool {
	bit := CommanderCatteryOpBit(opType)
	if bit == 0 {
		return false
	}
	return opFlag&bit != 0
}

func CommanderClearCatteryOpFlag(opFlag uint32, opType uint32) uint32 {
	bit := CommanderCatteryOpBit(opType)
	if bit == 0 {
		return opFlag
	}
	return opFlag &^ bit
}

func loadCommanderHomeSlotCount() uint32 {
	entry, err := GetConfigEntry("ShareCfg/gameset.json", "commander_home_number")
	if err != nil {
		return defaultCommanderHomeSlots
	}
	var payload struct {
		KeyValue uint32 `json:"key_value"`
	}
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return defaultCommanderHomeSlots
	}
	if payload.KeyValue == 0 {
		return defaultCommanderHomeSlots
	}
	return payload.KeyValue
}

func loadCommanderHomeLevelConfig(level uint32) commanderHomeLevelConfig {
	entry, err := GetConfigEntry("ShareCfg/commander_home.json", strconv.FormatUint(uint64(level), 10))
	if err != nil {
		return commanderHomeLevelConfig{}
	}
	var payload commanderHomeLevelConfig
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return commanderHomeLevelConfig{}
	}
	return payload
}

func GetCommanderHomeStyleList(level uint32) []uint32 {
	cfg := loadCommanderHomeLevelConfig(level)
	if len(cfg.NestAppearance) == 0 {
		return []uint32{1}
	}
	return cfg.NestAppearance
}

func GetCommanderHomeFeedExp(level uint32) uint32 {
	cfg := loadCommanderHomeLevelConfig(level)
	if len(cfg.FeedLevel) < 2 {
		return 0
	}
	return cfg.FeedLevel[1]
}

func EnsureCommanderHome(commanderID uint32) (*CommanderHome, []CommanderHomeSlot, error) {
	home, slots, err := GetCommanderHome(commanderID)
	if err == nil {
		return home, slots, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, nil, err
	}

	ctx := context.Background()
	err = db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO commander_homes (commander_id, level, exp, clean, scene_open)
VALUES ($1, 1, 0, 0, false)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID))
		if err != nil {
			return err
		}
		slotCount := loadCommanderHomeSlotCount()
		for slotID := uint32(1); slotID <= slotCount; slotID++ {
			_, err = tx.Exec(ctx, `
INSERT INTO commander_home_slots (commander_id, slot_id, op_flag, exp_time, assigned_commander_id, style, cache_exp)
VALUES ($1, $2, $3, 0, 0, 1, 0)
ON CONFLICT (commander_id, slot_id) DO NOTHING
`, int64(commanderID), int64(slotID), int64(defaultCommanderOpFlag))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return GetCommanderHome(commanderID)
}

func GetCommanderHome(commanderID uint32) (*CommanderHome, []CommanderHomeSlot, error) {
	ctx := context.Background()
	home := CommanderHome{CommanderID: commanderID}
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT level, exp, clean, scene_open
FROM commander_homes
WHERE commander_id = $1
`, int64(commanderID))
	var level, exp, clean int64
	if err := row.Scan(&level, &exp, &clean, &home.SceneOpen); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, db.ErrNotFound
		}
		return nil, nil, err
	}
	home.Level = uint32(level)
	home.Exp = uint32(exp)
	home.Clean = uint32(clean)

	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT slot_id, op_flag, exp_time, assigned_commander_id, style, cache_exp
FROM commander_home_slots
WHERE commander_id = $1
ORDER BY slot_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	slots := make([]CommanderHomeSlot, 0)
	for rows.Next() {
		slot := CommanderHomeSlot{CommanderID: commanderID}
		var slotID, opFlag, expTime, assignedCommanderID, style, cacheExp int64
		if err := rows.Scan(&slotID, &opFlag, &expTime, &assignedCommanderID, &style, &cacheExp); err != nil {
			return nil, nil, err
		}
		slot.SlotID = uint32(slotID)
		slot.OpFlag = uint32(opFlag)
		slot.ExpTime = uint32(expTime)
		slot.AssignedCommanderID = uint32(assignedCommanderID)
		slot.Style = uint32(style)
		slot.CacheExp = uint32(cacheExp)
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return &home, slots, nil
}

func UpdateCommanderHome(home *CommanderHome) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE commander_homes
SET level = $2, exp = $3, clean = $4, scene_open = $5
WHERE commander_id = $1
`, int64(home.CommanderID), int64(home.Level), int64(home.Exp), int64(home.Clean), home.SceneOpen)
	return err
}

func UpdateCommanderHomeSlot(slot *CommanderHomeSlot) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE commander_home_slots
SET op_flag = $3,
    exp_time = $4,
    assigned_commander_id = $5,
    style = $6,
    cache_exp = $7
WHERE commander_id = $1
  AND slot_id = $2
`, int64(slot.CommanderID), int64(slot.SlotID), int64(slot.OpFlag), int64(slot.ExpTime), int64(slot.AssignedCommanderID), int64(slot.Style), int64(slot.CacheExp))
	return err
}

func ClearCommanderHomeCacheExp(commanderID uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
UPDATE commander_home_slots
SET cache_exp = 0
WHERE commander_id = $1
`, int64(commanderID))
	return err
}
