-- name: ListTasks :many
SELECT task_id, progress, accept_time FROM commander_tasks
WHERE commander_id = ? AND submit_time = 0 ORDER BY rowid;

-- name: GetTask :one
SELECT * FROM commander_tasks WHERE commander_id = ? AND task_id = ?;

-- name: CreateTask :execrows
INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time) VALUES (?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: SetTaskProgress :exec
UPDATE commander_tasks SET progress = ? WHERE commander_id = ? AND task_id = ?;

-- name: SubmitTask :execrows
UPDATE commander_tasks SET submit_time = ? WHERE commander_id = ? AND task_id = ? AND submit_time = 0;

-- name: ListWeeklyTasks :many
SELECT task_id, progress FROM commander_weekly_tasks WHERE commander_id = ? ORDER BY task_id;

-- name: CreateWeeklyTask :exec
INSERT INTO commander_weekly_tasks (commander_id, task_id, progress) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: SetWeeklyTaskProgress :exec
UPDATE commander_weekly_tasks SET progress = ? WHERE commander_id = ? AND task_id = ?;

-- name: GetWeeklyRewardLevel :one
SELECT reward_lv FROM commander_weekly WHERE commander_id = ?;

-- name: SetWeeklyRewardLevel :exec
INSERT INTO commander_weekly (commander_id, reward_lv) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET reward_lv = excluded.reward_lv;

-- name: ListActivityTasks :many
SELECT act_id, task_id, progress, finished FROM commander_activity_tasks WHERE commander_id = ?;

-- name: SetActivityTaskProgress :exec
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, act_id, task_id) DO UPDATE SET progress = excluded.progress;

-- name: FinishActivityTask :exec
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, finished) VALUES (?, ?, ?, ?, 1)
ON CONFLICT (commander_id, act_id, task_id) DO UPDATE SET finished = 1;

-- name: ListTaskCounters :many
SELECT sub_type, target, value FROM commander_task_counters WHERE commander_id = ?;

-- name: GetTaskCounter :one
SELECT value FROM commander_task_counters WHERE commander_id = ? AND sub_type = ? AND target = ?;

-- name: SetTaskCounter :exec
INSERT INTO commander_task_counters (commander_id, sub_type, target, value) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, sub_type, target) DO UPDATE SET value = excluded.value;
