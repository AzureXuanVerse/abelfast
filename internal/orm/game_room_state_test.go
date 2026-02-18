package orm

import (
	"context"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

func TestLoadGameRoomStateResetsWeeklyAndMonthlyWindows(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &GameRoomScore{})
	clearTable(t, &GameRoomState{})
	clearTable(t, &Commander{})

	commanderID := uint32(9911)
	if err := CreateCommanderRoot(commanderID, commanderID, "Game Room State", 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}

	now := time.Date(2026, time.February, 18, 12, 0, 0, 0, time.UTC)
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO game_room_states (commander_id, week_start_unix, weekly_claimed, pay_coin_count, first_enter_claimed, month_key, monthly_ticket)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, int64(commanderID), int64(CurrentWeeklyResetUnix(now.AddDate(0, 0, -7))), true, int64(12), true, int64(202601), int64(333)); err != nil {
		t.Fatalf("seed game room state: %v", err)
	}

	state, err := LoadGameRoomState(commanderID, now)
	if err != nil {
		t.Fatalf("load game room state: %v", err)
	}
	if state.WeekStartUnix != CurrentWeeklyResetUnix(now) {
		t.Fatalf("expected week start reset")
	}
	if state.WeeklyClaimed {
		t.Fatalf("expected weekly flag reset")
	}
	if state.MonthKey != 202602 {
		t.Fatalf("expected month key 202602, got %d", state.MonthKey)
	}
	if state.MonthlyTicket != 0 {
		t.Fatalf("expected monthly ticket reset")
	}
	if state.PayCoinCount != 12 {
		t.Fatalf("expected pay coin count preserved")
	}
	if !state.FirstEnterClaimed {
		t.Fatalf("expected first enter flag preserved")
	}
}

func TestUpsertGameRoomScoreKeepsMax(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &GameRoomScore{})
	clearTable(t, &Commander{})

	commanderID := uint32(9912)
	if err := CreateCommanderRoot(commanderID, commanderID, "Game Room Score", 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}

	err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := UpsertGameRoomScoreTx(context.Background(), tx, commanderID, 1, 80); err != nil {
			return err
		}
		if err := UpsertGameRoomScoreTx(context.Background(), tx, commanderID, 1, 40); err != nil {
			return err
		}
		return UpsertGameRoomScoreTx(context.Background(), tx, commanderID, 1, 120)
	})
	if err != nil {
		t.Fatalf("upsert score: %v", err)
	}

	scores, err := ListGameRoomScores(commanderID)
	if err != nil {
		t.Fatalf("list scores: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("expected one score row")
	}
	if scores[0].MaxScore != 120 {
		t.Fatalf("expected max score to stay at 120, got %d", scores[0].MaxScore)
	}
}
