package orm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	commanderGameSetCategory         = "ShareCfg/gameset.json"
	commanderAbilityGroupCategory    = "ShareCfg/commander_ability_group.json"
	commanderAbilityTemplateCategory = "ShareCfg/commander_ability_template.json"
)

type CommanderPacketState struct {
	OwnerCommanderID  uint32
	CommanderID       uint32
	Level             uint32
	Name              string
	IsLocked          bool
	UsedPt            uint32
	AbilityIDs        []uint32
	AbilityOriginIDs  []uint32
	PendingAbilityIDs []uint32
	AbilityResetAt    time.Time
	RenameCooldownAt  time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CommanderPacketState) TableName() string {
	return "commander_packet_states"
}

type CommanderPrefabSlot struct {
	Pos         uint32 `json:"pos"`
	CommanderID uint32 `json:"commander_id"`
}

type CommanderPrefabFleet struct {
	OwnerCommanderID uint32
	PrefabID         uint32
	Name             string
	RenameCooldownAt time.Time
	CommanderSlots   []CommanderPrefabSlot

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CommanderPrefabFleet) TableName() string {
	return "commander_prefab_fleets"
}

type CommanderAbilityTemplate struct {
	ID      uint32 `json:"id"`
	GroupID uint32 `json:"group_id"`
	Cost    uint32 `json:"cost"`
	Worth   uint32 `json:"worth"`
	Next    uint32 `json:"next"`
}

type CommanderAbilityGroup struct {
	ID          uint32   `json:"id"`
	AbilityList []uint32 `json:"ability_list"`
}

type commanderGameSetEntry struct {
	KeyValue    uint32          `json:"key_value"`
	Description json.RawMessage `json:"description"`
}

type CommanderGameSet struct {
	RenameCooldownSeconds       uint32
	AbilityResetCooldownSeconds uint32
	SkillResetCosts             []uint32
}

func GetCommanderPacketState(ownerCommanderID uint32, commanderID uint32) (*CommanderPacketState, error) {
	entry := &CommanderPacketState{}
	var (
		abilityRaw       []byte
		abilityOriginRaw []byte
		pendingRaw       []byte
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT owner_commander_id, commander_id, level, name, is_locked, used_pt,
       ability_ids, ability_origin_ids, pending_ability_ids,
       ability_reset_at, rename_cooldown_at, created_at, updated_at
FROM commander_packet_states
WHERE owner_commander_id = $1
  AND commander_id = $2
`, int64(ownerCommanderID), int64(commanderID)).Scan(
		&entry.OwnerCommanderID,
		&entry.CommanderID,
		&entry.Level,
		&entry.Name,
		&entry.IsLocked,
		&entry.UsedPt,
		&abilityRaw,
		&abilityOriginRaw,
		&pendingRaw,
		&entry.AbilityResetAt,
		&entry.RenameCooldownAt,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(abilityRaw, &entry.AbilityIDs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(abilityOriginRaw, &entry.AbilityOriginIDs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(pendingRaw, &entry.PendingAbilityIDs); err != nil {
		return nil, err
	}
	if entry.AbilityIDs == nil {
		entry.AbilityIDs = []uint32{}
	}
	if entry.AbilityOriginIDs == nil {
		entry.AbilityOriginIDs = []uint32{}
	}
	if entry.PendingAbilityIDs == nil {
		entry.PendingAbilityIDs = []uint32{}
	}
	return entry, nil
}

func GetOrCreateCommanderPacketState(ownerCommanderID uint32, commanderID uint32) (*CommanderPacketState, error) {
	state, err := GetCommanderPacketState(ownerCommanderID, commanderID)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	state = &CommanderPacketState{
		OwnerCommanderID:  ownerCommanderID,
		CommanderID:       commanderID,
		Level:             1,
		Name:              "Commander",
		AbilityIDs:        []uint32{},
		AbilityOriginIDs:  []uint32{},
		PendingAbilityIDs: []uint32{},
		AbilityResetAt:    time.Unix(0, 0).UTC(),
		RenameCooldownAt:  time.Unix(0, 0).UTC(),
	}
	if err := SaveCommanderPacketState(state); err != nil {
		return nil, err
	}
	return GetCommanderPacketState(ownerCommanderID, commanderID)
}

func SaveCommanderPacketState(entry *CommanderPacketState) error {
	if entry == nil {
		return errors.New("commander packet state is nil")
	}
	if entry.AbilityIDs == nil {
		entry.AbilityIDs = []uint32{}
	}
	if entry.AbilityOriginIDs == nil {
		entry.AbilityOriginIDs = []uint32{}
	}
	if entry.PendingAbilityIDs == nil {
		entry.PendingAbilityIDs = []uint32{}
	}
	abilityRaw, err := json.Marshal(entry.AbilityIDs)
	if err != nil {
		return err
	}
	abilityOriginRaw, err := json.Marshal(entry.AbilityOriginIDs)
	if err != nil {
		return err
	}
	pendingRaw, err := json.Marshal(entry.PendingAbilityIDs)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commander_packet_states (
  owner_commander_id, commander_id, level, name, is_locked, used_pt,
  ability_ids, ability_origin_ids, pending_ability_ids,
  ability_reset_at, rename_cooldown_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
ON CONFLICT (owner_commander_id, commander_id)
DO UPDATE SET
  level = EXCLUDED.level,
  name = EXCLUDED.name,
  is_locked = EXCLUDED.is_locked,
  used_pt = EXCLUDED.used_pt,
  ability_ids = EXCLUDED.ability_ids,
  ability_origin_ids = EXCLUDED.ability_origin_ids,
  pending_ability_ids = EXCLUDED.pending_ability_ids,
  ability_reset_at = EXCLUDED.ability_reset_at,
  rename_cooldown_at = EXCLUDED.rename_cooldown_at,
  updated_at = NOW()
`, int64(entry.OwnerCommanderID), int64(entry.CommanderID), int64(entry.Level), entry.Name, entry.IsLocked, int64(entry.UsedPt), abilityRaw, abilityOriginRaw, pendingRaw, entry.AbilityResetAt, entry.RenameCooldownAt)
	return err
}

