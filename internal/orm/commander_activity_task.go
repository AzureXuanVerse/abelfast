package orm

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

var ErrInvalidActivityTaskProgressMode = errors.New("invalid activity task progress mode")

const (
	ActivityTaskProgressModeSet    = uint32(0)
	ActivityTaskProgressModeAppend = uint32(1)
)

type CommanderActivityTask struct {
	CommanderID uint32
	ActID       uint32
	TaskID      uint32
	Progress    uint32
	Submitted   bool
}

func ListCommanderActivityTasks(commanderID uint32) ([]CommanderActivityTask, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, act_id, task_id, progress, submitted
FROM commander_activity_tasks
WHERE commander_id = $1
ORDER BY act_id ASC, task_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CommanderActivityTask, 0)
	for rows.Next() {
		var row CommanderActivityTask
		if err := rows.Scan(&row.CommanderID, &row.ActID, &row.TaskID, &row.Progress, &row.Submitted); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func GetCommanderActivityTask(commanderID uint32, actID uint32, taskID uint32) (*CommanderActivityTask, error) {
	row := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, act_id, task_id, progress, submitted
FROM commander_activity_tasks
WHERE commander_id = $1 AND act_id = $2 AND task_id = $3
`, int64(commanderID), int64(actID), int64(taskID))

	var out CommanderActivityTask
	err := row.Scan(&out.CommanderID, &out.ActID, &out.TaskID, &out.Progress, &out.Submitted)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func TrySubmitCommanderActivityTaskTx(ctx context.Context, tx pgx.Tx, commanderID uint32, actID uint32, taskID uint32) (bool, error) {
	result, err := tx.Exec(ctx, `
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, submitted, updated_at)
VALUES ($1, $2, $3, 0, true, now())
ON CONFLICT (commander_id, act_id, task_id)
DO UPDATE SET submitted = true, updated_at = now()
WHERE commander_activity_tasks.submitted = false
`, int64(commanderID), int64(actID), int64(taskID))
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}

func UpsertCommanderActivityTaskProgressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, actID uint32, taskID uint32, mode uint32, progress uint32) error {
	switch mode {
	case ActivityTaskProgressModeSet:
		_, err := tx.Exec(ctx, `
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, submitted, updated_at)
VALUES ($1, $2, $3, $4, false, now())
ON CONFLICT (commander_id, act_id, task_id)
DO UPDATE SET progress = EXCLUDED.progress, updated_at = now()
`, int64(commanderID), int64(actID), int64(taskID), int64(progress))
		return err
	case ActivityTaskProgressModeAppend:
		_, err := tx.Exec(ctx, `
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, submitted, updated_at)
VALUES ($1, $2, $3, $4, false, now())
ON CONFLICT (commander_id, act_id, task_id)
DO UPDATE SET progress = commander_activity_tasks.progress + EXCLUDED.progress, updated_at = now()
`, int64(commanderID), int64(actID), int64(taskID), int64(progress))
		return err
	default:
		return ErrInvalidActivityTaskProgressMode
	}
}
