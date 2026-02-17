package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIslandShipBreakoutAndInventoryConsumeTx(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandShip{})
	clearTable(t, &IslandInventory{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(930001, 1, "Island ORM Tester", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}
	if err := UpsertIslandShip(&IslandShip{CommanderID: 930001, ShipID: 1051700, Level: 10, BreakLv: 1, CanFollow: true}); err != nil {
		t.Fatalf("seed island ship: %v", err)
	}

	if err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := AddIslandInventoryTx(context.Background(), tx, 930001, 100201, 2); err != nil {
			return err
		}
		if err := ConsumeIslandInventoryTx(context.Background(), tx, 930001, 100201, 1); err != nil {
			return err
		}
		return IncrementIslandShipBreakoutTx(context.Background(), tx, 930001, 1051700)
	}); err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	ship, err := GetIslandShip(930001, 1051700)
	if err != nil {
		t.Fatalf("get island ship: %v", err)
	}
	if ship.BreakLv != 2 {
		t.Fatalf("expected break level 2, got %d", ship.BreakLv)
	}

	item, err := GetIslandInventoryItem(930001, 100201)
	if err != nil {
		t.Fatalf("get island inventory item: %v", err)
	}
	if item.Count != 1 {
		t.Fatalf("expected remaining count 1, got %d", item.Count)
	}
}

func TestIslandFollowerListAddRemoveTx(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandFollower{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(930002, 1, "Island Follower Tester", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	if err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if err := AddIslandFollowerTx(context.Background(), tx, 930002, 101600, 0); err != nil {
			return err
		}
		if err := AddIslandFollowerTx(context.Background(), tx, 930002, 1070300, 1); err != nil {
			return err
		}
		return RemoveIslandFollowerTx(context.Background(), tx, 930002, 101600)
	}); err != nil {
		t.Fatalf("follower transaction failed: %v", err)
	}

	followers, err := ListIslandFollowers(930002)
	if err != nil {
		t.Fatalf("list followers: %v", err)
	}
	if len(followers) != 1 || followers[0].ShipID != 1070300 {
		t.Fatalf("expected one follower 1070300, got %#v", followers)
	}
}

func TestIslandShipOrderStateRoundTripTx(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandShipOrderSlot{})
	clearTable(t, &IslandShipOrderState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(930003, 1, "Island Order Tester", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := LoadIslandShipOrderStateForUpdateTx(context.Background(), tx, 930003)
		if err != nil {
			return err
		}
		state.RefreshAt = 12345
		state.AppointList = []IslandShipOrderAppoint{{ID: 100001, ViewTime: 55, Cost: [][]uint32{{4001, 2}}, Reward: [][]uint32{{5001, 1}}}}
		if err := SaveIslandShipOrderStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		slot, err := LoadIslandShipOrderSlotTx(context.Background(), tx, 930003, 301)
		if err != nil {
			return err
		}
		slot.State = 1
		slot.GetTime = 456
		return UpsertIslandShipOrderSlotTx(context.Background(), tx, slot)
	})
	if err != nil {
		t.Fatalf("order tx failed: %v", err)
	}

	err = WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := LoadIslandShipOrderStateForUpdateTx(context.Background(), tx, 930003)
		if err != nil {
			return err
		}
		if state.RefreshAt != 12345 {
			t.Fatalf("expected refresh_at 12345, got %d", state.RefreshAt)
		}
		if len(state.AppointList) != 1 || state.AppointList[0].ID != 100001 {
			t.Fatalf("expected appoint list roundtrip, got %#v", state.AppointList)
		}
		slot, err := LoadIslandShipOrderSlotTx(context.Background(), tx, 930003, 301)
		if err != nil {
			return err
		}
		if slot.State != 1 || slot.GetTime != 456 {
			t.Fatalf("expected slot roundtrip state/get_time, got %#v", slot)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("load order tx failed: %v", err)
	}
}
