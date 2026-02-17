package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandFishWeightState struct {
	FishID    uint32 `json:"fish_id"`
	MinWeight uint32 `json:"min_weight"`
	MaxWeight uint32 `json:"max_weight"`
	GoldState uint32 `json:"gold_state"`
}

type IslandFishingState struct {
	CommanderID uint32
	BaitID      uint32
	FishRod     uint32
	FishWeights []IslandFishWeightState
}

func (IslandFishingState) TableName() string {
	return "island_fishing_states"
}

func GetIslandFishingState(commanderID uint32) (*IslandFishingState, error) {
	var (
		commanderIDRaw int64
		baitIDRaw      int64
		fishRodRaw     int64
		weightsJSON    []byte
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, bait_id, fish_rod, fish_weights
FROM island_fishing_states
WHERE commander_id = $1
`, int64(commanderID)).Scan(&commanderIDRaw, &baitIDRaw, &fishRodRaw, &weightsJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	state := &IslandFishingState{
		CommanderID: uint32(commanderIDRaw),
		BaitID:      uint32(baitIDRaw),
		FishRod:     uint32(fishRodRaw),
		FishWeights: []IslandFishWeightState{},
	}
	if len(weightsJSON) > 0 {
		if err := json.Unmarshal(weightsJSON, &state.FishWeights); err != nil {
			return nil, err
		}
	}
	if state.FishWeights == nil {
		state.FishWeights = []IslandFishWeightState{}
	}
	return state, nil
}

func GetIslandFishingStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandFishingState, error) {
	var (
		commanderIDRaw int64
		baitIDRaw      int64
		fishRodRaw     int64
		weightsJSON    []byte
	)
	err := tx.QueryRow(ctx, `
SELECT commander_id, bait_id, fish_rod, fish_weights
FROM island_fishing_states
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID)).Scan(&commanderIDRaw, &baitIDRaw, &fishRodRaw, &weightsJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	state := &IslandFishingState{
		CommanderID: uint32(commanderIDRaw),
		BaitID:      uint32(baitIDRaw),
		FishRod:     uint32(fishRodRaw),
		FishWeights: []IslandFishWeightState{},
	}
	if len(weightsJSON) > 0 {
		if err := json.Unmarshal(weightsJSON, &state.FishWeights); err != nil {
			return nil, err
		}
	}
	if state.FishWeights == nil {
		state.FishWeights = []IslandFishWeightState{}
	}
	return state, nil
}

func UpsertIslandFishingStateTx(ctx context.Context, tx pgx.Tx, state *IslandFishingState) error {
	weights := state.FishWeights
	if weights == nil {
		weights = []IslandFishWeightState{}
	}
	weightsJSON, err := json.Marshal(weights)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_fishing_states (commander_id, bait_id, fish_rod, fish_weights)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id)
DO UPDATE SET
	bait_id = EXCLUDED.bait_id,
	fish_rod = EXCLUDED.fish_rod,
	fish_weights = EXCLUDED.fish_weights,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.BaitID), int64(state.FishRod), weightsJSON)
	return err
}
