package orm

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func TestConsumeIslandSpeedupTicketsTxAtomic(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandSpeedupTicket{})
	clearTable(t, &Commander{})

	const commanderID = 9301
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Speedup", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}
	if err := UpsertIslandSpeedupTicket(commanderID, 1001, 100, 2); err != nil {
		t.Fatalf("seed ticket 1: %v", err)
	}
	if err := UpsertIslandSpeedupTicket(commanderID, 1002, 200, 1); err != nil {
		t.Fatalf("seed ticket 2: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		requests := []IslandSpeedupTicketConsume{
			{SpeedID: 1001, EndTime: 100, Count: 1},
			{SpeedID: 1002, EndTime: 200, Count: 2},
		}
		return ConsumeIslandSpeedupTicketsTx(context.Background(), tx, commanderID, requests)
	})
	if err == nil {
		t.Fatalf("expected consume failure")
	}

	tickets, err := ListIslandSpeedupTickets(commanderID)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("expected no partial consume, got %+v", tickets)
	}
}

func TestDeleteIslandSpeedupTicketKeysTx(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandSpeedupTicket{})
	clearTable(t, &Commander{})

	const commanderID = 9302
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Speedup Delete", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}
	if err := UpsertIslandSpeedupTicket(commanderID, 1001, 100, 2); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return DeleteIslandSpeedupTicketKeysTx(context.Background(), tx, commanderID, []IslandSpeedupTicketKey{{SpeedID: 1001, EndTime: 100}})
	})
	if err != nil {
		t.Fatalf("delete keys: %v", err)
	}

	tickets, err := ListIslandSpeedupTickets(commanderID)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 0 {
		t.Fatalf("expected empty tickets, got %+v", tickets)
	}
}
