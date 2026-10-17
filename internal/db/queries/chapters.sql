-- name: ListChapterProgress :many
SELECT * FROM chapter_progress WHERE commander_id = ? ORDER BY chapter_id;

-- name: GetChapterProgress :one
SELECT * FROM chapter_progress WHERE commander_id = ? AND chapter_id = ?;

-- name: SaveChapterProgress :exec
INSERT INTO chapter_progress (commander_id, chapter_id, progress, kill_boss_count, kill_enemy_count, take_box_count, defeat_count, today_defeat_count, pass_count)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (commander_id, chapter_id) DO UPDATE SET
	progress = excluded.progress,
	kill_boss_count = excluded.kill_boss_count,
	kill_enemy_count = excluded.kill_enemy_count,
	take_box_count = excluded.take_box_count,
	defeat_count = excluded.defeat_count,
	today_defeat_count = excluded.today_defeat_count,
	pass_count = excluded.pass_count;

-- name: GetChapterState :one
SELECT * FROM chapter_states WHERE commander_id = ?;

-- name: SaveChapterState :exec
INSERT INTO chapter_states (commander_id, state, ambushed) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET state = excluded.state, ambushed = excluded.ambushed;

-- name: DeleteChapterState :exec
DELETE FROM chapter_states WHERE commander_id = ?;
