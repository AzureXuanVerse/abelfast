-- name: ListHandbookPages :many
SELECT page_id, award FROM commander_handbook_pages WHERE commander_id = ? ORDER BY page_id;

-- name: GetHandbookPageAward :one
SELECT award FROM commander_handbook_pages WHERE commander_id = ? AND page_id = ?;

-- name: ClaimHandbookPageAward :execrows
UPDATE commander_handbook_pages SET award = award + 1
WHERE commander_id = ? AND page_id = ? AND award = ?;

-- name: CreateHandbookPage :exec
INSERT INTO commander_handbook_pages (commander_id, page_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: ListHandbookFinished :many
SELECT page_id, task_id FROM commander_handbook_finished WHERE commander_id = ? ORDER BY rowid;

-- name: AddHandbookFinished :execrows
INSERT INTO commander_handbook_finished (commander_id, page_id, task_id) VALUES (?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: ListHandbookPageFinished :many
SELECT task_id FROM commander_handbook_finished WHERE commander_id = ? AND page_id = ?;
