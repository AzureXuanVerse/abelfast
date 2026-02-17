package orm

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

var ErrInsufficientIslandInventory = errors.New("insufficient island inventory")

type IslandInventory struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	ItemID      uint32 `gorm:"primaryKey;column:item_id"`
	Count       uint32 `gorm:"column:count"`
}

func (IslandInventory) TableName() string {
	return "island_inventories"
}

func AddIslandInventoryTx(ctx context.Context, tx pgx.Tx, commanderID uint32, itemID uint32, count uint32) error {
	if count == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO island_inventories (commander_id, item_id, count)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, item_id)
DO UPDATE SET count = island_inventories.count + EXCLUDED.count
`, int64(commanderID), int64(itemID), int64(count))
	return err
}

func ConsumeIslandInventoryCheckedTx(ctx context.Context, tx pgx.Tx, commanderID uint32, itemID uint32, count uint32) error {
	if count == 0 {
		return nil
	}
	result, err := tx.Exec(ctx, `
UPDATE island_inventories
SET count = count - $3
WHERE commander_id = $1 AND item_id = $2 AND count >= $3
`, int64(commanderID), int64(itemID), int64(count))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	_, err = tx.Exec(ctx, `
DELETE FROM island_inventories
WHERE commander_id = $1 AND item_id = $2 AND count = 0
`, int64(commanderID), int64(itemID))
	return err
}

func ListIslandInventoryItems(commanderID uint32) ([]IslandInventory, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, item_id, count
FROM island_inventories
WHERE commander_id = $1
ORDER BY item_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]IslandInventory, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var itemIDRaw int64
		var countRaw int64
		if err := rows.Scan(&commanderIDRaw, &itemIDRaw, &countRaw); err != nil {
			return nil, err
		}
		items = append(items, IslandInventory{
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

func GetIslandInventoryItem(commanderID uint32, itemID uint32) (*IslandInventory, error) {
	var (
		commanderIDRaw int64
		itemIDRaw      int64
		countRaw       int64
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, item_id, count
FROM island_inventories
WHERE commander_id = $1 AND item_id = $2
`, int64(commanderID), int64(itemID)).Scan(&commanderIDRaw, &itemIDRaw, &countRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &IslandInventory{
		CommanderID: uint32(commanderIDRaw),
		ItemID:      uint32(itemIDRaw),
		Count:       uint32(countRaw),
	}, nil
}

func ConsumeIslandInventoryTx(ctx context.Context, tx pgx.Tx, commanderID uint32, itemID uint32, count uint32) error {
	if count == 0 {
		return nil
	}
	row := tx.QueryRow(ctx, `
UPDATE island_inventories
SET count = count - $3
WHERE commander_id = $1 AND item_id = $2 AND count >= $3
RETURNING count
`, int64(commanderID), int64(itemID), int64(count))
	var remaining int64
	err := row.Scan(&remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInsufficientIslandInventory
	}
	return err
}
