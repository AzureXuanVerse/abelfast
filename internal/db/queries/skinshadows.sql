-- name: AddShipSkinShadow :execrows
INSERT INTO ship_skin_shadows (ship_id, shadow_id) VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: HasShipSkinShadow :one
SELECT EXISTS (SELECT 1 FROM ship_skin_shadows WHERE ship_id = ? AND shadow_id = ?);

-- name: SetShipSkinShadowSkin :execrows
UPDATE ship_skin_shadows SET skin_id = ? WHERE ship_id = ? AND shadow_id = ?;

-- name: ListShipSkinShadowsByCommander :many
SELECT d.ship_id, d.shadow_id, d.skin_id FROM ship_skin_shadows d
JOIN ships s ON s.id = d.ship_id
WHERE s.commander_id = ? ORDER BY d.ship_id, d.shadow_id;

-- name: ResetExpiredShadowSkins :exec
UPDATE ship_skin_shadows SET skin_id = 0
WHERE ship_id IN (SELECT s.id FROM ships s WHERE s.commander_id = sqlc.arg(commander_id))
AND skin_id IN (
	SELECT k.skin_id FROM commander_skins k
	WHERE k.commander_id = sqlc.arg(commander_id) AND k.end_time > 0 AND k.end_time <= sqlc.arg(now)
);
