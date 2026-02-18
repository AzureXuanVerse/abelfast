package orm

import (
	"context"
	"errors"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

type GameRoomState struct {
	CommanderID       uint32
	WeekStartUnix     uint32
	WeeklyClaimed     bool
	PayCoinCount      uint32
	FirstEnterClaimed bool
	MonthKey          uint32
	MonthlyTicket     uint32
}

func (GameRoomState) TableName() string {
	return "game_room_states"
}

type GameRoomScore struct {
	CommanderID uint32
	RoomID      uint32
	MaxScore    uint32
}

func (GameRoomScore) TableName() string {
	return "game_room_scores"
}

func LoadGameRoomState(commanderID uint32, now time.Time) (*GameRoomState, error) {
	ctx := context.Background()
	var state *GameRoomState
	err := WithPGXTx(ctx, func(tx pgx.Tx) error {
		loaded, err := LoadGameRoomStateForUpdateTx(ctx, tx, commanderID, now)
		if err != nil {
			return err
		}
		state = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func LoadGameRoomStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, now time.Time) (*GameRoomState, error) {
	weekStartUnix := CurrentWeeklyResetUnix(now)
	monthKey := gameRoomMonthKey(now)

	row := tx.QueryRow(ctx, `
SELECT week_start_unix, weekly_claimed, pay_coin_count, first_enter_claimed, month_key, monthly_ticket
FROM game_room_states
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))

	var weekStart int64
	var weeklyClaimed bool
	var payCoinCount int64
	var firstEnterClaimed bool
	var storedMonthKey int64
	var monthlyTicket int64
	err := row.Scan(&weekStart, &weeklyClaimed, &payCoinCount, &firstEnterClaimed, &storedMonthKey, &monthlyTicket)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			state := &GameRoomState{
				CommanderID:   commanderID,
				WeekStartUnix: weekStartUnix,
				MonthKey:      monthKey,
			}
			if err := insertGameRoomStateTx(ctx, tx, state); err != nil {
				return nil, err
			}
			return state, nil
		}
		return nil, err
	}

	state := &GameRoomState{
		CommanderID:       commanderID,
		WeekStartUnix:     uint32(weekStart),
		WeeklyClaimed:     weeklyClaimed,
		PayCoinCount:      uint32(payCoinCount),
		FirstEnterClaimed: firstEnterClaimed,
		MonthKey:          uint32(storedMonthKey),
		MonthlyTicket:     uint32(monthlyTicket),
	}

	if state.WeekStartUnix != weekStartUnix {
		state.WeekStartUnix = weekStartUnix
		state.WeeklyClaimed = false
	}
	if state.MonthKey != monthKey {
		state.MonthKey = monthKey
		state.MonthlyTicket = 0
	}
	if state.WeekStartUnix != uint32(weekStart) || state.WeeklyClaimed != weeklyClaimed || state.MonthKey != uint32(storedMonthKey) || state.MonthlyTicket != uint32(monthlyTicket) {
		if err := SaveGameRoomStateTx(ctx, tx, state); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func SaveGameRoomStateTx(ctx context.Context, tx pgx.Tx, state *GameRoomState) error {
	_, err := tx.Exec(ctx, `
UPDATE game_room_states
SET week_start_unix = $2,
	weekly_claimed = $3,
	pay_coin_count = $4,
	first_enter_claimed = $5,
	month_key = $6,
	monthly_ticket = $7,
	updated_at = CURRENT_TIMESTAMP
WHERE commander_id = $1
`, int64(state.CommanderID), int64(state.WeekStartUnix), state.WeeklyClaimed, int64(state.PayCoinCount), state.FirstEnterClaimed, int64(state.MonthKey), int64(state.MonthlyTicket))
	return err
}

func ListGameRoomScores(commanderID uint32) ([]GameRoomScore, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT commander_id, room_id, max_score
FROM game_room_scores
WHERE commander_id = $1
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scores := make([]GameRoomScore, 0)
	for rows.Next() {
		var commanderIDValue int64
		var roomID int64
		var maxScore int64
		if err := rows.Scan(&commanderIDValue, &roomID, &maxScore); err != nil {
			return nil, err
		}
		scores = append(scores, GameRoomScore{
			CommanderID: uint32(commanderIDValue),
			RoomID:      uint32(roomID),
			MaxScore:    uint32(maxScore),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return scores, nil
}

func UpsertGameRoomScoreTx(ctx context.Context, tx pgx.Tx, commanderID uint32, roomID uint32, score uint32) error {
	_, err := tx.Exec(ctx, `
INSERT INTO game_room_scores (commander_id, room_id, max_score)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, room_id)
DO UPDATE SET max_score = GREATEST(game_room_scores.max_score, EXCLUDED.max_score),
	updated_at = CURRENT_TIMESTAMP
`, int64(commanderID), int64(roomID), int64(score))
	return err
}

func gameRoomMonthKey(now time.Time) uint32 {
	utc := now.UTC()
	return uint32(utc.Year()*100 + int(utc.Month()))
}

func insertGameRoomStateTx(ctx context.Context, tx pgx.Tx, state *GameRoomState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO game_room_states (commander_id, week_start_unix, weekly_claimed, pay_coin_count, first_enter_claimed, month_key, monthly_ticket)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, int64(state.CommanderID), int64(state.WeekStartUnix), state.WeeklyClaimed, int64(state.PayCoinCount), state.FirstEnterClaimed, int64(state.MonthKey), int64(state.MonthlyTicket))
	return err
}
