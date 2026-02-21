package orm

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type CommanderChargeSuccessEvent struct {
	CommanderID uint32
	PayID       string
}

func (CommanderChargeSuccessEvent) TableName() string {
	return "commander_charge_success_events"
}

func TryRecordChargeSuccessEventTx(ctx context.Context, tx pgx.Tx, commanderID uint32, payID string) (bool, error) {
	result, err := tx.Exec(ctx, `
INSERT INTO commander_charge_success_events (commander_id, pay_id)
VALUES ($1, $2)
ON CONFLICT (commander_id, pay_id) DO NOTHING
`, int64(commanderID), payID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}
