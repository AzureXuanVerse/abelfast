package orm

import (
	"testing"
)

func TestMiniGameHubStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &ConfigEntry{})

	if err := UpsertConfigEntry(miniGameHubCategory, "100", []byte(`{"id":100,"reborn_times":3,"reward_need":2,"reward_display":[2,123,1]}`)); err != nil {
		t.Fatalf("seed hub config: %v", err)
	}

	config, err := GetMiniGameHubConfig(100)
	if err != nil {
		t.Fatalf("load hub config: %v", err)
	}
	state, err := GetOrCreateMiniGameHubState(42, config)
	if err != nil {
		t.Fatalf("create state: %v", err)
	}
	if state.AvailableCnt != 3 {
		t.Fatalf("expected default available count 3, got %d", state.AvailableCnt)
	}
	state.UsedCnt = 2
	state.MaxScores[7001] = MiniGameScoreEntry{Score: 222, Extra: 9}
	if err := SaveMiniGameHubState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	reloaded, err := GetOrCreateMiniGameHubState(42, config)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if reloaded.UsedCnt != 2 {
		t.Fatalf("expected used count 2, got %d", reloaded.UsedCnt)
	}
	if reloaded.MaxScores[7001].Score != 222 {
		t.Fatalf("expected high score 222, got %d", reloaded.MaxScores[7001].Score)
	}
}

func TestIslandNodeStateStoredSorted(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &ConfigEntry{})

	nodes := []IslandNodeState{{ID: 9, EventID: 2, IsNew: 1}, {ID: 3, EventID: 5, IsNew: 0}}
	if err := SaveIslandNodeState(44, 700, nodes); err != nil {
		t.Fatalf("save nodes: %v", err)
	}
	reloaded, err := GetOrCreateIslandNodeState(44, 700)
	if err != nil {
		t.Fatalf("load nodes: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(reloaded))
	}
	if reloaded[0].ID != 3 || reloaded[1].ID != 9 {
		t.Fatalf("expected sorted nodes by id, got %+v", reloaded)
	}
}
