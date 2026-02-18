package orm

import "testing"

func TestNewServerShopStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &NewServerShopState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9801, 9801, "new server shop", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := &NewServerShopState{
		CommanderID: 9801,
		ActivityID:  30862,
		Goods: []NewServerShopGoodsState{
			{ID: 301, Count: 1, BoughtRecord: []uint32{100011}},
			{ID: 302, Count: 20, BoughtRecord: []uint32{}},
		},
	}
	if err := UpsertNewServerShopState(state); err != nil {
		t.Fatalf("upsert state: %v", err)
	}

	reloaded, err := GetNewServerShopState(9801, 30862)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if len(reloaded.Goods) != 2 {
		t.Fatalf("expected 2 goods entries, got %d", len(reloaded.Goods))
	}
	if reloaded.Goods[0].ID != 301 || reloaded.Goods[0].Count != 1 || len(reloaded.Goods[0].BoughtRecord) != 1 {
		t.Fatalf("unexpected first goods state: %+v", reloaded.Goods[0])
	}
}
