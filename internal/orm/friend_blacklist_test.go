package orm

import (
	"testing"
	"time"
)

func TestFriendBlacklistAddRemoveRoundTrip(t *testing.T) {
	t.Setenv("MODE", "test")
	InitDatabase()

	ownerID := uint32(time.Now().UnixNano())
	targetID := ownerID + 1
	if err := CreateCommanderRoot(ownerID, ownerID, "Blacklist Owner", 0, 0); err != nil {
		t.Fatalf("create owner commander: %v", err)
	}
	if err := CreateCommanderRoot(targetID, targetID, "Blacklist Target", 0, 0); err != nil {
		t.Fatalf("create target commander: %v", err)
	}

	added, err := AddFriendBlacklist(ownerID, targetID)
	if err != nil {
		t.Fatalf("add blacklist: %v", err)
	}
	if !added {
		t.Fatalf("expected new blacklist entry to be added")
	}

	added, err = AddFriendBlacklist(ownerID, targetID)
	if err != nil {
		t.Fatalf("add duplicate blacklist: %v", err)
	}
	if added {
		t.Fatalf("expected duplicate add to be idempotent")
	}

	blacklist, err := GetFriendBlacklist(ownerID)
	if err != nil {
		t.Fatalf("get blacklist: %v", err)
	}
	if len(blacklist) != 1 || blacklist[0] != targetID {
		t.Fatalf("unexpected blacklist after add: %v", blacklist)
	}

	removed, err := RemoveFriendBlacklist(ownerID, targetID)
	if err != nil {
		t.Fatalf("remove blacklist: %v", err)
	}
	if !removed {
		t.Fatalf("expected remove to succeed")
	}

	removed, err = RemoveFriendBlacklist(ownerID, targetID)
	if err != nil {
		t.Fatalf("remove missing blacklist: %v", err)
	}
	if removed {
		t.Fatalf("expected missing remove to report not removed")
	}

	blacklist, err = GetFriendBlacklist(ownerID)
	if err != nil {
		t.Fatalf("get blacklist after remove: %v", err)
	}
	if len(blacklist) != 0 {
		t.Fatalf("expected empty blacklist after remove, got %v", blacklist)
	}
}
