package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandSpeedupTarget struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	TargetType  uint32 `gorm:"primaryKey;column:target_type"`
	TargetID    uint32 `gorm:"primaryKey;column:target_id"`
	EndTime     uint32 `gorm:"column:end_time"`
}

func (IslandSpeedupTarget) TableName() string {
	return "island_speedup_targets"
}

func UpsertIslandSpeedupTarget(commanderID uint32, targetType uint32, targetID uint32, endTime uint32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_speedup_targets (commander_id, target_type, target_id, end_time)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id, target_type, target_id)
DO UPDATE SET end_time = EXCLUDED.end_time
`, int64(commanderID), int64(targetType), int64(targetID), int64(endTime))
	return err
}

func ReduceIslandSpeedupTargetTx(ctx context.Context, tx pgx.Tx, commanderID uint32, targetType uint32, targetID uint32, now uint32, reduceBy uint32) (uint32, error) {
	var endTimeRaw int64
	err := tx.QueryRow(ctx, `
SELECT end_time
FROM island_speedup_targets
WHERE commander_id = $1 AND target_type = $2 AND target_id = $3
FOR UPDATE
`, int64(commanderID), int64(targetType), int64(targetID)).Scan(&endTimeRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return 0, err
	}
	endTime := uint32(endTimeRaw)
	if endTime <= now {
		return 0, db.ErrNotFound
	}
	newEndTime := endTime
	if reduceBy >= endTime-now {
		newEndTime = now
	} else {
		newEndTime = endTime - reduceBy
	}
	_, err = tx.Exec(ctx, `
UPDATE island_speedup_targets
SET end_time = $4
WHERE commander_id = $1 AND target_type = $2 AND target_id = $3
`, int64(commanderID), int64(targetType), int64(targetID), int64(newEndTime))
	if err != nil {
		return 0, err
	}
	return newEndTime, nil
}
