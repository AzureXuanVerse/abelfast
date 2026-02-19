package orm

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
)

type CommanderFriendRelation struct {
	CommanderID       uint32 `gorm:"primaryKey"`
	FriendCommanderID uint32 `gorm:"primaryKey"`
}

func (CommanderFriendRelation) TableName() string {
	return "commander_friend_relations"
}

func CreateCommanderFriendRelationPair(commanderID uint32, friendCommanderID uint32) error {
	if commanderID == 0 || friendCommanderID == 0 || commanderID == friendCommanderID {
		return db.ErrNotFound
	}
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO commander_friend_relations (commander_id, friend_commander_id)
VALUES ($1, $2), ($2, $1)
ON CONFLICT DO NOTHING
`, int64(commanderID), int64(friendCommanderID))
	return err
}

func DeleteCommanderFriendRelationPair(commanderID uint32, friendCommanderID uint32) (bool, error) {
	ctx := context.Background()
	result, err := db.DefaultStore.Pool.Exec(ctx, `
DELETE FROM commander_friend_relations
WHERE (commander_id = $1 AND friend_commander_id = $2)
   OR (commander_id = $2 AND friend_commander_id = $1)
`, int64(commanderID), int64(friendCommanderID))
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func ListCommanderFriendIDs(commanderID uint32) ([]uint32, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT friend_commander_id
FROM commander_friend_relations
WHERE commander_id = $1
ORDER BY friend_commander_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	friendIDs := make([]uint32, 0)
	for rows.Next() {
		var friendID uint32
		if err := rows.Scan(&friendID); err != nil {
			return nil, err
		}
		friendIDs = append(friendIDs, friendID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return friendIDs, nil
}
