-- name: CreateNotice :one
INSERT INTO notices (kind, version, btn_title, title, title_image, time_desc, content, tag_type, icon, track, priority, need_level)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetNotice :one
SELECT * FROM notices WHERE id = ?;

-- name: ListNotices :many
SELECT * FROM notices ORDER BY kind, priority DESC, id DESC;

-- name: ListNoticesByKind :many
SELECT * FROM notices WHERE kind = ? ORDER BY priority DESC, id DESC;

-- name: UpdateNotice :exec
UPDATE notices SET kind = ?, version = ?, btn_title = ?, title = ?, title_image = ?, time_desc = ?,
	content = ?, tag_type = ?, icon = ?, track = ?, priority = ?, need_level = ?
WHERE id = ?;

-- name: DeleteNotice :execrows
DELETE FROM notices WHERE id = ?;
