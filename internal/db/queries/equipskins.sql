-- name: ListEquipSkins :many
SELECT skin_id, count FROM commander_equip_skins WHERE commander_id = ? AND count > 0 ORDER BY skin_id;

-- name: AddEquipSkin :exec
INSERT INTO commander_equip_skins (commander_id, skin_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, skin_id) DO UPDATE SET count = count + excluded.count;

-- name: SpendEquipSkin :execrows
UPDATE commander_equip_skins SET count = count - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND skin_id = sqlc.arg(skin_id) AND count >= sqlc.arg(count);
