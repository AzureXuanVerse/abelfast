package orm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestIslandHandPlantUpsertResetAndRead(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandHandPlant{})
	clearTable(t, &Commander{})

	const commanderID = uint32(9301)
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Hand Plant", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := UpsertIslandHandPlantTx(context.Background(), tx, &IslandHandPlant{
			CommanderID: commanderID,
			BuildID:     10101,
			SlotID:      2001,
			State:       1,
			FormulaID:   3001,
			StartTime:   100,
			EndTime:     200,
		}); err != nil {
			return err
		}
		if err := ResetIslandHandPlantsTx(context.Background(), tx, commanderID, []uint32{2001}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	rows, err := ListIslandHandPlantsByBuild(commanderID, 10101)
	if err != nil {
		t.Fatalf("list by build: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].State != 0 || rows[0].FormulaID != 0 || rows[0].StartTime != 0 || rows[0].EndTime != 0 {
		t.Fatalf("expected row to be reset, got %+v", rows[0])
	}
}

func TestListIslandHandPlantsBySlotIDsForUpdateTx(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandHandPlant{})
	clearTable(t, &Commander{})

	const commanderID = uint32(9302)
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Hand Plant", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := UpsertIslandHandPlantTx(context.Background(), tx, &IslandHandPlant{
			CommanderID: commanderID,
			BuildID:     10102,
			SlotID:      2101,
			State:       1,
			FormulaID:   3002,
			StartTime:   100,
			EndTime:     220,
		}); err != nil {
			return err
		}

		rows, err := ListIslandHandPlantsBySlotIDsForUpdateTx(context.Background(), tx, commanderID, []uint32{2101, 2102})
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			t.Fatalf("expected one existing slot row, got %d", len(rows))
		}
		if rows[0].SlotID != 2101 || rows[0].FormulaID != 3002 {
			t.Fatalf("unexpected row %+v", rows[0])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
}

func TestEnsureIslandHandPlantRowsTxSerializesMissingSlot(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandHandPlant{})
	clearTable(t, &Commander{})

	const commanderID = uint32(9303)
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Hand Plant", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	tx1, err := db.DefaultStore.Pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	defer tx1.Rollback(context.Background())

	if err := EnsureIslandHandPlantRowsTx(context.Background(), tx1, commanderID, []uint32{2201}); err != nil {
		t.Fatalf("ensure rows tx1: %v", err)
	}

	resultCh := make(chan error, 1)
	go func() {
		tx2, beginErr := db.DefaultStore.Pool.Begin(context.Background())
		if beginErr != nil {
			resultCh <- beginErr
			return
		}
		defer tx2.Rollback(context.Background())

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		resultCh <- EnsureIslandHandPlantRowsTx(ctx, tx2, commanderID, []uint32{2201})
	}()

	err = <-resultCh
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout while row lock is held, got %v", err)
	}

	if err := tx1.Commit(context.Background()); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return EnsureIslandHandPlantRowsTx(context.Background(), tx, commanderID, []uint32{2201})
	})
	if err != nil {
		t.Fatalf("ensure rows after commit: %v", err)
	}

	var rowCount int64
	if err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM island_hand_plants
WHERE commander_id = $1 AND slot_id = $2
`, int64(commanderID), int64(2201)).Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected one serialized slot row, got %d", rowCount)
	}
}
