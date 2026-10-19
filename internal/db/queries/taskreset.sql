-- name: GetWeeklyWeekStart :one
SELECT week_start FROM commander_weekly WHERE commander_id = ?;

-- name: SetWeeklyWeekStart :exec
INSERT INTO commander_weekly (commander_id, reward_lv, week_start) VALUES (?, 0, ?)
ON CONFLICT (commander_id) DO UPDATE SET week_start = excluded.week_start;

-- name: ResetWeekly :exec
UPDATE commander_weekly SET reward_lv = 0, week_start = ? WHERE commander_id = ?;

-- name: DeleteWeeklyTasks :exec
DELETE FROM commander_weekly_tasks WHERE commander_id = ?;

-- name: ClearVirtualItem :exec
UPDATE commander_virtual_items SET amount = 0 WHERE commander_id = ? AND item_id = ?;

-- name: DeleteTasks :exec
DELETE FROM commander_tasks WHERE commander_id = sqlc.arg(commander_id) AND task_id IN (sqlc.slice(task_ids));
