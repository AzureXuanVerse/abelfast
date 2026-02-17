package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

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
