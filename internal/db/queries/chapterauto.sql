-- name: ListChapterAutoRecords :many
SELECT * FROM chapter_auto_records WHERE commander_id = ? ORDER BY type, chapter_id;

-- name: GetChapterAutoRecord :one
SELECT seconds FROM chapter_auto_records WHERE commander_id = ? AND type = ? AND chapter_id = ?;

-- name: SaveChapterAutoRecord :exec
INSERT INTO chapter_auto_records (commander_id, type, chapter_id, seconds) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, type, chapter_id) DO UPDATE SET seconds = min(seconds, excluded.seconds);

-- name: ListChapterAutoTickets :many
SELECT * FROM chapter_auto_tickets WHERE commander_id = ? AND count > 0 ORDER BY type, expire_at;

-- name: AddChapterAutoTickets :exec
INSERT INTO chapter_auto_tickets (commander_id, type, expire_at, count) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, type, expire_at) DO UPDATE SET count = count + excluded.count;

-- name: SetChapterAutoTicketCount :exec
UPDATE chapter_auto_tickets SET count = ? WHERE commander_id = ? AND type = ? AND expire_at = ?;

-- name: DeleteExpiredChapterAutoTickets :exec
DELETE FROM chapter_auto_tickets WHERE commander_id = ? AND (expire_at < ? OR count = 0);

-- name: ListChapterAutoBattles :many
SELECT * FROM chapter_auto_battles WHERE commander_id = ? ORDER BY seq;

-- name: AddChapterAutoBattle :exec
INSERT INTO chapter_auto_battles (commander_id, seq, type, chapter_id, finish_at, ticket_expire_at, seconds)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteChapterAutoBattles :exec
DELETE FROM chapter_auto_battles WHERE commander_id = ?;

-- name: GetChapterAutoState :one
SELECT * FROM chapter_auto_states WHERE commander_id = ?;

-- name: SaveChapterAutoState :exec
INSERT INTO chapter_auto_states (commander_id, oil, time_used, extra_time, day) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	oil = excluded.oil,
	time_used = excluded.time_used,
	extra_time = excluded.extra_time,
	day = excluded.day;
