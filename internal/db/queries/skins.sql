-- name: ListSkins :many
SELECT skin_id, end_time FROM commander_skins
WHERE commander_id = sqlc.arg(commander_id) AND (end_time = 0 OR end_time > sqlc.arg(now))
ORDER BY skin_id;

-- name: SetSkin :exec
INSERT INTO commander_skins (commander_id, skin_id, end_time) VALUES (?, ?, ?)
ON CONFLICT (commander_id, skin_id) DO UPDATE SET end_time = excluded.end_time;

-- name: ResetExpiredShipSkins :exec
UPDATE ships SET skin_id = 0
WHERE ships.commander_id = sqlc.arg(commander_id) AND ships.skin_id IN (
	SELECT k.skin_id FROM commander_skins k
	WHERE k.commander_id = sqlc.arg(commander_id) AND k.end_time > 0 AND k.end_time <= sqlc.arg(now)
);

-- name: DeleteExpiredSkins :exec
DELETE FROM commander_skins
WHERE commander_id = sqlc.arg(commander_id) AND end_time > 0 AND end_time <= sqlc.arg(now);
