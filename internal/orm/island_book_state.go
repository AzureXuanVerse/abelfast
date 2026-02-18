package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandBookCond struct {
	CommanderID uint32 `json:"commander_id"`
	Type        uint32 `json:"type"`
	UnlockID    uint32 `json:"unlock_id"`
}

func (IslandBookCond) TableName() string {
	return "island_book_conds"
}

func AddIslandBookCondTx(ctx context.Context, tx pgx.Tx, commanderID uint32, condType uint32, unlockID uint32) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_book_conds (commander_id, type, unlock_id)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, type, unlock_id) DO NOTHING
`, int64(commanderID), int64(condType), int64(unlockID))
	return err
}

func IslandBookCondExistsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, condType uint32, unlockID uint32) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM island_book_conds
  WHERE commander_id = $1 AND type = $2 AND unlock_id = $3
)
`, int64(commanderID), int64(condType), int64(unlockID)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func ListIslandBookConds(commanderID uint32) ([]IslandBookCond, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, type, unlock_id
FROM island_book_conds
WHERE commander_id = $1
ORDER BY type ASC, unlock_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conds := make([]IslandBookCond, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var condTypeRaw int64
		var unlockIDRaw int64
		if err := rows.Scan(&commanderIDRaw, &condTypeRaw, &unlockIDRaw); err != nil {
			return nil, err
		}
		conds = append(conds, IslandBookCond{
			CommanderID: uint32(commanderIDRaw),
			Type:        uint32(condTypeRaw),
			UnlockID:    uint32(unlockIDRaw),
		})
	}

	return conds, rows.Err()
}
