package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

type CommanderIslandTradeInviteState struct {
	CommanderID         uint32
	InvitedCommanderIDs []uint32
	UpdatedAt           time.Time
}

func GetCommanderIslandTradeInviteState(commanderID uint32) (*CommanderIslandTradeInviteState, error) {
	if db.DefaultStore == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	state := &CommanderIslandTradeInviteState{}
	var invitedRaw []byte
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, invited_commander_ids, updated_at
FROM commander_island_trade_invite_states
WHERE commander_id = $1
`, int64(commanderID)).Scan(&state.CommanderID, &invitedRaw, &state.UpdatedAt)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}

	if len(invitedRaw) == 0 {
		state.InvitedCommanderIDs = []uint32{}
		return state, nil
	}
	if err := json.Unmarshal(invitedRaw, &state.InvitedCommanderIDs); err != nil {
		return nil, err
	}
	return state, nil
}

func GetOrCreateCommanderIslandTradeInviteState(commanderID uint32) (*CommanderIslandTradeInviteState, error) {
	state, err := GetCommanderIslandTradeInviteState(commanderID)
	if err == nil {
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}

	state = &CommanderIslandTradeInviteState{
		CommanderID:         commanderID,
		InvitedCommanderIDs: []uint32{},
	}
	if err := SaveCommanderIslandTradeInviteState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func SaveCommanderIslandTradeInviteState(state *CommanderIslandTradeInviteState) error {
	if db.DefaultStore == nil {
		return fmt.Errorf("database is not initialized")
	}

	invitedRaw, err := json.Marshal(state.InvitedCommanderIDs)
	if err != nil {
		return err
	}

	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commander_island_trade_invite_states (commander_id, invited_commander_ids, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (commander_id)
DO UPDATE SET
  invited_commander_ids = EXCLUDED.invited_commander_ids,
  updated_at = NOW()
`, int64(state.CommanderID), invitedRaw)
	return err
}
