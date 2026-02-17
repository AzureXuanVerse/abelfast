package orm

import (
	"testing"
	"time"
)

func TestCommanderIslandSocialStateRoundTrip(t *testing.T) {
	t.Setenv("MODE", "test")
	InitDatabase()

	commanderID := uint32(time.Now().UnixNano())
	if err := CreateCommanderRoot(commanderID, commanderID, "Island Social Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	state, err := GetOrCreateCommanderIslandSocialState(commanderID)
	if err != nil {
		t.Fatalf("get or create state: %v", err)
	}
	state.InviteCode = "ABCD1234"
	state.InviteCodeRefreshDay = 12345
	state.InvitedCommanderIDs = []uint32{10, 11}
	state.GiftCount = 2
	state.GiftTimestamp = 999
	state.GiftVisitors = []uint32{22}
	if err := SaveCommanderIslandSocialState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := GetCommanderIslandSocialState(commanderID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.InviteCode != "ABCD1234" {
		t.Fatalf("expected invite code ABCD1234, got %q", loaded.InviteCode)
	}
	if loaded.GiftCount != 2 || loaded.GiftTimestamp != 999 {
		t.Fatalf("unexpected gift state: count=%d timestamp=%d", loaded.GiftCount, loaded.GiftTimestamp)
	}
	if len(loaded.InvitedCommanderIDs) != 2 {
		t.Fatalf("expected 2 invited ids, got %d", len(loaded.InvitedCommanderIDs))
	}
}

func TestCommanderIslandSocialStateBatchLookup(t *testing.T) {
	t.Setenv("MODE", "test")
	InitDatabase()

	firstID := uint32(time.Now().UnixNano())
	secondID := firstID + 1
	if err := CreateCommanderRoot(firstID, firstID, "Island Batch 1", 0, 0); err != nil {
		t.Fatalf("create first commander: %v", err)
	}
	if err := CreateCommanderRoot(secondID, secondID, "Island Batch 2", 0, 0); err != nil {
		t.Fatalf("create second commander: %v", err)
	}

	first, _ := GetOrCreateCommanderIslandSocialState(firstID)
	first.GiftCount = 5
	first.GiftTimestamp = 777
	if err := SaveCommanderIslandSocialState(first); err != nil {
		t.Fatalf("save first state: %v", err)
	}

	states, err := BatchGetCommanderIslandSocialStates([]uint32{firstID, secondID})
	if err != nil {
		t.Fatalf("batch lookup: %v", err)
	}
	if states[firstID] == nil {
		t.Fatalf("expected first commander in result")
	}
	if states[firstID].GiftCount != 5 {
		t.Fatalf("expected first gift count 5, got %d", states[firstID].GiftCount)
	}
	if states[secondID] != nil {
		t.Fatalf("expected missing second commander state")
	}
}
