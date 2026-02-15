package orm

import (
	"context"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

func TestLoadWeeklyTaskProgressCreatesDefault(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &WeeklyTaskProgress{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9101, 9101, "Weekly Default", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}
	now := time.Date(2026, time.February, 10, 12, 0, 0, 0, time.UTC)
	state, err := LoadWeeklyTaskProgress(9101, now)
	if err != nil {
		t.Fatalf("load weekly state: %v", err)
	}
	if state.WeekStartUnix != CurrentWeeklyResetUnix(now) {
		t.Fatalf("expected week start %d, got %d", CurrentWeeklyResetUnix(now), state.WeekStartUnix)
	}
	if state.Pt != 0 || state.RewardLv != 0 || len(state.Tasks) != 0 {
		t.Fatalf("expected zeroed state, got %+v", state)
	}
}

func TestLoadWeeklyTaskProgressResetsOnNewWeek(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &WeeklyTaskProgress{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9102, 9102, "Weekly Reset", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}
	firstWeek := time.Date(2026, time.February, 3, 9, 0, 0, 0, time.UTC)
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := LoadWeeklyTaskProgressForUpdateTx(context.Background(), tx, 9102, firstWeek)
		if err != nil {
			return err
		}
		state.Pt = 120
		state.RewardLv = 3
		state.Tasks = []WeeklyTaskEntry{{ID: 10001, Progress: 50}}
		return SaveWeeklyTaskProgressTx(context.Background(), tx, state)
	})
	if err != nil {
		t.Fatalf("seed weekly state: %v", err)
	}

	nextWeek := time.Date(2026, time.February, 17, 10, 0, 0, 0, time.UTC)
	state, err := LoadWeeklyTaskProgress(9102, nextWeek)
	if err != nil {
		t.Fatalf("load reset weekly state: %v", err)
	}
	if state.WeekStartUnix != CurrentWeeklyResetUnix(nextWeek) {
		t.Fatalf("expected reset week start %d, got %d", CurrentWeeklyResetUnix(nextWeek), state.WeekStartUnix)
	}
	if state.Pt != 0 || state.RewardLv != 0 || len(state.Tasks) != 0 {
		t.Fatalf("expected reset state, got %+v", state)
	}
}
