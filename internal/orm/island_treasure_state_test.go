package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestIslandTreasureStatePersistsAndLoads(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandTreasureState{})
	clearTable(t, &Commander{})

	const commanderID = 9301
	if err := CreateCommanderRoot(commanderID, commanderID, "Treasure State", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := &IslandTreasureState{
		CommanderID: commanderID,
		WeekBuyNum:  7,
		SellList:    []IslandTreasureSellState{{IslandID: 771, Num: 3}},
		PriceList:   []IslandTreasurePriceState{{Timestamp: 12345, Price: 88}},
	}
	if err := UpsertIslandTreasureState(state); err != nil {
		t.Fatalf("upsert state: %v", err)
	}

	loaded, err := GetIslandTreasureState(commanderID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.WeekBuyNum != 7 {
		t.Fatalf("expected week_buy_num 7, got %d", loaded.WeekBuyNum)
	}
	if loaded.SellCount(771) != 3 {
		t.Fatalf("expected sell counter 3, got %d", loaded.SellCount(771))
	}
	if len(loaded.PriceList) != 1 || loaded.PriceList[0].Price != 88 {
		t.Fatalf("unexpected price list: %+v", loaded.PriceList)
	}
}

func TestIslandTreasureStateForUpdateCreatesRow(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandTreasureState{})
	clearTable(t, &Commander{})

	const commanderID = 9302
	if err := CreateCommanderRoot(commanderID, commanderID, "Treasure Lock", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := GetIslandTreasureStateForUpdateTx(context.Background(), tx, commanderID)
		if err != nil {
			return err
		}
		state.WeekBuyNum = 9
		state.AddSellCount(901, 2)
		state.UpsertPrice(45678, 120)
		return UpsertIslandTreasureStateTx(context.Background(), tx, state)
	})
	if err != nil {
		t.Fatalf("update in transaction: %v", err)
	}

	loaded, err := GetIslandTreasureState(commanderID)
	if err != nil {
		t.Fatalf("load state after tx: %v", err)
	}
	if loaded.WeekBuyNum != 9 {
		t.Fatalf("expected week_buy_num 9, got %d", loaded.WeekBuyNum)
	}
	if loaded.SellCount(901) != 2 {
		t.Fatalf("expected sell counter 2, got %d", loaded.SellCount(901))
	}
}
