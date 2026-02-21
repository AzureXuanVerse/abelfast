package orm

import "testing"

func TestTechnologyResearchStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &TechnologyResearchState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9801, 9801, "tech-state", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := &TechnologyResearchState{
		CommanderID:    9801,
		RefreshFlag:    1,
		RefreshDay:     20260221,
		CatchupVersion: 2,
		CatchupTarget:  19901,
		RefreshPools: []TechnologyRefreshPoolState{
			{ID: 1, Target: 0, Technologies: []TechnologyProjectState{{TechID: 1, FinishTime: 12345}}},
		},
		Queue: []TechnologyQueueState{{TechID: 1, RefreshID: 1, FinishTime: 12345}},
	}
	if err := SaveTechnologyResearchState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := GetTechnologyResearchState(9801)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if loaded.RefreshFlag != 1 || loaded.CatchupVersion != 2 {
		t.Fatalf("unexpected scalar fields: %+v", loaded)
	}
	if len(loaded.RefreshPools) != 1 || len(loaded.Queue) != 1 {
		t.Fatalf("unexpected collection fields: %+v", loaded)
	}
}

func TestBuildTechnologyRefreshPools(t *testing.T) {
	initCommanderItemTestDB(t)
	pools, err := BuildTechnologyRefreshPools(0)
	if err != nil {
		t.Fatalf("build pools: %v", err)
	}
	if len(pools) == 0 {
		t.Fatalf("expected at least one pool")
	}
	for _, pool := range pools {
		if pool.ID == 0 {
			t.Fatalf("pool id must be non-zero")
		}
		if len(pool.Technologies) == 0 {
			t.Fatalf("pool %d has no technologies", pool.ID)
		}
	}
}
