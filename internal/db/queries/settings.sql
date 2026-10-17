-- name: GetSetting :one
SELECT value FROM settings WHERE key = ?;

-- name: InsertSettingIfMissing :exec
INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO NOTHING;
