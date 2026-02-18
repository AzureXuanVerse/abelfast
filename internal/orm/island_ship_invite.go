package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

func ListIslandShipInvites(commanderID uint32) ([]uint32, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT ship_id
FROM island_ship_invites
WHERE commander_id = $1
ORDER BY ship_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invites := make([]uint32, 0)
	for rows.Next() {
		var shipIDRaw int64
		if err := rows.Scan(&shipIDRaw); err != nil {
			return nil, err
		}
		invites = append(invites, uint32(shipIDRaw))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invites, nil
}

func AddIslandShipInvite(commanderID uint32, shipID uint32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_ship_invites (commander_id, ship_id)
VALUES ($1, $2)
ON CONFLICT (commander_id, ship_id) DO NOTHING
`, int64(commanderID), int64(shipID))
	return err
}

func HasIslandShipInviteTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM island_ship_invites
	WHERE commander_id = $1 AND ship_id = $2
)
`, int64(commanderID), int64(shipID)).Scan(&exists)
	return exists, err
}

func DeleteIslandShipInviteTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipID uint32) error {
	_, err := tx.Exec(ctx, `
DELETE FROM island_ship_invites
WHERE commander_id = $1 AND ship_id = $2
`, int64(commanderID), int64(shipID))
	return err
}