func GetCommanderPrefabFleet(ownerCommanderID uint32, prefabID uint32) (*CommanderPrefabFleet, error) {
	entry := &CommanderPrefabFleet{}
	var slotsRaw []byte
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT owner_commander_id, prefab_id, name, rename_cooldown_at, commander_slots, created_at, updated_at
FROM commander_prefab_fleets
WHERE owner_commander_id = $1
  AND prefab_id = $2
`, int64(ownerCommanderID), int64(prefabID)).Scan(
		&entry.OwnerCommanderID,
		&entry.PrefabID,
		&entry.Name,
		&entry.RenameCooldownAt,
		&slotsRaw,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(slotsRaw, &entry.CommanderSlots); err != nil {
		return nil, err
	}
	if entry.CommanderSlots == nil {
		entry.CommanderSlots = []CommanderPrefabSlot{}
	}
	return entry, nil
}

func SaveCommanderPrefabFleet(entry *CommanderPrefabFleet) error {
	if entry == nil {
		return errors.New("commander prefab fleet is nil")
	}
	if entry.CommanderSlots == nil {
		entry.CommanderSlots = []CommanderPrefabSlot{}
	}
	slotsRaw, err := json.Marshal(entry.CommanderSlots)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commander_prefab_fleets (
  owner_commander_id, prefab_id, name, rename_cooldown_at, commander_slots, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (owner_commander_id, prefab_id)
DO UPDATE SET
  name = EXCLUDED.name,
  rename_cooldown_at = EXCLUDED.rename_cooldown_at,
  commander_slots = EXCLUDED.commander_slots,
  updated_at = NOW()
`, int64(entry.OwnerCommanderID), int64(entry.PrefabID), entry.Name, entry.RenameCooldownAt, slotsRaw)
	return err
}

func GetCommanderAbilityTemplate(abilityID uint32) (*CommanderAbilityTemplate, error) {
	entry, err := GetConfigEntry(commanderAbilityTemplateCategory, fmt.Sprintf("%d", abilityID))
	if err != nil {
		return nil, err
	}
	parsed := &CommanderAbilityTemplate{}
	if err := json.Unmarshal(entry.Data, parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func ListCommanderAbilityGroups() ([]CommanderAbilityGroup, error) {
	entries, err := ListConfigEntries(commanderAbilityGroupCategory)
	if err != nil {
		return nil, err
	}
	groups := make([]CommanderAbilityGroup, 0, len(entries))
	for _, entry := range entries {
		parsed := CommanderAbilityGroup{}
		if err := json.Unmarshal(entry.Data, &parsed); err != nil {
			return nil, err
		}
		groups = append(groups, parsed)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})
	return groups, nil
}

func LoadCommanderGameSet() (*CommanderGameSet, error) {
	renameEntry, err := loadCommanderGameSetEntry("commander_rename_coldtime")
	if err != nil {
		return nil, err
	}
	abilityResetCooldownEntry, err := loadCommanderGameSetEntry("commander_ability_reset_coldtime")
	if err != nil {
		return nil, err
	}
	abilityResetCostEntry, err := loadCommanderGameSetEntry("commander_skill_reset_cost")
	if err != nil {
		return nil, err
	}

	var descriptionRows [][]uint32
	if err := json.Unmarshal(abilityResetCostEntry.Description, &descriptionRows); err != nil {
		return nil, err
	}
	costs := []uint32{}
	if len(descriptionRows) > 0 {
		costs = descriptionRows[0]
	}

	return &CommanderGameSet{
		RenameCooldownSeconds:       renameEntry.KeyValue,
		AbilityResetCooldownSeconds: abilityResetCooldownEntry.KeyValue,
		SkillResetCosts:             costs,
	}, nil
}

func loadCommanderGameSetEntry(key string) (*commanderGameSetEntry, error) {
	entry, err := GetConfigEntry(commanderGameSetCategory, key)
	if err != nil {
		return nil, err
	}
	parsed := &commanderGameSetEntry{}
	if err := json.Unmarshal(entry.Data, parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
