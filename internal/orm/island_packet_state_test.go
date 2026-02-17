package orm

import "testing"

func TestIslandSnapshotRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandSnapshot{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9701, 9701, "snapshot", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	if err := UpsertIslandSnapshot(&IslandSnapshot{CommanderID: 9701, Name: "My Island", Level: 5, StorageLevel: 3, FollowShips: []uint32{1, 2}}); err != nil {
		t.Fatalf("upsert snapshot: %v", err)
	}
	state, err := GetIslandSnapshot(9701)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if state.Name != "My Island" || state.Level != 5 || len(state.FollowShips) != 2 {
		t.Fatalf("unexpected snapshot round-trip: %+v", state)
	}
}

func TestIslandSignInStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandSignInState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9702, 9702, "signin", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	if err := UpsertIslandSignInState(&IslandSignInState{CommanderID: 9702, DayStartUnix: 100, SignedIn: true, ExternalClaimCount: 2, ClaimedSlots: []string{"1:1"}}); err != nil {
		t.Fatalf("upsert sign-in: %v", err)
	}
	state, err := GetIslandSignInState(9702)
	if err != nil {
		t.Fatalf("get sign-in: %v", err)
	}
	if !state.SignedIn || state.ExternalClaimCount != 2 || len(state.ClaimedSlots) != 1 {
		t.Fatalf("unexpected sign-in state: %+v", state)
	}
}

func TestIslandTechnologyStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandTechnologyState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9703, 9703, "tech", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := NewIslandTechnologyState(9703)
	state.UnlockedTechIDs = []uint32{1}
	state.AbilityIDs = []uint32{2}
	state.FinishCounts[1] = 3
	if err := UpsertIslandTechnologyState(state); err != nil {
		t.Fatalf("upsert tech: %v", err)
	}
	reloaded, err := GetIslandTechnologyState(9703)
	if err != nil {
		t.Fatalf("get tech: %v", err)
	}
	if len(reloaded.UnlockedTechIDs) != 1 || reloaded.FinishCounts[1] != 3 {
		t.Fatalf("unexpected tech state: %+v", reloaded)
	}
}

func TestIslandShopAndDressStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &IslandShopState{})
	clearTable(t, &IslandCommanderDressState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9704, 9704, "shop", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	if err := UpsertIslandShopState(&IslandShopState{CommanderID: 9704, ShopID: 22, RefreshCount: 1, Goods: []IslandShopGoodsState{{ID: 100, Num: 0}}}); err != nil {
		t.Fatalf("upsert shop: %v", err)
	}
	shop, err := GetIslandShopState(9704, 22)
	if err != nil {
		t.Fatalf("get shop: %v", err)
	}
	if shop.RefreshCount != 1 || len(shop.Goods) != 1 {
		t.Fatalf("unexpected shop state: %+v", shop)
	}

	if err := MarkCommanderIslandDressRead(9704, []uint32{3001, 3002, 3002}); err != nil {
		t.Fatalf("mark dress read: %v", err)
	}
	dresses, err := ListIslandCommanderDressStates(9704)
	if err != nil {
		t.Fatalf("list dress: %v", err)
	}
	if len(dresses) != 2 {
		t.Fatalf("expected two dress rows, got %d", len(dresses))
	}
}
