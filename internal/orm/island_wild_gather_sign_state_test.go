package orm

import (
	"testing"
	"time"
)

func TestIslandWildGatherSignStateUpsertAndList(t *testing.T) {
	t.Setenv("MODE", "test")
	InitDatabase()

	commanderID := uint32(time.Now().UnixNano())
	if err := CreateCommanderRoot(commanderID, commanderID, "Gather Sign Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	state := &IslandWildGatherSignState{
		IslandID:          5001,
		GatherID:          300,
		SignerCommanderID: commanderID,
		Mark:              commanderID,
	}
	if err := UpsertIslandWildGatherSignState(state); err != nil {
		t.Fatalf("upsert sign state: %v", err)
	}

	state.Mark = commanderID + 1
	if err := UpsertIslandWildGatherSignState(state); err != nil {
		t.Fatalf("upsert sign state update: %v", err)
	}

	states, err := ListIslandWildGatherSignStates(5001, 300)
	if err != nil {
		t.Fatalf("list sign states: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 row, got %d", len(states))
	}
	if states[0].Mark != commanderID+1 {
		t.Fatalf("expected mark %d, got %d", commanderID+1, states[0].Mark)
	}
}
