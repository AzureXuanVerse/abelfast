package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

type CommanderIslandSocialState struct {
	CommanderID          uint32
	InviteCode           string
	InviteCodeRefreshDay uint32
	InvitedCommanderIDs  []uint32
	GiftCount            uint32
	GiftTimestamp        uint32
	GiftVisitors         []uint32
	UpdatedAt            time.Time
}

func GetCommanderIslandSocialState(commanderID uint32) (*CommanderIslandSocialState, error) {
	if db.DefaultStore == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	ctx := context.Background()
	state := &CommanderIslandSocialState{}
	var invitedRaw []byte
	var visitorsRaw []byte
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, invite_code, invite_code_refresh_day, invited_commander_ids, gift_count, gift_timestamp, gift_visitors, updated_at
FROM commander_island_social_states
WHERE commander_id = $1
`, int64(commanderID)).Scan(
		&state.CommanderID,
		&state.InviteCode,
		&state.InviteCodeRefreshDay,
		&invitedRaw,
		&state.GiftCount,
		&state.GiftTimestamp,
		&visitorsRaw,
		&state.UpdatedAt,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if len(invitedRaw) == 0 {
		state.InvitedCommanderIDs = []uint32{}
	} else if err := json.Unmarshal(invitedRaw, &state.InvitedCommanderIDs); err != nil {
		return nil, err
	}
	if len(visitorsRaw) == 0 {
		state.GiftVisitors = []uint32{}
	} else if err := json.Unmarshal(visitorsRaw, &state.GiftVisitors); err != nil {
		return nil, err
	}
	return state, nil
}

func GetOrCreateCommanderIslandSocialState(commanderID uint32) (*CommanderIslandSocialState, error) {
	state, err := GetCommanderIslandSocialState(commanderID)
	if err == nil {
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}

	state = &CommanderIslandSocialState{
		CommanderID:         commanderID,
		InvitedCommanderIDs: []uint32{},
		GiftVisitors:        []uint32{},
	}
	if err := SaveCommanderIslandSocialState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func SaveCommanderIslandSocialState(state *CommanderIslandSocialState) error {
	if db.DefaultStore == nil {
		return fmt.Errorf("database is not initialized")
	}

	invitedRaw, err := json.Marshal(state.InvitedCommanderIDs)
	if err != nil {
		return err
	}
	visitorsRaw, err := json.Marshal(state.GiftVisitors)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO commander_island_social_states (commander_id, invite_code, invite_code_refresh_day, invited_commander_ids, gift_count, gift_timestamp, gift_visitors, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (commander_id)
DO UPDATE SET
  invite_code = EXCLUDED.invite_code,
  invite_code_refresh_day = EXCLUDED.invite_code_refresh_day,
  invited_commander_ids = EXCLUDED.invited_commander_ids,
  gift_count = EXCLUDED.gift_count,
  gift_timestamp = EXCLUDED.gift_timestamp,
  gift_visitors = EXCLUDED.gift_visitors,
  updated_at = NOW()
`, int64(state.CommanderID), state.InviteCode, int64(state.InviteCodeRefreshDay), invitedRaw, int64(state.GiftCount), int64(state.GiftTimestamp), visitorsRaw)
	return err
}

func GetCommanderIDByIslandInviteCode(code string) (uint32, error) {
	if db.DefaultStore == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	ctx := context.Background()
	var commanderID uint32
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id
FROM commander_island_social_states
WHERE invite_code = $1
`, code).Scan(&commanderID)
	err = db.MapNotFound(err)
	if err != nil {
		return 0, err
	}
	return commanderID, nil
}

func IsIslandInviteCodeTaken(code string, excludeCommanderID uint32) (bool, error) {
	if code == "" {
		return false, nil
	}
	if db.DefaultStore == nil {
		return false, fmt.Errorf("database is not initialized")
	}
	ctx := context.Background()
	var count int64
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM commander_island_social_states
WHERE invite_code = $1 AND commander_id <> $2
`, code, int64(excludeCommanderID)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func BatchGetCommanderIslandSocialStates(commanderIDs []uint32) (map[uint32]*CommanderIslandSocialState, error) {
	result := make(map[uint32]*CommanderIslandSocialState, len(commanderIDs))
	if len(commanderIDs) == 0 {
		return result, nil
	}
	if db.DefaultStore == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT commander_id, invite_code, invite_code_refresh_day, invited_commander_ids, gift_count, gift_timestamp, gift_visitors, updated_at
FROM commander_island_social_states
WHERE commander_id = ANY($1)
`, commanderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		state := &CommanderIslandSocialState{}
		var invitedRaw []byte
		var visitorsRaw []byte
		if err := rows.Scan(
			&state.CommanderID,
			&state.InviteCode,
			&state.InviteCodeRefreshDay,
			&invitedRaw,
			&state.GiftCount,
			&state.GiftTimestamp,
			&visitorsRaw,
			&state.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(invitedRaw) == 0 {
			state.InvitedCommanderIDs = []uint32{}
		} else if err := json.Unmarshal(invitedRaw, &state.InvitedCommanderIDs); err != nil {
			return nil, err
		}
		if len(visitorsRaw) == 0 {
			state.GiftVisitors = []uint32{}
		} else if err := json.Unmarshal(visitorsRaw, &state.GiftVisitors); err != nil {
			return nil, err
		}
		result[state.CommanderID] = state
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
