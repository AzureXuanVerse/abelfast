package orm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/ggmolly/belfast/internal/db"
)

var coloringStateTestCommanderSeed uint32 = 9900

func nextColoringStateCommanderID() uint32 {
	return atomic.AddUint32(&coloringStateTestCommanderSeed, 1)
}

func TestCommanderColoringStateCRUD(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderColoringState{})
	commanderID := nextColoringStateCommanderID()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), fmt.Sprintf("DELETE FROM %s WHERE commander_id = $1", QualifiedTable("commanders")), int64(commanderID)); err != nil {
		t.Fatalf("delete commander: %v", err)
	}
	if err := CreateCommanderRoot(commanderID, commanderID, "Coloring ORM Tester", 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}

	state, err := GetOrCreateCommanderColoringState(commanderID, 4890, 1700000000)
	if err != nil {
		t.Fatalf("get or create coloring state: %v", err)
	}
	if state.StartTime != 1700000000 {
		t.Fatalf("expected start time seed, got %d", state.StartTime)
	}

	state.Cells = []ColoringCellState{{PageID: 92, Row: 1, Column: 2, Color: 3}}
	state.Awards = []ColoringAwardState{{PageID: 92, Drops: []ColoringDropState{{Type: 2, ID: 20001, Number: 1}}}}
	if err := SaveCommanderColoringState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := GetCommanderColoringState(commanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(loaded.Cells) != 1 || loaded.Cells[0].PageID != 92 || loaded.Cells[0].Color != 3 {
		t.Fatalf("unexpected cells: %+v", loaded.Cells)
	}
	if len(loaded.Awards) != 1 || len(loaded.Awards[0].Drops) != 1 || loaded.Awards[0].Drops[0].ID != 20001 {
		t.Fatalf("unexpected awards: %+v", loaded.Awards)
	}

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), fmt.Sprintf("DELETE FROM %s WHERE commander_id = $1 AND activity_id = $2", QualifiedTable("commander_coloring_states")), int64(commanderID), int64(4890)); err != nil {
		t.Fatalf("delete coloring state: %v", err)
	}
	_, err = GetCommanderColoringState(commanderID, 4890)
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}
