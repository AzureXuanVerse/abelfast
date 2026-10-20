-- name: ListPermanentActivities :many
SELECT * FROM commander_permanent_activities WHERE commander_id = ? ORDER BY activity_id;

-- name: StartPermanentActivity :exec
INSERT INTO commander_permanent_activities (commander_id, activity_id, running) VALUES (?, ?, 1)
ON CONFLICT (commander_id, activity_id) DO UPDATE SET running = 1;

-- name: StopPermanentActivity :execrows
UPDATE commander_permanent_activities SET running = 0
WHERE commander_id = ? AND activity_id = ? AND running;

-- name: FinishPermanentActivity :execrows
UPDATE commander_permanent_activities SET running = 0, finished = 1
WHERE commander_id = ? AND activity_id = ? AND running;

-- name: ListRemasters :many
SELECT * FROM commander_remasters WHERE commander_id = ? ORDER BY activated_at, remaster_id;

-- name: ActivateRemaster :exec
INSERT INTO commander_remasters (commander_id, remaster_id, timer_id, activated_at) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, remaster_id) DO UPDATE SET timer_id = excluded.timer_id, activated_at = excluded.activated_at;
