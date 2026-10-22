-- name: AdminListInnerAccounts :many
SELECT a.username, a.mail_box, a.account_id, a.registered_at,
	CAST(COALESCE(c.id, 0) AS INTEGER) AS commander_id,
	CAST(COALESCE(c.name, '') AS TEXT) AS commander_name,
	CAST(COALESCE(c.level, 0) AS INTEGER) AS commander_level,
	CAST(p.commander_id IS NOT NULL AS BOOLEAN) AS secondary_set,
	CAST(COALESCE(p.fail_cd, 0) AS INTEGER) AS secondary_fail_cd
FROM inner_accounts a
LEFT JOIN commanders c ON c.account_id = a.account_id
LEFT JOIN commander_secondary_passwords p ON p.commander_id = c.id
ORDER BY a.registered_at, a.username;

-- name: AdminGetInnerAccount :one
SELECT * FROM inner_accounts WHERE account_id = ?;

-- name: AdminSetInnerPassword :exec
UPDATE inner_accounts SET password_hash = ? WHERE account_id = ?;

-- name: AdminSetInnerMailBox :exec
UPDATE inner_accounts SET mail_box = ? WHERE account_id = ?;

-- name: AdminDeleteInnerAccount :exec
DELETE FROM inner_accounts WHERE account_id = ?;

-- name: AdminCommanderByAccount :one
SELECT id, name, level FROM commanders WHERE account_id = ?;

-- name: AdminCountCommandersOfAccount :one
SELECT COUNT(*) FROM commanders WHERE account_id = ?;

-- name: AdminSetCommanderAccount :exec
UPDATE commanders SET account_id = ? WHERE id = ?;

-- name: AdminListRedemptions :many
SELECT code, redeemed_at FROM exchange_code_redemptions WHERE commander_id = ? ORDER BY redeemed_at DESC, code;

-- name: AdminDeleteRedemption :execrows
DELETE FROM exchange_code_redemptions WHERE commander_id = ? AND code = ?;
