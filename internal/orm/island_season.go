package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandSeason struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	PT          uint32 `gorm:"column:pt"`
}

func (IslandSeason) TableName() string {
	return "island_seasons"
}

func AddIslandSeasonPTTx(ctx context.Context, tx pgx.Tx, commanderID uint32, pt uint32) error {
	if pt == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO island_seasons (commander_id, pt)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET pt = island_seasons.pt + EXCLUDED.pt
`, int64(commanderID), int64(pt))
	return err
}

func GetIslandSeason(commanderID uint32) (*IslandSeason, error) {
	var (
		commanderIDRaw int64
		ptRaw          int64
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, pt
FROM island_seasons
WHERE commander_id = $1
`, int64(commanderID)).Scan(&commanderIDRaw, &ptRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &IslandSeason{CommanderID: uint32(commanderIDRaw), PT: uint32(ptRaw)}, nil
}
