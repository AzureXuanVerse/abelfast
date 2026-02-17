package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestIslandDelegationClaimTypeOneClearsRewardAndIncrementsGetTimes(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandDelegation{})
	clearTable(t, &Commander{})

	const commanderID = 9101
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Delegation Claim", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	if err := UpsertIslandDelegation(&IslandDelegation{
		CommanderID: commanderID,
		BuildID:     10101,
		AreaID:      9001,
		HasRole:     true,
		RewardReady: true,
		FormulaID:   101001,
		MainNum:     2,
		OtherNum:    1,
		GetTimes:    4,
		PTAward:     9,
	}); err != nil {
		t.Fatalf("seed delegation: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		getTimes, err := ApplyIslandDelegationClaimTx(context.Background(), tx, commanderID, 10101, 9001, 1)
		if err != nil {
			return err
		}
		if getTimes != 5 {
			t.Fatalf("expected get_times=5, got %d", getTimes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("apply claim: %v", err)
	}

	reloaded, err := GetIslandDelegation(commanderID, 10101, 9001)
	if err != nil {
		t.Fatalf("reload delegation: %v", err)
	}
	if reloaded.RewardReady {
		t.Fatalf("expected reward_ready to be false")
	}
	if reloaded.MainNum != 0 || reloaded.OtherNum != 0 || reloaded.ExtraMainNum != 0 || reloaded.ExtraOtherNum != 0 {
		t.Fatalf("expected reward counters cleared")
	}
	if reloaded.GetTimes != 5 {
		t.Fatalf("expected get_times=5, got %d", reloaded.GetTimes)
	}
	if !reloaded.HasRole {
		t.Fatalf("expected role assignment to remain")
	}
}

func TestIslandDelegationClaimTypeTwoResetsGetTimes(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandDelegation{})
	clearTable(t, &Commander{})

	const commanderID = 9102
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Delegation Claim 2", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	if err := UpsertIslandDelegation(&IslandDelegation{
		CommanderID:  commanderID,
		BuildID:      10102,
		AreaID:       9002,
		HasRole:      false,
		RewardReady:  true,
		FormulaID:    101002,
		MainNum:      3,
		GetTimes:     9,
		ExtraMainNum: 4,
	}); err != nil {
		t.Fatalf("seed delegation: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		getTimes, err := ApplyIslandDelegationClaimTx(context.Background(), tx, commanderID, 10102, 9002, 2)
		if err != nil {
			return err
		}
		if getTimes != 0 {
			t.Fatalf("expected get_times=0, got %d", getTimes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("apply claim: %v", err)
	}

	reloaded, err := GetIslandDelegation(commanderID, 10102, 9002)
	if err != nil {
		t.Fatalf("reload delegation: %v", err)
	}
	if reloaded.GetTimes != 0 {
		t.Fatalf("expected get_times=0, got %d", reloaded.GetTimes)
	}
}
