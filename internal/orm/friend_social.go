package orm

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

var ErrFriendRequestNotFound = errors.New("friend request not found")

type FriendRequest struct {
	RequesterID uint32
	TargetID    uint32
	Content     string
	CreatedAt   time.Time
}

type FriendLink struct {
	CommanderID uint32
	FriendID    uint32
	CreatedAt   time.Time
}

type PendingFriendRequest struct {
	Requester CommanderSocialProfile
	Content   string
	CreatedAt time.Time
}

func CreateFriendRequest(requesterID uint32, targetID uint32, content string) (bool, error) {
	ctx := context.Background()
	res, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO friend_requests (requester_id, target_id, content)
VALUES ($1, $2, $3)
ON CONFLICT (requester_id, target_id)
DO NOTHING
`, int64(requesterID), int64(targetID), content)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func DeleteFriendRequest(targetID uint32, requesterID uint32) (bool, error) {
	ctx := context.Background()
	res, err := db.DefaultStore.Pool.Exec(ctx, `
DELETE FROM friend_requests
WHERE requester_id = $1
  AND target_id = $2
`, int64(requesterID), int64(targetID))
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func DeleteAllFriendRequestsForTarget(targetID uint32) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
DELETE FROM friend_requests
WHERE target_id = $1
`, int64(targetID))
	return err
}

func ListIncomingFriendRequests(targetID uint32) ([]PendingFriendRequest, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT fr.requester_id, c.name, c.level, fr.content, fr.created_at
FROM friend_requests fr
JOIN commanders c ON c.commander_id = fr.requester_id
WHERE fr.target_id = $1
ORDER BY fr.created_at ASC
`, int64(targetID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]PendingFriendRequest, 0)
	for rows.Next() {
		var requesterID int64
		var level int64
		var request PendingFriendRequest
		if err := rows.Scan(&requesterID, &request.Requester.Name, &level, &request.Content, &request.CreatedAt); err != nil {
			return nil, err
		}
		request.Requester.CommanderID = uint32(requesterID)
		request.Requester.Level = uint32(level)
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

func CountFriends(commanderID uint32) (uint32, error) {
	ctx := context.Background()
	var count int64
	if err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM friend_links fl
JOIN commanders c ON c.commander_id = fl.friend_id
WHERE fl.commander_id = $1
  AND c.deleted_at IS NULL
`, int64(commanderID)).Scan(&count); err != nil {
		return 0, err
	}
	return uint32(count), nil
}

func AreFriends(commanderID uint32, otherID uint32) (bool, error) {
	ctx := context.Background()
	var exists bool
	if err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT EXISTS(
	SELECT 1 FROM friend_links WHERE commander_id = $1 AND friend_id = $2
)
`, int64(commanderID), int64(otherID)).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func CreateFriendLinkPair(commanderID uint32, friendID uint32) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
INSERT INTO friend_links (commander_id, friend_id)
VALUES ($1, $2)
ON CONFLICT (commander_id, friend_id)
DO NOTHING
`, int64(commanderID), int64(friendID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO friend_links (commander_id, friend_id)
VALUES ($1, $2)
ON CONFLICT (commander_id, friend_id)
DO NOTHING
`, int64(friendID), int64(commanderID)); err != nil {
			return err
		}
		return nil
	})
}

func AcceptFriendRequest(targetID uint32, requesterID uint32) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		res, err := tx.Exec(ctx, `
DELETE FROM friend_requests
WHERE requester_id = $1
  AND target_id = $2
`, int64(requesterID), int64(targetID))
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			return ErrFriendRequestNotFound
		}

		if _, err := tx.Exec(ctx, `
DELETE FROM friend_requests
WHERE requester_id = $1
  AND target_id = $2
`, int64(targetID), int64(requesterID)); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
INSERT INTO friend_links (commander_id, friend_id)
VALUES ($1, $2)
ON CONFLICT (commander_id, friend_id)
DO NOTHING
`, int64(targetID), int64(requesterID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO friend_links (commander_id, friend_id)
VALUES ($1, $2)
ON CONFLICT (commander_id, friend_id)
DO NOTHING
`, int64(requesterID), int64(targetID)); err != nil {
			return err
		}

		return nil
	})
}

func GetCommanderSocialProfile(commanderID uint32) (CommanderSocialProfile, error) {
	ctx := context.Background()
	var profile CommanderSocialProfile
	var id int64
	var level int64
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, name, level
FROM commanders
WHERE commander_id = $1
  AND deleted_at IS NULL
`, int64(commanderID)).Scan(&id, &profile.Name, &level)
	err = db.MapNotFound(err)
	if err != nil {
		return CommanderSocialProfile{}, err
	}
	profile.CommanderID = uint32(id)
	profile.Level = uint32(level)
	return profile, nil
}

func ListFriendProfiles(commanderID uint32) ([]CommanderSocialProfile, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT c.commander_id, c.name, c.level
FROM friend_links fl
JOIN commanders c ON c.commander_id = fl.friend_id
WHERE fl.commander_id = $1
ORDER BY fl.created_at ASC, c.commander_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	friends := make([]CommanderSocialProfile, 0)
	for rows.Next() {
		var profile CommanderSocialProfile
		var id int64
		var level int64
		if err := rows.Scan(&id, &profile.Name, &level); err != nil {
			return nil, err
		}
		profile.CommanderID = uint32(id)
		profile.Level = uint32(level)
		friends = append(friends, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return friends, nil
}

func ListFriendRequestsForTarget(targetID uint32) ([]FriendRequest, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT requester_id, target_id, content, created_at
FROM friend_requests
WHERE target_id = $1
ORDER BY created_at ASC
`, int64(targetID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]FriendRequest, 0)
	for rows.Next() {
		var requesterID int64
		var target int64
		var request FriendRequest
		if err := rows.Scan(&requesterID, &target, &request.Content, &request.CreatedAt); err != nil {
			return nil, err
		}
		request.RequesterID = uint32(requesterID)
		request.TargetID = uint32(target)
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}
