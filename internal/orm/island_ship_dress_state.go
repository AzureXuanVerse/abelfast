package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandShipDressState struct {
	CommanderID uint32
	ShipID      uint32
	DressID     uint32
}

func ListIslandShipDressStates(commanderID uint32) ([]IslandShipDressState, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, ship_id, dress_id
FROM island_ship_dresses
WHERE commander_id = $1
ORDER BY ship_id, dress_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]IslandShipDressState, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var shipIDRaw int64
		var dressIDRaw int64
		if err := rows.Scan(&commanderIDRaw, &shipIDRaw, &dressIDRaw); err != nil {
			return nil, err
		}
		states = append(states, IslandShipDressState{
			CommanderID: uint32(commanderIDRaw),
			ShipID:      uint32(shipIDRaw),
			DressID:     uint32(dressIDRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func RemoveIslandShipDressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32, dressID uint32) (bool, error) {
	result, err := tx.Exec(ctx, `
DELETE FROM island_ship_dresses
WHERE commander_id = $1 AND ship_id = $2 AND dress_id = $3
`, int64(commanderID), int64(shipID), int64(dressID))
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func RemoveIslandShipDressByDressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, dressID uint32) (uint32, error) {
	var sourceShipIDRaw int64
	err := tx.QueryRow(ctx, `
SELECT ship_id
FROM island_ship_dresses
WHERE commander_id = $1 AND dress_id = $2
LIMIT 1
`, int64(commanderID), int64(dressID)).Scan(&sourceShipIDRaw)
	err = db.MapNotFound(err)
	if err != nil {
		if db.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	_, err = tx.Exec(ctx, `
DELETE FROM island_ship_dresses
WHERE commander_id = $1 AND dress_id = $2
`, int64(commanderID), int64(dressID))
	if err != nil {
		return 0, err
	}
	return uint32(sourceShipIDRaw), nil
}

func UpsertIslandShipDressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32, dressID uint32) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_ship_dresses (commander_id, ship_id, dress_id)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, ship_id, dress_id) DO NOTHING
`, int64(commanderID), int64(shipID), int64(dressID))
	return err
}
