package orm

import (
	"context"
	"testing"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

func TestIslandAchievementStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandAchievementState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9801, 9801, "achievement", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := GetIslandAchievementStateForUpdateTx(context.Background(), tx, 9801)
		if err != nil {
			return err
		}
		state.SetProgress(8, 2, 9)
		state.SetProgress(7, 3, 4)
		state.SetProgress(8, 2, 11)
		state.FinishList = []uint32{3002, 3001}
		return SaveIslandAchievementStateTx(context.Background(), tx, state)
	})
	if err != nil {
		t.Fatalf("save achievement state: %v", err)
	}

	state, err := GetIslandAchievementState(9801)
	if err != nil {
		t.Fatalf("load achievement state: %v", err)
	}
	if len(state.ProgressEntries) != 2 {
		t.Fatalf("expected two progress entries, got %+v", state.ProgressEntries)
	}
	if state.ProgressEntries[0].EventType != 7 || state.ProgressEntries[0].EventArg != 3 || state.ProgressEntries[0].Value != 4 {
		t.Fatalf("unexpected first progress entry: %+v", state.ProgressEntries[0])
	}
	if state.ProgressEntries[1].EventType != 8 || state.ProgressEntries[1].EventArg != 2 || state.ProgressEntries[1].Value != 11 {
		t.Fatalf("unexpected second progress entry: %+v", state.ProgressEntries[1])
	}
	if len(state.FinishList) != 2 || state.FinishList[0] != 3001 || state.FinishList[1] != 3002 {
		t.Fatalf("expected sorted finish list, got %+v", state.FinishList)
	}
}

func TestIslandAchievementStateForUpdateCreatesDefault(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandAchievementState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9802, 9802, "achievement-default", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := GetIslandAchievementStateForUpdateTx(context.Background(), tx, 9802)
		if err != nil {
			return err
		}
		if state.CommanderID != 9802 || len(state.ProgressEntries) != 0 || len(state.FinishList) != 0 {
			t.Fatalf("unexpected default state: %+v", state)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("load achievement state for update: %v", err)
	}
}
