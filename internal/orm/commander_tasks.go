package orm

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	TaskProgressUpdate = uint32(0)
	TaskProgressAppend = uint32(1)
)

type CommanderTask struct {
	CommanderID uint32 `gorm:"primaryKey;autoIncrement:false"`
	TaskID      uint32 `gorm:"primaryKey;autoIncrement:false"`
	Progress    uint32 `gorm:"not null;default:0"`
	AcceptTime  uint32 `gorm:"not null;default:0"`
	SubmitTime  uint32 `gorm:"not null;default:0"`
}

func (CommanderTask) TableName() string {
	return "commander_tasks"
}

func ListCommanderTasks(commanderID uint32) ([]CommanderTask, error) {
	if db.DefaultStore == nil {
		return nil, errors.New("db not initialized")
	}
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, task_id, progress, accept_time, submit_time
FROM commander_tasks
WHERE commander_id = $1
ORDER BY task_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CommanderTask, 0)
	for rows.Next() {
		var entry CommanderTask
		if err := rows.Scan(&entry.CommanderID, &entry.TaskID, &entry.Progress, &entry.AcceptTime, &entry.SubmitTime); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func GetCommanderTaskTx(ctx context.Context, tx pgx.Tx, commanderID uint32, taskID uint32) (*CommanderTask, error) {
	row := tx.QueryRow(ctx, `
SELECT commander_id, task_id, progress, accept_time, submit_time
FROM commander_tasks
WHERE commander_id = $1 AND task_id = $2
`, int64(commanderID), int64(taskID))
	var task CommanderTask
	err := row.Scan(&task.CommanderID, &task.TaskID, &task.Progress, &task.AcceptTime, &task.SubmitTime)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return &task, nil
}

func UpsertCommanderTaskProgressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, taskID uint32, mode uint32, progress uint32, targetNum uint32, now uint32) error {
	if mode != TaskProgressUpdate && mode != TaskProgressAppend {
		return fmt.Errorf("unsupported progress mode %d", mode)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time, submit_time)
VALUES (
  $1,
  $2,
  CASE WHEN $5 > 0 THEN LEAST($4, $5) ELSE $4 END,
  $6,
  0
)
ON CONFLICT (commander_id, task_id)
DO UPDATE SET progress = CASE
  WHEN $3 = 0 THEN CASE WHEN $5 > 0 THEN LEAST($4, $5) ELSE $4 END
  ELSE CASE WHEN $5 > 0 THEN LEAST(commander_tasks.progress + $4, $5) ELSE commander_tasks.progress + $4 END
END
`, int64(commanderID), int64(taskID), int64(mode), int64(progress), int64(targetNum), int64(now))
	return err
}

func MarkCommanderTaskSubmittedTx(ctx context.Context, tx pgx.Tx, commanderID uint32, taskID uint32, submitTime uint32) (bool, error) {
	res, err := tx.Exec(ctx, `
UPDATE commander_tasks
SET submit_time = $3
WHERE commander_id = $1
  AND task_id = $2
  AND submit_time = 0
`, int64(commanderID), int64(taskID), int64(submitTime))
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}
