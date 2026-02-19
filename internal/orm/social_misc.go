package orm

import (
	"context"
	"fmt"

	"github.com/ggmolly/belfast/internal/db"
)

type FriendRelationship struct {
	CommanderID uint32 `json:"commander_id"`
	FriendID    uint32 `json:"friend_id"`
	CreatedAt   uint32 `json:"created_at"`
}

type FriendDirectMessage struct {
	ID         uint64 `json:"id"`
	SenderID   uint32 `json:"sender_id"`
	ReceiverID uint32 `json:"receiver_id"`
	Content    string `json:"content"`
	CreatedAt  uint32 `json:"created_at"`
}

type PlayerInform struct {
	ID         uint64 `json:"id"`
	ReporterID uint32 `json:"reporter_id"`
	TargetID   uint32 `json:"target_id"`
	Info       string `json:"info"`
	Content    string `json:"content"`
	CreatedAt  uint32 `json:"created_at"`
}

func normalizeFriendPair(commanderID uint32, friendID uint32) (uint32, uint32, error) {
	if commanderID == friendID {
		return 0, 0, fmt.Errorf("commander id and friend id cannot match")
	}
	if commanderID < friendID {
		return commanderID, friendID, nil
	}
	return friendID, commanderID, nil
}

func CreateFriendRelationship(commanderID uint32, friendID uint32, createdAt uint32) error {
	left, right, err := normalizeFriendPair(commanderID, friendID)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO friend_relationships (
  commander_id,
  friend_id,
  created_at
) VALUES (
  $1, $2, $3
)
ON CONFLICT (commander_id, friend_id) DO NOTHING
`, int64(left), int64(right), int64(createdAt))
	return err
}

func IsFriend(commanderID uint32, friendID uint32) (bool, error) {
	if commanderID == friendID {
		return false, nil
	}
	left, right, err := normalizeFriendPair(commanderID, friendID)
	if err != nil {
		return false, err
	}
	var exists bool
	err = db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT EXISTS(
	SELECT 1
	FROM friend_relationships
	WHERE commander_id = $1
	  AND friend_id = $2
)
`, int64(left), int64(right)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func CreateFriendDirectMessage(senderID uint32, receiverID uint32, content string, createdAt uint32) (*FriendDirectMessage, error) {
	entry := &FriendDirectMessage{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
		CreatedAt:  createdAt,
	}
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
INSERT INTO friend_direct_messages (
  sender_id,
  receiver_id,
  content,
  created_at
) VALUES (
  $1, $2, $3, $4
)
RETURNING id
`, int64(entry.SenderID), int64(entry.ReceiverID), entry.Content, int64(entry.CreatedAt)).Scan(&entry.ID)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func CreatePlayerInform(reporterID uint32, targetID uint32, info string, content string, createdAt uint32) (*PlayerInform, error) {
	entry := &PlayerInform{
		ReporterID: reporterID,
		TargetID:   targetID,
		Info:       info,
		Content:    content,
		CreatedAt:  createdAt,
	}
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
INSERT INTO player_informs (
  reporter_id,
  target_id,
  info,
  content,
  created_at
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING id
`, int64(entry.ReporterID), int64(entry.TargetID), entry.Info, entry.Content, int64(entry.CreatedAt)).Scan(&entry.ID)
	if err != nil {
		return nil, err
	}
	return entry, nil
}
