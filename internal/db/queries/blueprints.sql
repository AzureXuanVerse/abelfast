-- name: ListBlueprints :many
SELECT * FROM commander_blueprints WHERE commander_id = ? ORDER BY blueprint_id;

-- name: GetBlueprint :one
SELECT * FROM commander_blueprints WHERE commander_id = ? AND blueprint_id = ?;

-- name: SaveBlueprint :exec
INSERT INTO commander_blueprints (commander_id, blueprint_id, ship_id, start_time, duration)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, blueprint_id) DO UPDATE SET
	ship_id = excluded.ship_id,
	start_time = excluded.start_time,
	duration = excluded.duration;

-- name: GetBlueprintState :one
SELECT * FROM commander_blueprint_state WHERE commander_id = ?;

-- name: SaveBlueprintState :exec
INSERT INTO commander_blueprint_state (commander_id, cold_time, pursue_day, pursued, pursued_ur)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	cold_time = excluded.cold_time,
	pursue_day = excluded.pursue_day,
	pursued = excluded.pursued,
	pursued_ur = excluded.pursued_ur;

-- name: ListSubmittedTasks :many
SELECT task_id, progress, accept_time, submit_time FROM commander_tasks
WHERE commander_id = ? AND submit_time != 0 ORDER BY rowid;

-- name: DeleteShipStrengths :exec
DELETE FROM ship_strengths WHERE ship_id = ?;
