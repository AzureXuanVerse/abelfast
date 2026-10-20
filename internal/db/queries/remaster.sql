-- name: GetRemasterState :one
SELECT * FROM remaster_states WHERE commander_id = ?;

-- name: SaveRemasterState :exec
INSERT INTO remaster_states (commander_id, tickets, claimed_day) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET tickets = excluded.tickets, claimed_day = excluded.claimed_day;

-- name: ListRemasterAwards :many
SELECT * FROM remaster_awards WHERE commander_id = ? ORDER BY act_id, chapter_id, pos;

-- name: SaveRemasterAward :exec
INSERT INTO remaster_awards (commander_id, act_id, chapter_id, pos, count, received) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, act_id, chapter_id, pos) DO UPDATE SET count = excluded.count, received = excluded.received;
