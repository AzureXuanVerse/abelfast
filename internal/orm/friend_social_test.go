package orm

import (
	"context"
	"testing"

	"github.com/ggmolly/belfast/internal/db"
)

func TestCreateFriendRequestIsUnique(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &FriendLink{})
	clearTable(t, &FriendRequest{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(7001, 7001, "Requester", 0, 0); err != nil {
		t.Fatalf("create requester: %v", err)
	}
	if err := CreateCommanderRoot(7002, 7002, "Target", 0, 0); err != nil {
		t.Fatalf("create target: %v", err)
	}

	created, err := CreateFriendRequest(7001, 7002, "hello")
	if err != nil {
		t.Fatalf("create friend request: %v", err)
	}
	if !created {
		t.Fatalf("expected first request to be created")
	}

	created, err = CreateFriendRequest(7001, 7002, "hello")
	if err != nil {
		t.Fatalf("create duplicate friend request: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate request to be ignored")
	}

	requests, err := ListFriendRequestsForTarget(7002)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected one request, got %d", len(requests))
	}
}

func TestAcceptFriendRequestCreatesBidirectionalFriendship(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &FriendLink{})
	clearTable(t, &FriendRequest{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(7101, 7101, "Requester", 0, 0); err != nil {
		t.Fatalf("create requester: %v", err)
	}
	if err := CreateCommanderRoot(7102, 7102, "Target", 0, 0); err != nil {
		t.Fatalf("create target: %v", err)
	}

	if _, err := CreateFriendRequest(7101, 7102, "hello"); err != nil {
		t.Fatalf("seed request: %v", err)
	}
	if err := AcceptFriendRequest(7102, 7101); err != nil {
		t.Fatalf("accept request: %v", err)
	}

	first, err := AreFriends(7101, 7102)
	if err != nil {
		t.Fatalf("check first direction: %v", err)
	}
	second, err := AreFriends(7102, 7101)
	if err != nil {
		t.Fatalf("check second direction: %v", err)
	}
	if !first || !second {
		t.Fatalf("expected bidirectional friendship")
	}

	requests, err := ListFriendRequestsForTarget(7102)
	if err != nil {
		t.Fatalf("list requests: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("expected no pending requests after accept")
	}
}

func TestAcceptFriendRequestRemovesReciprocalRequest(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &FriendLink{})
	clearTable(t, &FriendRequest{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(7201, 7201, "Requester", 0, 0); err != nil {
		t.Fatalf("create requester: %v", err)
	}
	if err := CreateCommanderRoot(7202, 7202, "Target", 0, 0); err != nil {
		t.Fatalf("create target: %v", err)
	}

	if _, err := CreateFriendRequest(7201, 7202, "forward"); err != nil {
		t.Fatalf("seed forward request: %v", err)
	}
	if _, err := CreateFriendRequest(7202, 7201, "reverse"); err != nil {
		t.Fatalf("seed reverse request: %v", err)
	}

	if err := AcceptFriendRequest(7202, 7201); err != nil {
		t.Fatalf("accept request: %v", err)
	}

	remainingForward, err := ListFriendRequestsForTarget(7202)
	if err != nil {
		t.Fatalf("list forward requests: %v", err)
	}
	if len(remainingForward) != 0 {
		t.Fatalf("expected no pending requests for target, got %d", len(remainingForward))
	}

	remainingReverse, err := ListFriendRequestsForTarget(7201)
	if err != nil {
		t.Fatalf("list reverse requests: %v", err)
	}
	if len(remainingReverse) != 0 {
		t.Fatalf("expected reciprocal request to be removed, got %d", len(remainingReverse))
	}
}

func TestCountFriendsIgnoresSoftDeletedFriend(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &FriendLink{})
	clearTable(t, &FriendRequest{})
	clearTable(t, &Commander{})

	if err := CreateCommanderRoot(7301, 7301, "Owner", 0, 0); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := CreateCommanderRoot(7302, 7302, "Active", 0, 0); err != nil {
		t.Fatalf("create active friend: %v", err)
	}
	if err := CreateCommanderRoot(7303, 7303, "Deleted", 0, 0); err != nil {
		t.Fatalf("create deleted friend: %v", err)
	}

	if err := CreateFriendLinkPair(7301, 7302); err != nil {
		t.Fatalf("create active link: %v", err)
	}
	if err := CreateFriendLinkPair(7301, 7303); err != nil {
		t.Fatalf("create deleted link: %v", err)
	}

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
UPDATE commanders
SET deleted_at = now()
WHERE commander_id = $1
`, int64(7303)); err != nil {
		t.Fatalf("soft delete friend: %v", err)
	}

	count, err := CountFriends(7301)
	if err != nil {
		t.Fatalf("count friends: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active friend, got %d", count)
	}
}
