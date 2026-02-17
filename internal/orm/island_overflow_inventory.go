package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandOverflowInventory struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	ItemID      uint32 `gorm:"primaryKey;column:item_id"`
	Count       uint32 `gorm:"column:count"`
}

func (IslandOverflowInventory) TableName() string {
	return "island_overflow_inventories"
}

func UpsertIslandOverflowInventory(commanderID uint32, itemID uint32, count uint32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_overflow_inventories (commander_id, item_id, count)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, item_id)
DO UPDATE SET count = EXCLUDED.count
`, int64(commanderID), int64(itemID), int64(count))
	return err
}

func ListIslandOverflowInventoryForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]IslandOverflowInventory, error) {
	rows, err := tx.Query(ctx, `
SELECT commander_id, item_id, count
FROM island_overflow_inventories
WHERE commander_id = $1
ORDER BY item_id
FOR UPDATE
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]IslandOverflowInventory, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var itemIDRaw int64
		var countRaw int64
		if err := rows.Scan(&commanderIDRaw, &itemIDRaw, &countRaw); err != nil {
			return nil, err
		}
		items = append(items, IslandOverflowInventory{
			CommanderID: uint32(commanderIDRaw),
			ItemID:      uint32(itemIDRaw),
			Count:       uint32(countRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func ClearIslandOverflowInventoryTx(ctx context.Context, tx pgx.Tx, commanderID uint32) error {
	_, err := tx.Exec(ctx, `
DELETE FROM island_overflow_inventories
WHERE commander_id = $1
`, int64(commanderID))
	return err
}
