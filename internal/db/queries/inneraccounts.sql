-- name: GetInnerAccount :one
SELECT * FROM inner_accounts WHERE username = ?;

-- name: CreateInnerAccount :execrows
INSERT INTO inner_accounts (username, password_hash, mail_box, account_id, registered_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;
