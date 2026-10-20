-- name: ListAppreciation :many
SELECT kind, item_id FROM commander_appreciation WHERE commander_id = ? ORDER BY rowid;

-- name: AddAppreciation :execrows
INSERT INTO commander_appreciation (commander_id, kind, item_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: RemoveAppreciation :exec
DELETE FROM commander_appreciation WHERE commander_id = ? AND kind = ? AND item_id = ?;

-- name: CountAppreciation :one
SELECT COUNT(*) FROM commander_appreciation WHERE commander_id = ? AND kind = ? AND item_id = ?;

-- name: GetMusicPlayer :one
SELECT music_no, music_mode FROM commander_music_player WHERE commander_id = ?;

-- name: SetMusicPlayer :exec
INSERT INTO commander_music_player (commander_id, music_no, music_mode) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET music_no = excluded.music_no, music_mode = excluded.music_mode;
