package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func ConsumeOwnedShipEnergyTx(ctx context.Context, tx pgx.Tx, commanderID uint32, ownedShipID uint32, cost uint32) (uint32, error) {
	if cost == 0 {
		ship, err := GetOwnedShipByOwnerAndID(commanderID, ownedShipID)
		if err != nil {
			return 0, err
		}
		return ship.Energy, nil
	}
	var energyRaw int64
	err := tx.QueryRow(ctx, `
UPDATE owned_ships
SET energy = energy - $3
WHERE owner_id = $1 AND id = $2 AND energy >= $3 AND deleted_at IS NULL
RETURNING energy
`, int64(commanderID), int64(ownedShipID), int64(cost)).Scan(&energyRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return 0, err
	}
	return uint32(energyRaw), nil
}
