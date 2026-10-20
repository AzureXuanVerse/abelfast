-- name: ListIslandNodes :many
SELECT * FROM commander_island_nodes WHERE commander_id = ? AND activity_id = ?;

-- name: SaveIslandNode :exec
INSERT INTO commander_island_nodes (commander_id, activity_id, node_id, event_id, is_new, refreshed_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, node_id) DO UPDATE SET
	event_id = excluded.event_id, is_new = excluded.is_new, refreshed_at = excluded.refreshed_at;
