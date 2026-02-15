package orm

import (
	"context"
	"math"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestCommanderActivityTaskProgressUpsertAndSubmit(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderActivityTask{})
	clearTable(t, &Commander{})

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commanders (commander_id, account_id, name) VALUES ($1, $2, $3)`, int64(4501), int64(4501), "Activity Task Tester"); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := UpsertCommanderActivityTaskProgressTx(context.Background(), tx, 4501, 6001, 7001, ActivityTaskProgressModeSet, 3); err != nil {
			return err
		}
		if err := UpsertCommanderActivityTaskProgressTx(context.Background(), tx, 4501, 6001, 7001, ActivityTaskProgressModeAppend, 2); err != nil {
			return err
		}
		ok, err := TrySubmitCommanderActivityTaskTx(context.Background(), tx, 4501, 6001, 7001)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatalf("expected submit to succeed")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx failed: %v", err)
	}

	state, err := GetCommanderActivityTask(4501, 6001, 7001)
	if err != nil {
		t.Fatalf("load task state: %v", err)
	}
	if state.Progress != 5 {
		t.Fatalf("expected progress 5, got %d", state.Progress)
	}
	if !state.Submitted {
		t.Fatalf("expected submitted state")
	}
}

func TestCommanderActivityTaskSubmitIsIdempotent(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderActivityTask{})
	clearTable(t, &Commander{})

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commanders (commander_id, account_id, name) VALUES ($1, $2, $3)`, int64(4502), int64(4502), "Activity Task Tester"); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ok, err := TrySubmitCommanderActivityTaskTx(context.Background(), tx, 4502, 6001, 7001)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatalf("expected first submit to succeed")
		}
		ok, err = TrySubmitCommanderActivityTaskTx(context.Background(), tx, 4502, 6001, 7001)
		if err != nil {
			return err
		}
		if ok {
			t.Fatalf("expected duplicate submit to fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx failed: %v", err)
	}
}

func TestCommanderActivityTaskProgressAppendClampsToUint32Max(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderActivityTask{})
	clearTable(t, &Commander{})

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commanders (commander_id, account_id, name) VALUES ($1, $2, $3)`, int64(4503), int64(4503), "Activity Task Clamp Tester"); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := UpsertCommanderActivityTaskProgressTx(context.Background(), tx, 4503, 6001, 7001, ActivityTaskProgressModeSet, math.MaxUint32-1); err != nil {
			return err
		}
		return UpsertCommanderActivityTaskProgressTx(context.Background(), tx, 4503, 6001, 7001, ActivityTaskProgressModeAppend, 10)
	})
	if err != nil {
		t.Fatalf("tx failed: %v", err)
	}

	state, err := GetCommanderActivityTask(4503, 6001, 7001)
	if err != nil {
		t.Fatalf("load task state: %v", err)
	}
	if state.Progress != math.MaxUint32 {
		t.Fatalf("expected progress to clamp at max uint32, got %d", state.Progress)
	}
}
