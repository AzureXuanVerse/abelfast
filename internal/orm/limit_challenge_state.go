package orm

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type LimitChallengeState struct {
	CommanderID uint32
	MonthBucket uint32
	BestTimes   map[uint32]uint32
	Awarded     map[uint32]bool
	PassIDs     []uint32
}

func (LimitChallengeState) TableName() string {
	return "limit_challenge_states"
}

func CurrentLimitChallengeMonthBucket(now time.Time) uint32 {
	utc := now.UTC()
	return uint32(utc.Year())*100 + uint32(utc.Month())
}

func LoadLimitChallengeState(commanderID uint32, now time.Time) (*LimitChallengeState, error) {
	ctx := context.Background()
	var state *LimitChallengeState
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		loaded, loadErr := LoadLimitChallengeStateForUpdateTx(ctx, tx, commanderID, now)
		if loadErr != nil {
			return loadErr
		}
		state = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func LoadLimitChallengeStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, now time.Time) (*LimitChallengeState, error) {
	currentMonth := CurrentLimitChallengeMonthBucket(now)
	row := tx.QueryRow(ctx, `
SELECT commander_id, month_bucket, best_times, awarded, pass_ids
FROM limit_challenge_states
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))

	state, err := scanLimitChallengeState(row)
	if err != nil {
		if db.IsNotFound(db.MapNotFound(err)) {
			created := &LimitChallengeState{
				CommanderID: commanderID,
				MonthBucket: currentMonth,
				BestTimes:   map[uint32]uint32{},
				Awarded:     map[uint32]bool{},
				PassIDs:     []uint32{},
			}
			if saveErr := SaveLimitChallengeStateTx(ctx, tx, created); saveErr != nil {
				return nil, saveErr
			}
			return created, nil
		}
		return nil, err
	}
	if state.MonthBucket != currentMonth {
		state.MonthBucket = currentMonth
		state.BestTimes = map[uint32]uint32{}
		state.Awarded = map[uint32]bool{}
		state.PassIDs = []uint32{}
		if err := SaveLimitChallengeStateTx(ctx, tx, &state); err != nil {
			return nil, err
		}
	}
	return &state, nil
}

func SaveLimitChallengeStateTx(ctx context.Context, tx pgx.Tx, state *LimitChallengeState) error {
	if state.BestTimes == nil {
		state.BestTimes = map[uint32]uint32{}
	}
	if state.Awarded == nil {
		state.Awarded = map[uint32]bool{}
	}
	if state.PassIDs == nil {
		state.PassIDs = []uint32{}
	}

	bestTimes, err := marshalUint32Map(state.BestTimes)
	if err != nil {
		return err
	}
	awarded, err := marshalBoolMap(state.Awarded)
	if err != nil {
		return err
	}
	passIDs, err := json.Marshal(sortedUint32Slice(state.PassIDs))
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO limit_challenge_states (
  commander_id, month_bucket, best_times, awarded, pass_ids, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (commander_id)
DO UPDATE SET
  month_bucket = EXCLUDED.month_bucket,
  best_times = EXCLUDED.best_times,
  awarded = EXCLUDED.awarded,
  pass_ids = EXCLUDED.pass_ids,
  updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.MonthBucket), bestTimes, awarded, passIDs)
	return err
}

func MarkLimitChallengePass(state *LimitChallengeState, challengeID uint32, totalTime uint32) {
	if state.BestTimes == nil {
		state.BestTimes = map[uint32]uint32{}
	}
	if state.Awarded == nil {
		state.Awarded = map[uint32]bool{}
	}
	if state.PassIDs == nil {
		state.PassIDs = []uint32{}
	}

	best, ok := state.BestTimes[challengeID]
	if !ok || (totalTime > 0 && totalTime < best) || best == 0 {
		state.BestTimes[challengeID] = totalTime
	}
	for _, id := range state.PassIDs {
		if id == challengeID {
			return
		}
	}
	state.PassIDs = append(state.PassIDs, challengeID)
	state.PassIDs = sortedUint32Slice(state.PassIDs)
}

type limitChallengeStateScanner interface {
	Scan(dest ...any) error
}

func scanLimitChallengeState(scanner limitChallengeStateScanner) (LimitChallengeState, error) {
	var state LimitChallengeState
	var (
		bestTimesJSON []byte
		awardedJSON   []byte
		passIDsJSON   []byte
	)
	err := scanner.Scan(&state.CommanderID, &state.MonthBucket, &bestTimesJSON, &awardedJSON, &passIDsJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return LimitChallengeState{}, err
	}

	bestTimes, err := unmarshalUint32Map(bestTimesJSON)
	if err != nil {
		return LimitChallengeState{}, err
	}
	awarded, err := unmarshalBoolMap(awardedJSON)
	if err != nil {
		return LimitChallengeState{}, err
	}
	var passIDs []uint32
	if len(passIDsJSON) > 0 {
		if err := json.Unmarshal(passIDsJSON, &passIDs); err != nil {
			return LimitChallengeState{}, err
		}
	}
	state.BestTimes = bestTimes
	state.Awarded = awarded
	state.PassIDs = sortedUint32Slice(passIDs)
	return state, nil
}

func marshalUint32Map(value map[uint32]uint32) ([]byte, error) {
	payload := map[string]uint32{}
	for key, number := range value {
		payload[strconv.FormatUint(uint64(key), 10)] = number
	}
	return json.Marshal(payload)
}

func unmarshalUint32Map(data []byte) (map[uint32]uint32, error) {
	if len(data) == 0 {
		return map[uint32]uint32{}, nil
	}
	raw := map[string]uint32{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	result := make(map[uint32]uint32, len(raw))
	for key, value := range raw {
		parsed, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			continue
		}
		result[uint32(parsed)] = value
	}
	return result, nil
}

func marshalBoolMap(value map[uint32]bool) ([]byte, error) {
	payload := map[string]bool{}
	for key, flag := range value {
		payload[strconv.FormatUint(uint64(key), 10)] = flag
	}
	return json.Marshal(payload)
}

func unmarshalBoolMap(data []byte) (map[uint32]bool, error) {
	if len(data) == 0 {
		return map[uint32]bool{}, nil
	}
	raw := map[string]bool{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	result := make(map[uint32]bool, len(raw))
	for key, value := range raw {
		parsed, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			continue
		}
		result[uint32(parsed)] = value
	}
	return result, nil
}

func sortedUint32Slice(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	copyValues := make([]uint32, len(values))
	copy(copyValues, values)
	sort.Slice(copyValues, func(i int, j int) bool {
		return copyValues[i] < copyValues[j]
	})
	return copyValues
}
