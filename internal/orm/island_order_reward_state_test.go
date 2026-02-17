package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestIslandOrderStateAndClaimsRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandOrderState{})
	clearTable(t, &Commander{})
	clearTable(t, &IslandInventory{})

	const commanderID = 19401
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Order", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := GetIslandOrderStateForUpdateTx(context.Background(), tx, commanderID)
		if err != nil {
			return err
		}
		state.Favor = 120
		if err := SaveIslandOrderStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		if _, err := AddIslandOrderFavorClaimTx(context.Background(), tx, commanderID, 3); err != nil {
			return err
		}
		if _, err := AddIslandOrderFavorClaimTx(context.Background(), tx, commanderID, 7); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("save order state: %v", err)
	}

	state, err := GetIslandOrderState(commanderID)
	if err != nil {
		t.Fatalf("get order state: %v", err)
	}
	if state.Favor != 120 {
		t.Fatalf("expected favor 120, got %d", state.Favor)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		levels, err := ListIslandOrderFavorClaimsTx(context.Background(), tx, commanderID)
		if err != nil {
			return err
		}
		if len(levels) != 2 || levels[0] != 3 || levels[1] != 7 {
			t.Fatalf("unexpected levels: %v", levels)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("list favor claims: %v", err)
	}
}

func TestIslandOrderSlotPersistence(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Commander{})
	clearTable(t, &IslandOrderSlot{})

	const commanderID = 19402
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Slots", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	slot := &protobuf.PB_ISLAND_ORDER_SLOT{
		Id:         proto.Uint32(201),
		Type:       proto.Uint32(2),
		CurSelect:  proto.Uint32(1),
		StartTime:  proto.Uint32(100),
		SubmitTime: proto.Uint32(101),
		Position:   proto.Uint32(1),
		DialogId:   proto.Uint32(30),
		Cost: []*protobuf.PB_ISLAND_ITEM{
			{Id: proto.Uint32(9001), Num: proto.Uint32(4)},
		},
		OrderLv:  proto.Uint32(3),
		ViewFlag: proto.Uint32(0),
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return UpsertIslandOrderSlotTx(context.Background(), tx, commanderID, slot)
	})
	if err != nil {
		t.Fatalf("upsert slot: %v", err)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		loaded, err := GetIslandOrderSlotForUpdateTx(context.Background(), tx, commanderID, 201)
		if err != nil {
			return err
		}
		if loaded.GetOrderLv() != 3 || len(loaded.GetCost()) != 1 {
			t.Fatalf("unexpected loaded slot: %+v", loaded)
		}
		return DeleteIslandOrderSlotTx(context.Background(), tx, commanderID, 201)
	})
	if err != nil {
		t.Fatalf("load/delete slot: %v", err)
	}
}

type IslandOrderSlot struct{}

func (IslandOrderSlot) TableName() string {
	return "island_order_slots"
}
