package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestIslandCardStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandCardState{})
	clearTable(t, &Commander{})

	commanderID := uint32(9811)
	if err := CreateCommanderRoot(commanderID, commanderID, "card", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := NewIslandCardState(commanderID)
	state.Picture = "4001"
	state.VisitWord = "hello world"
	state.LabelCounts = []IslandCardLabelCount{{ID: 2, Num: 1}}
	state.AchieveDisplayIDs = []uint32{3, 1}
	state.GoodNum = 5
	if err := UpsertIslandCardState(state); err != nil {
		t.Fatalf("upsert card state: %v", err)
	}

	reloaded, err := GetIslandCardState(commanderID)
	if err != nil {
		t.Fatalf("get card state: %v", err)
	}
	if reloaded.Picture != "4001" || reloaded.GoodNum != 5 || len(reloaded.LabelCounts) != 1 {
		t.Fatalf("unexpected card state: %+v", reloaded)
	}
}

func TestIslandBookStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandBookState{})
	clearTable(t, &Commander{})

	commanderID := uint32(9812)
	if err := CreateCommanderRoot(commanderID, commanderID, "book", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := NewIslandBookState(commanderID)
	state.BookList = []uint32{7, 3}
	state.BookAwards = []uint32{9}
	state.BookCollects = []IslandBookCollectEntry{{ID: 7, Base: 20, LvList: []IslandBookCollectLevel{{Lv: 50, Value: 50}}}}
	if err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return SaveIslandBookStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("save book state: %v", err)
	}

	reloaded, err := GetIslandBookState(commanderID)
	if err != nil {
		t.Fatalf("get book state: %v", err)
	}
	if len(reloaded.BookList) != 2 || len(reloaded.BookCollects) != 1 {
		t.Fatalf("unexpected book state: %+v", reloaded)
	}
}

func TestIslandCardLikeAndLabelDedup(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &Commander{})
	if err := CreateCommanderRoot(9813, 9813, "a", 0, 0); err != nil {
		t.Fatalf("seed commander A: %v", err)
	}
	if err := CreateCommanderRoot(9814, 9814, "b", 0, 0); err != nil {
		t.Fatalf("seed commander B: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		insertedLike, err := AddIslandCardLikeTx(context.Background(), tx, 9813, 9814)
		if err != nil {
			return err
		}
		if !insertedLike {
			t.Fatalf("expected first like insert")
		}
		insertedLike, err = AddIslandCardLikeTx(context.Background(), tx, 9813, 9814)
		if err != nil {
			return err
		}
		if insertedLike {
			t.Fatalf("expected duplicate like insert rejected")
		}

		insertedLabel, err := AddIslandCardLabelGiftTx(context.Background(), tx, 9813, 9814, 1)
		if err != nil {
			return err
		}
		if !insertedLabel {
			t.Fatalf("expected first label gift insert")
		}
		insertedLabel, err = AddIslandCardLabelGiftTx(context.Background(), tx, 9813, 9814, 2)
		if err != nil {
			return err
		}
		if insertedLabel {
			t.Fatalf("expected duplicate target gift rejected")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx failed: %v", err)
	}
}
