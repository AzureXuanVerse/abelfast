-- name: CreateCheaterMark :exec
INSERT INTO cheater_marks (commander_id, account_id, reason, marked_at)
VALUES (?, ?, ?, ?);
