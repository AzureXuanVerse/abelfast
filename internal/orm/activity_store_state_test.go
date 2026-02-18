package orm

import "testing"

func TestActivityStoreStateRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &ActivityStoreState{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(9802, 9802, "activity store", 0, 0); err != nil {
		t.Fatalf("seed commander: %v", err)
	}

	state := &ActivityStoreState{CommanderID: 9802, ActivityID: 6001, Data1: 55, StrData1: "abc"}
	if err := UpsertActivityStoreState(state); err != nil {
		t.Fatalf("upsert state: %v", err)
	}

	reloaded, err := GetActivityStoreState(9802, 6001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if reloaded.Data1 != 55 || reloaded.StrData1 != "abc" {
		t.Fatalf("unexpected state: %+v", reloaded)
	}
}
