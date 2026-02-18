package answer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func loadGameRoomResourceAmountForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, resourceID uint32) (uint32, error) {
	var amount int64
	err := tx.QueryRow(ctx, `
SELECT amount
FROM owned_resources
WHERE commander_id = $1
	AND resource_id = $2
FOR UPDATE
`, int64(commanderID), int64(resourceID)).Scan(&amount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return uint32(amount), nil
}
