-- name: ListHandbookPages :many
SELECT page_id FROM commander_handbook_pages WHERE commander_id = ? ORDER BY page_id;

-- name: CreateHandbookPage :exec
INSERT INTO commander_handbook_pages (commander_id, page_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: ListHandbookFinished :many
SELECT page_id, task_id FROM commander_handbook_finished WHERE commander_id = ? ORDER BY rowid;

-- name: AddHandbookFinished :execrows
INSERT INTO commander_handbook_finished (commander_id, page_id, task_id) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;
