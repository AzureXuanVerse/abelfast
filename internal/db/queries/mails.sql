-- name: CreateMail :one
INSERT INTO mails (commander_id, date, title, sender, content, attachments)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CountInboxMails :one
SELECT COUNT(*) FROM mails WHERE commander_id = ? AND archived = 0;

-- name: CountUnreadMails :one
SELECT COUNT(*) FROM mails WHERE commander_id = ? AND archived = 0 AND is_read = 0;

-- name: CountArchivedMails :one
SELECT COUNT(*) FROM mails WHERE commander_id = ? AND archived = 1;

-- name: ListInboxMails :many
SELECT * FROM mails WHERE commander_id = ? AND archived = 0 ORDER BY id;

-- name: ListInboxMailsPage :many
SELECT * FROM mails WHERE commander_id = sqlc.arg(commander_id) AND archived = 0
ORDER BY id LIMIT sqlc.arg(count) OFFSET sqlc.arg(skip);

-- name: ListArchivedMailsPage :many
SELECT * FROM mails WHERE commander_id = sqlc.arg(commander_id) AND archived = 1
ORDER BY id LIMIT sqlc.arg(count) OFFSET sqlc.arg(skip);

-- name: ListMailsByID :many
SELECT * FROM mails WHERE commander_id = sqlc.arg(commander_id) AND id IN (sqlc.slice(ids)) ORDER BY id;

-- name: SetMailFlags :exec
UPDATE mails SET is_read = ?, is_claimed = ?, is_important = ?, archived = ?
WHERE id = ? AND commander_id = ?;

-- name: DeleteMail :exec
DELETE FROM mails WHERE id = ? AND commander_id = ?;

-- name: DeleteArchivedMail :execrows
DELETE FROM mails WHERE id = ? AND commander_id = ? AND archived = 1;

-- name: ListCommanderIDs :many
SELECT id FROM commanders ORDER BY id;
