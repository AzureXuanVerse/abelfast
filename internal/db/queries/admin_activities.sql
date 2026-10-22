-- name: AdminActivitySetTask :exec
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, finished) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, act_id, task_id) DO UPDATE SET progress = excluded.progress, finished = excluded.finished;

-- name: AdminActivityDeleteTask :exec
DELETE FROM commander_activity_tasks WHERE commander_id = ? AND act_id = ? AND task_id = ?;

-- name: AdminActivityListTasks :many
SELECT task_id, progress, finished FROM commander_activity_tasks WHERE commander_id = ? AND act_id = ?;

-- name: AdminTaskSet :exec
UPDATE commander_tasks SET progress = ?, submit_time = ? WHERE commander_id = ? AND task_id = ?;
