package orm

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestIslandCollectStateTables(t *testing.T) {
	t.Setenv("MODE", "test")
	InitDatabase()

	commanderID := uint32(time.Now().UnixNano())
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Collect Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		created, err := CreateIslandWildGatherCollectStateTx(context.Background(), tx, 9001, 77, commanderID)
		if err != nil {
			return err
		}
		if !created {
			t.Fatalf("expected gather collect state creation")
		}
		created, err = CreateIslandWildGatherCollectStateTx(context.Background(), tx, 9001, 77, commanderID)
		if err != nil {
			return err
		}
		if created {
			t.Fatalf("expected duplicate gather collect state rejection")
		}

		pickedUp, err := CreateIslandCollectFragmentStateTx(context.Background(), tx, 9001, 501, commanderID, commanderID)
		if err != nil {
			return err
		}
		if !pickedUp {
			t.Fatalf("expected fragment pickup insert")
		}
		hasFragment, err := HasIslandCollectFragmentTx(context.Background(), tx, 9001, 501)
		if err != nil {
			return err
		}
		if !hasFragment {
			t.Fatalf("expected fragment state to exist")
		}

		completed, err := MarkIslandCollectionCompletedTx(context.Background(), tx, commanderID, 301)
		if err != nil {
			return err
		}
		if !completed {
			t.Fatalf("expected completion insert")
		}
		completed, err = MarkIslandCollectionCompletedTx(context.Background(), tx, commanderID, 301)
		if err != nil {
			return err
		}
		if completed {
			t.Fatalf("expected duplicate completion to be ignored")
		}

		isCompleted, err := IsIslandCollectionCompletedTx(context.Background(), tx, commanderID, 301)
		if err != nil {
			return err
		}
		if !isCompleted {
			t.Fatalf("expected completion marker")
		}

		state := &IslandSlotCollectState{
			CommanderID:     commanderID,
			BuildID:         401,
			AreaID:          2001,
			SlotType:        1,
			NextRefreshTime: 1234,
			CollectedCount:  1,
			Consumed:        false,
		}
		if err := UpsertIslandSlotCollectStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		reloaded, err := GetIslandSlotCollectStateTx(context.Background(), tx, commanderID, 401, 2001, 1)
		if err != nil {
			return err
		}
		if reloaded == nil || reloaded.NextRefreshTime != 1234 || reloaded.CollectedCount != 1 {
			t.Fatalf("unexpected slot collect state: %+v", reloaded)
		}

		state.NextRefreshTime = 5678
		state.CollectedCount = 2
		if err := UpsertIslandSlotCollectStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		reloaded, err = GetIslandSlotCollectStateTx(context.Background(), tx, commanderID, 401, 2001, 1)
		if err != nil {
			return err
		}
		if reloaded == nil || reloaded.NextRefreshTime != 5678 || reloaded.CollectedCount != 2 {
			t.Fatalf("expected upserted slot collect state update, got %+v", reloaded)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("collect state tx failed: %v", err)
	}
}
