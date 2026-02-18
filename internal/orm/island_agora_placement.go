package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandAgoraPlacement struct {
	CommanderID uint32 `json:"commander_id"`
	PlacedData  []byte `json:"placed_data"`
}

func (IslandAgoraPlacement) TableName() string {
	return "island_agora_placements"
}

func UpsertIslandAgoraPlacementTx(ctx context.Context, tx pgx.Tx, commanderID uint32, placedData []byte) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_agora_placements (commander_id, placed_data)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET
  placed_data = EXCLUDED.placed_data,
  updated_at = CURRENT_TIMESTAMP
`, int64(commanderID), placedData)
	return err
}

func GetIslandAgoraPlacement(commanderID uint32) (*IslandAgoraPlacement, error) {
	placement := &IslandAgoraPlacement{}
	var commanderIDRaw int64
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, placed_data
FROM island_agora_placements
WHERE commander_id = $1
`, int64(commanderID)).Scan(&commanderIDRaw, &placement.PlacedData)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	placement.CommanderID = uint32(commanderIDRaw)
	return placement, nil
}
