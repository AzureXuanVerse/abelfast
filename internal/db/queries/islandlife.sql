-- name: GetIsland :one
SELECT * FROM commander_islands WHERE commander_id = ?;

-- name: SaveIsland :exec
INSERT INTO commander_islands (commander_id, island, position, extra, invite_code) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	island = excluded.island, position = excluded.position, extra = excluded.extra,
	invite_code = excluded.invite_code;

-- name: GetIslandByInviteCode :one
SELECT commander_id FROM commander_islands WHERE invite_code = ? AND invite_code != '';
