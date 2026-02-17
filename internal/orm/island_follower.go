package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandFollower struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	ShipID      uint32 `gorm:"primaryKey;column:ship_id"`
	OrderIdx    uint32 `gorm:"column:order_idx"`
}

func (IslandFollower) TableName() string {
	return "island_followers"
}

func ListIslandFollowers(commanderID uint32) ([]IslandFollower, error) {
	ctx := context.Background()
	return listIslandFollowers(ctx, db.DefaultStore.Pool, commanderID, false)
}

func ListIslandFollowersForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]IslandFollower, error) {
	return listIslandFollowers(ctx, tx, commanderID, true)
}

type islandFollowerQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func listIslandFollowers(ctx context.Context, queryer islandFollowerQueryer, commanderID uint32, forUpdate bool) ([]IslandFollower, error) {
	query := `
SELECT commander_id, ship_id, order_idx
FROM island_followers
WHERE commander_id = $1
ORDER BY order_idx ASC, ship_id ASC
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	rows, err := queryer.Query(ctx, query, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	followers := make([]IslandFollower, 0)
	for rows.Next() {
		var (
			commanderIDRaw int64
			shipIDRaw      int64
			orderIdxRaw    int64
		)
		if err := rows.Scan(&commanderIDRaw, &shipIDRaw, &orderIdxRaw); err != nil {
			return nil, err
		}
		followers = append(followers, IslandFollower{
			CommanderID: uint32(commanderIDRaw),
			ShipID:      uint32(shipIDRaw),
			OrderIdx:    uint32(orderIdxRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return followers, nil
}

func AddIslandFollowerTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32, orderIdx uint32) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_followers (commander_id, ship_id, order_idx)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, ship_id)
DO UPDATE SET order_idx = EXCLUDED.order_idx
`, int64(commanderID), int64(shipID), int64(orderIdx))
	return err
}

func RemoveIslandFollowerTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) error {
	_, err := tx.Exec(ctx, `
DELETE FROM island_followers
WHERE commander_id = $1 AND ship_id = $2
`, int64(commanderID), int64(shipID))
	return err
}
