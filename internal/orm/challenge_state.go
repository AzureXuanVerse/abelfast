package orm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type ChallengeCommanderSlot struct {
	Pos         uint32 `json:"pos"`
	CommanderID uint32 `json:"commander_id"`
}

type ChallengeModeState struct {
	CommanderID  uint32
	ActivityID   uint32
	Mode         uint32
	SeasonID     uint32
	Level        uint32
	CurrentScore uint32
	Issl         uint32

	RegularGroupID   uint32
	SubmarineGroupID uint32

	RegularShipIDs      []uint32
	SubmarineShipIDs    []uint32
	RegularCommanders   []ChallengeCommanderSlot
	SubmarineCommanders []ChallengeCommanderSlot
}

func (ChallengeModeState) TableName() string {
	return "challenge_mode_states"
}

func ListChallengeModeStates(commanderID uint32, activityID uint32) ([]ChallengeModeState, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT commander_id, activity_id, mode, season_id, level, current_score, issl,
       regular_group_id, submarine_group_id,
       regular_ship_ids, submarine_ship_ids,
       regular_commanders, submarine_commanders
FROM challenge_mode_states
WHERE commander_id = $1
  AND activity_id = $2
ORDER BY mode ASC
`, int64(commanderID), int64(activityID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]ChallengeModeState, 0)
	for rows.Next() {
		state, scanErr := scanChallengeModeState(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func GetChallengeModeStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, activityID uint32, mode uint32) (*ChallengeModeState, error) {
	row := tx.QueryRow(ctx, `
SELECT commander_id, activity_id, mode, season_id, level, current_score, issl,
       regular_group_id, submarine_group_id,
       regular_ship_ids, submarine_ship_ids,
       regular_commanders, submarine_commanders
FROM challenge_mode_states
WHERE commander_id = $1
  AND activity_id = $2
  AND mode = $3
FOR UPDATE
`, int64(commanderID), int64(activityID), int64(mode))

	state, err := scanChallengeModeState(row)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func DeleteChallengeModeStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, activityID uint32, mode uint32) error {
	_, err := tx.Exec(ctx, `
DELETE FROM challenge_mode_states
WHERE commander_id = $1
  AND activity_id = $2
  AND mode = $3
`, int64(commanderID), int64(activityID), int64(mode))
	return err
}

func UpsertChallengeModeStateTx(ctx context.Context, tx pgx.Tx, state *ChallengeModeState) error {
	if state == nil {
		return errors.New("challenge mode state is nil")
	}
	if state.RegularShipIDs == nil {
		state.RegularShipIDs = []uint32{}
	}
	if state.SubmarineShipIDs == nil {
		state.SubmarineShipIDs = []uint32{}
	}
	if state.RegularCommanders == nil {
		state.RegularCommanders = []ChallengeCommanderSlot{}
	}
	if state.SubmarineCommanders == nil {
		state.SubmarineCommanders = []ChallengeCommanderSlot{}
	}

	regularShipIDs, err := json.Marshal(state.RegularShipIDs)
	if err != nil {
		return err
	}
	submarineShipIDs, err := json.Marshal(state.SubmarineShipIDs)
	if err != nil {
		return err
	}
	regularCommanders, err := json.Marshal(state.RegularCommanders)
	if err != nil {
		return err
	}
	submarineCommanders, err := json.Marshal(state.SubmarineCommanders)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO challenge_mode_states (
  commander_id, activity_id, mode, season_id, level, current_score, issl,
  regular_group_id, submarine_group_id,
  regular_ship_ids, submarine_ship_ids,
  regular_commanders, submarine_commanders,
  created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (commander_id, activity_id, mode)
DO UPDATE SET
  season_id = EXCLUDED.season_id,
  level = EXCLUDED.level,
  current_score = EXCLUDED.current_score,
  issl = EXCLUDED.issl,
  regular_group_id = EXCLUDED.regular_group_id,
  submarine_group_id = EXCLUDED.submarine_group_id,
  regular_ship_ids = EXCLUDED.regular_ship_ids,
  submarine_ship_ids = EXCLUDED.submarine_ship_ids,
  regular_commanders = EXCLUDED.regular_commanders,
  submarine_commanders = EXCLUDED.submarine_commanders,
  updated_at = CURRENT_TIMESTAMP
`,
		int64(state.CommanderID), int64(state.ActivityID), int64(state.Mode),
		int64(state.SeasonID), int64(state.Level), int64(state.CurrentScore), int64(state.Issl),
		int64(state.RegularGroupID), int64(state.SubmarineGroupID),
		regularShipIDs, submarineShipIDs, regularCommanders, submarineCommanders,
	)
	return err
}

type challengeModeStateScanner interface {
	Scan(dest ...any) error
}

func scanChallengeModeState(scanner challengeModeStateScanner) (ChallengeModeState, error) {
	var state ChallengeModeState
	var (
		regularShipIDs      []byte
		submarineShipIDs    []byte
		regularCommanders   []byte
		submarineCommanders []byte
	)
	err := scanner.Scan(
		&state.CommanderID,
		&state.ActivityID,
		&state.Mode,
		&state.SeasonID,
		&state.Level,
		&state.CurrentScore,
		&state.Issl,
		&state.RegularGroupID,
		&state.SubmarineGroupID,
		&regularShipIDs,
		&submarineShipIDs,
		&regularCommanders,
		&submarineCommanders,
	)
	if err != nil {
		return ChallengeModeState{}, err
	}
	if err := json.Unmarshal(regularShipIDs, &state.RegularShipIDs); err != nil {
		return ChallengeModeState{}, err
	}
	if err := json.Unmarshal(submarineShipIDs, &state.SubmarineShipIDs); err != nil {
		return ChallengeModeState{}, err
	}
	if err := json.Unmarshal(regularCommanders, &state.RegularCommanders); err != nil {
		return ChallengeModeState{}, err
	}
	if err := json.Unmarshal(submarineCommanders, &state.SubmarineCommanders); err != nil {
		return ChallengeModeState{}, err
	}
	if state.RegularShipIDs == nil {
		state.RegularShipIDs = []uint32{}
	}
	if state.SubmarineShipIDs == nil {
		state.SubmarineShipIDs = []uint32{}
	}
	if state.RegularCommanders == nil {
		state.RegularCommanders = []ChallengeCommanderSlot{}
	}
	if state.SubmarineCommanders == nil {
		state.SubmarineCommanders = []ChallengeCommanderSlot{}
	}
	return state, nil
}
