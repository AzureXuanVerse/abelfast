-- name: GetWorldState :one
SELECT state FROM world_states WHERE commander_id = ?;

-- name: SaveWorldState :exec
INSERT INTO world_states (commander_id, state) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET state = excluded.state;

-- name: DeleteWorldState :exec
DELETE FROM world_states WHERE commander_id = ?;

-- name: GetWorldMap :one
SELECT state FROM world_maps WHERE commander_id = ? AND random_id = ?;

-- name: SaveWorldMap :exec
INSERT INTO world_maps (commander_id, random_id, state) VALUES (?, ?, ?)
ON CONFLICT (commander_id, random_id) DO UPDATE SET state = excluded.state;

-- name: DeleteWorldMap :exec
DELETE FROM world_maps WHERE commander_id = ? AND random_id = ?;

-- name: DeleteWorldMaps :exec
DELETE FROM world_maps WHERE commander_id = ?;
