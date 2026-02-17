package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestAddIslandInventoryTxAccumulatesCount(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandInventory{})
	clearTable(t, &Commander{})

	const commanderID = 9201
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Inventory", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := AddIslandInventoryTx(context.Background(), tx, commanderID, 2000, 12); err != nil {
			return err
		}
		if err := AddIslandInventoryTx(context.Background(), tx, commanderID, 2000, 8); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("add island inventory: %v", err)
	}

	item, err := GetIslandInventoryItem(commanderID, 2000)
	if err != nil {
		t.Fatalf("get island inventory: %v", err)
	}
	if item.Count != 20 {
		t.Fatalf("expected count 20, got %d", item.Count)
	}
}
