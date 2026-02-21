package orm

import (
	"context"
	"sort"

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

func ListOwnedShipMetaRepairIDsByShips(ownerID uint32, shipIDs []uint32) (map[uint32][]uint32, error) {
	if ownerID == 0 || len(shipIDs) == 0 {
		return map[uint32][]uint32{}, nil
	}
	ids := make([]int64, 0, len(shipIDs))
	seen := make(map[uint32]struct{}, len(shipIDs))
	for _, shipID := range shipIDs {
		if shipID == 0 {
			continue
		}
		if _, ok := seen[shipID]; ok {
			continue
		}
		seen[shipID] = struct{}{}
		ids = append(ids, int64(shipID))
	}
	if len(ids) == 0 {
		return map[uint32][]uint32{}, nil
	}
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT ship_id, repair_id
FROM owned_ship_meta_repairs
WHERE owner_id = $1 AND ship_id = ANY($2::bigint[])
ORDER BY ship_id ASC, repair_id ASC
`, int64(ownerID), ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uint32][]uint32)
	for rows.Next() {
		var shipID uint32
		var repairID uint32
		if err := rows.Scan(&shipID, &repairID); err != nil {
			return nil, err
		}
		result[shipID] = append(result[shipID], repairID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for shipID := range result {
		sort.Slice(result[shipID], func(i int, j int) bool {
			return result[shipID][i] < result[shipID][j]
		})
	}
	return result, nil
}
