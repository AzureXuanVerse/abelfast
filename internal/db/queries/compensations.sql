-- name: CreateCompensation :one
INSERT INTO compensations (commander_id, date, expires, title, sender, content, attachments)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: ListCompensations :many
SELECT * FROM compensations
WHERE commander_id = sqlc.arg(commander_id) AND expires > sqlc.arg(now)
ORDER BY id;

-- name: GetCompensation :one
SELECT * FROM compensations WHERE id = ? AND commander_id = ?;

-- name: SetCompensationClaimed :exec
UPDATE compensations SET is_claimed = 1 WHERE id = ? AND commander_id = ?;
