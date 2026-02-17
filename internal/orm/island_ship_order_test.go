package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestIslandShipOrderSlotRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandShipOrderSlot{})
	clearTable(t, &Commander{})

	const commanderID = 9401
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Ship Order", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	slot := &IslandShipOrderSlot{
		CommanderID: commanderID,
		ShipSlotID:  100,
		State:       0,
		GetTime:     0,
		EndTime:     500,
		CostList: []IslandShipOrderCost{
			{ID: 7001, Num: 2, State: 0},
			{ID: 7002, Num: 3, State: 1},
		},
	}
	if err := UpsertIslandShipOrderSlot(slot); err != nil {
		t.Fatalf("upsert slot: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		reloaded, err := GetIslandRuntimeShipOrderSlotForUpdateTx(context.Background(), tx, commanderID, 100)
		if err != nil {
			return err
		}
		if len(reloaded.CostList) != 2 || reloaded.CostList[1].State != 1 {
			t.Fatalf("unexpected reloaded slot: %+v", reloaded)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read slot in tx: %v", err)
	}
}
