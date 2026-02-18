package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

const atelierStateCategory = "Runtime/atelier_state"

type AtelierBuffSlotState struct {
	Pos     uint32 `json:"pos"`
	ItemID  uint32 `json:"item_id"`
	ItemNum uint32 `json:"item_num"`
}

type AtelierState struct {
	CommanderID uint32                          `json:"commander_id"`
	ActID       uint32                          `json:"act_id"`
	Items       map[uint32]uint32               `json:"items"`
	RecipeUses  map[uint32]uint32               `json:"recipe_uses"`
	Slots       map[uint32]AtelierBuffSlotState `json:"slots"`
}

func GetOrCreateAtelierState(commanderID uint32, actID uint32) (*AtelierState, error) {
	entry, err := GetConfigEntry(atelierStateCategory, atelierStateKey(commanderID, actID))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := defaultAtelierState(commanderID, actID)
		if err := SaveAtelierState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	state := &AtelierState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	normalizeAtelierState(state, commanderID, actID)
	return state, nil
}

func GetOrCreateAtelierStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, actID uint32) (*AtelierState, error) {
	state, err := getAtelierStateTx(ctx, tx, commanderID, actID)
	if err == nil {
		normalizeAtelierState(state, commanderID, actID)
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}
	state = defaultAtelierState(commanderID, actID)
	if err := SaveAtelierStateTx(ctx, tx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func SaveAtelierState(state *AtelierState) error {
	normalizeAtelierState(state, state.CommanderID, state.ActID)
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(atelierStateCategory, atelierStateKey(state.CommanderID, state.ActID), payload)
}

func SaveAtelierStateTx(ctx context.Context, tx pgx.Tx, state *AtelierState) error {
	normalizeAtelierState(state, state.CommanderID, state.ActID)
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO config_entries (category, key, data)
VALUES ($1, $2, $3)
ON CONFLICT (category, key)
DO UPDATE SET data = EXCLUDED.data
`, atelierStateCategory, atelierStateKey(state.CommanderID, state.ActID), payload)
	return err
}

func LockAtelierStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, actID uint32) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1, $2)", int32(commanderID), int32(actID))
	return err
}

func getAtelierStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, actID uint32) (*AtelierState, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `
SELECT data
FROM config_entries
WHERE category = $1 AND key = $2
`, atelierStateCategory, atelierStateKey(commanderID, actID)).Scan(&raw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	state := &AtelierState{}
	if err := json.Unmarshal(raw, state); err != nil {
		return nil, err
	}
	return state, nil
}

func defaultAtelierState(commanderID uint32, actID uint32) *AtelierState {
	return &AtelierState{
		CommanderID: commanderID,
		ActID:       actID,
		Items:       map[uint32]uint32{},
		RecipeUses:  map[uint32]uint32{},
		Slots:       defaultAtelierSlots(),
	}
}

func defaultAtelierSlots() map[uint32]AtelierBuffSlotState {
	slots := make(map[uint32]AtelierBuffSlotState, 5)
	for pos := uint32(1); pos <= 5; pos++ {
		slots[pos] = AtelierBuffSlotState{Pos: pos}
	}
	return slots
}

func normalizeAtelierState(state *AtelierState, commanderID uint32, actID uint32) {
	state.CommanderID = commanderID
	state.ActID = actID
	if state.Items == nil {
		state.Items = map[uint32]uint32{}
	}
	if state.RecipeUses == nil {
		state.RecipeUses = map[uint32]uint32{}
	}
	if state.Slots == nil {
		state.Slots = defaultAtelierSlots()
	}
	for pos := uint32(1); pos <= 5; pos++ {
		slot, ok := state.Slots[pos]
		if !ok {
			state.Slots[pos] = AtelierBuffSlotState{Pos: pos}
			continue
		}
		slot.Pos = pos
		if slot.ItemID == 0 {
			slot.ItemNum = 0
		}
		state.Slots[pos] = slot
	}
	for pos := range state.Slots {
		if pos < 1 || pos > 5 {
			delete(state.Slots, pos)
		}
	}
	for itemID, count := range state.Items {
		if itemID == 0 || count == 0 {
			delete(state.Items, itemID)
		}
	}
	for recipeID, count := range state.RecipeUses {
		if recipeID == 0 || count == 0 {
			delete(state.RecipeUses, recipeID)
		}
	}
}

func atelierStateKey(commanderID uint32, actID uint32) string {
	return fmt.Sprintf("%s:%s", strconv.FormatUint(uint64(commanderID), 10), strconv.FormatUint(uint64(actID), 10))
}
