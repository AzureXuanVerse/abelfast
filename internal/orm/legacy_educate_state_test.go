package orm

import (
	"os"
	"testing"
)

func TestLegacyEducateStateRoundTrip(t *testing.T) {
	os.Setenv("MODE", "test")
	InitDatabase()

	if err := UpsertConfigEntry(legacyEducateStateCategory, "1", []byte(`{"commander_id":1}`)); err != nil {
		t.Fatalf("seed legacy state: %v", err)
	}

	state, err := GetOrCreateLegacyEducateState(1)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.FavorLv != 1 {
		t.Fatalf("expected default favor level 1, got %d", state.FavorLv)
	}

	state.CallName = "Commander"
	state.TargetID = 2
	state.TaskProgress[101] = 3
	state.Resources[3] = 7
	state.Endings = []uint32{11}
	if err := SaveLegacyEducateState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	reloaded, err := GetOrCreateLegacyEducateState(1)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if reloaded.CallName != "Commander" {
		t.Fatalf("expected call name Commander, got %q", reloaded.CallName)
	}
	if reloaded.TargetID != 2 {
		t.Fatalf("expected target 2, got %d", reloaded.TargetID)
	}
	if reloaded.TaskProgress[101] != 3 {
		t.Fatalf("expected task progress 3, got %d", reloaded.TaskProgress[101])
	}
	if reloaded.Resources[3] != 7 {
		t.Fatalf("expected resource 3 to equal 7, got %d", reloaded.Resources[3])
	}
	if len(reloaded.Endings) != 1 || reloaded.Endings[0] != 11 {
		t.Fatalf("expected endings [11], got %v", reloaded.Endings)
	}
}
