package orm

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
)

func ListOwnedShipMetaRepairIDs(ownerID uint32, shipID uint32) ([]uint32, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT repair_id
FROM owned_ship_meta_repairs
WHERE owner_id = $1 AND ship_id = $2
ORDER BY repair_id ASC
`, int64(ownerID), int64(shipID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]uint32, 0)
	for rows.Next() {
		var repairID uint32
		if err := rows.Scan(&repairID); err != nil {
			return nil, err
		}
		result = append(result, repairID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func AddOwnedShipMetaRepairTx(ctx context.Context, tx pgx.Tx, ownerID uint32, shipID uint32, repairID uint32) error {
	_, err := tx.Exec(ctx, `
INSERT INTO owned_ship_meta_repairs (owner_id, ship_id, repair_id)
VALUES ($1, $2, $3)
ON CONFLICT (owner_id, ship_id, repair_id)
DO NOTHING
`, int64(ownerID), int64(shipID), int64(repairID))
	return err
}
