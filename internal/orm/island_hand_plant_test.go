package orm

import (
	"context"
	"testing"

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
