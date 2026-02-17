package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandShip struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	ShipID      uint32 `gorm:"primaryKey;column:ship_id"`
	Level       uint32 `gorm:"column:level"`
	BreakLv     uint32 `gorm:"column:break_lv"`
	CanFollow   bool   `gorm:"column:can_follow"`
}

func (IslandShip) TableName() string {
	return "island_ships"
}

func UpsertIslandShip(ship *IslandShip) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO island_ships (commander_id, ship_id, level, break_lv, can_follow)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (commander_id, ship_id)
DO UPDATE SET
	level = EXCLUDED.level,
	break_lv = EXCLUDED.break_lv,
	can_follow = EXCLUDED.can_follow
`, int64(ship.CommanderID), int64(ship.ShipID), int64(ship.Level), int64(ship.BreakLv), ship.CanFollow)
	return err
}

func GetIslandShip(commanderID uint32, shipID uint32) (*IslandShip, error) {
	ctx := context.Background()
	return queryIslandShip(ctx, db.DefaultStore.Pool, commanderID, shipID, false)
}

func GetIslandShipForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) (*IslandShip, error) {
	return queryIslandShip(ctx, tx, commanderID, shipID, true)
}

type islandShipQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandShip(ctx context.Context, queryer islandShipQueryer, commanderID uint32, shipID uint32, forUpdate bool) (*IslandShip, error) {
	query := `
SELECT commander_id, ship_id, level, break_lv, can_follow
FROM island_ships
WHERE commander_id = $1 AND ship_id = $2
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var (
		commanderIDRaw int64
		shipIDRaw      int64
		levelRaw       int64
		breakLvRaw     int64
		ship           IslandShip
	)
	err := queryer.QueryRow(ctx, query, int64(commanderID), int64(shipID)).Scan(
		&commanderIDRaw,
		&shipIDRaw,
		&levelRaw,
		&breakLvRaw,
		&ship.CanFollow,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}

	ship.CommanderID = uint32(commanderIDRaw)
	ship.ShipID = uint32(shipIDRaw)
	ship.Level = uint32(levelRaw)
	ship.BreakLv = uint32(breakLvRaw)
	return &ship, nil
}

func IncrementIslandShipBreakoutTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) error {
	_, err := tx.Exec(ctx, `
UPDATE island_ships
SET break_lv = break_lv + 1
WHERE commander_id = $1 AND ship_id = $2
`, int64(commanderID), int64(shipID))
	return err
}
