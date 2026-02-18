package orm

import (
	"context"
	"encoding/json"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandNPCFeedbackState struct {
	CommanderID   uint32
	DayStartUnix  uint32
	ClaimedNPCIDs []uint32
}

func GetIslandNPCFeedbackState(commanderID uint32) (*IslandNPCFeedbackState, error) {
	var (
		commanderIDRaw int64
		dayStartRaw    int64
		claimedJSON    []byte
		state          IslandNPCFeedbackState
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, day_start_unix, claimed_npc_ids
FROM island_npc_feedback_states
WHERE commander_id = $1
`, int64(commanderID)).Scan(&commanderIDRaw, &dayStartRaw, &claimedJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(claimedJSON, &state.ClaimedNPCIDs); err != nil {
		return nil, err
	}
	if state.ClaimedNPCIDs == nil {
		state.ClaimedNPCIDs = []uint32{}
	}
	state.CommanderID = uint32(commanderIDRaw)
	state.DayStartUnix = uint32(dayStartRaw)
	return &state, nil
}

func UpsertIslandNPCFeedbackState(state *IslandNPCFeedbackState) error {
	claimed := state.ClaimedNPCIDs
	if claimed == nil {
		claimed = []uint32{}
	}
	claimedJSON, err := json.Marshal(claimed)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_npc_feedback_states (commander_id, day_start_unix, claimed_npc_ids)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id)
DO UPDATE SET
	day_start_unix = EXCLUDED.day_start_unix,
	claimed_npc_ids = EXCLUDED.claimed_npc_ids,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.DayStartUnix), claimedJSON)
	return err
}
