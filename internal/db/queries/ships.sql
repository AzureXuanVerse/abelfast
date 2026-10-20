-- name: CreateShip :one
INSERT INTO ships (commander_id, template_id, level, exp, energy, intimacy, max_level, is_locked, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetShip :one
SELECT * FROM ships WHERE id = ? AND commander_id = ?;

-- name: ListShipsByCommander :many
SELECT * FROM ships WHERE commander_id = ? ORDER BY id;

-- name: ListShipsByID :many
SELECT * FROM ships WHERE commander_id = ? AND id IN (sqlc.slice(ids)) ORDER BY id;

-- name: CountOwnedShips :one
SELECT COUNT(*) FROM ships WHERE commander_id = ? AND id IN (sqlc.slice(ids));

-- name: DeleteShip :exec
DELETE FROM ships WHERE id = ? AND commander_id = ?;

-- name: SetShipLocked :exec
UPDATE ships SET is_locked = ? WHERE id = ? AND commander_id = ?;

-- name: UpdateShipBattleState :exec
UPDATE ships SET level = ?, exp = ?, intimacy = ?, energy = ? WHERE id = ? AND commander_id = ?;

-- name: SetShipEquipment :exec
INSERT INTO ship_equipments (ship_id, pos, equip_id, skin_id) VALUES (?, ?, ?, ?)
ON CONFLICT (ship_id, pos) DO UPDATE SET equip_id = excluded.equip_id, skin_id = excluded.skin_id;

-- name: ListShipEquipments :many
SELECT * FROM ship_equipments WHERE ship_id = ? ORDER BY pos;

-- name: ListShipEquipmentsByCommander :many
SELECT e.ship_id, e.pos, e.equip_id, e.skin_id FROM ship_equipments e
JOIN ships s ON s.id = e.ship_id
WHERE s.commander_id = ? ORDER BY e.ship_id, e.pos;

-- name: AddShipSkill :exec
INSERT INTO ship_skills (ship_id, skill_id, level, exp) VALUES (?, ?, ?, 0) ON CONFLICT DO NOTHING;

-- name: ListShipSkillsByCommander :many
SELECT k.ship_id, k.skill_id, k.level, k.exp FROM ship_skills k
JOIN ships s ON s.id = k.ship_id
WHERE s.commander_id = ? ORDER BY k.ship_id, k.rowid;

-- name: AddShipGroup :execrows
INSERT INTO commander_ship_groups (commander_id, group_id, intimacy_max, lv_max, marry_flag, star) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: RaiseShipGroup :execrows
UPDATE commander_ship_groups SET
	star = CASE WHEN star < sqlc.arg(star) THEN sqlc.arg(star) ELSE star END,
	intimacy_max = CASE WHEN intimacy_max < sqlc.arg(intimacy_max) THEN sqlc.arg(intimacy_max) ELSE intimacy_max END,
	lv_max = CASE WHEN lv_max < sqlc.arg(lv_max) THEN sqlc.arg(lv_max) ELSE lv_max END,
	marry_flag = CASE WHEN marry_flag < sqlc.arg(marry_flag) THEN sqlc.arg(marry_flag) ELSE marry_flag END
WHERE commander_id = sqlc.arg(commander_id) AND group_id = sqlc.arg(group_id)
AND (star < sqlc.arg(star) OR intimacy_max < sqlc.arg(intimacy_max) OR lv_max < sqlc.arg(lv_max) OR marry_flag < sqlc.arg(marry_flag));

-- name: ListShipGroups :many
SELECT * FROM commander_ship_groups WHERE commander_id = ? ORDER BY rowid;

-- name: CountShipGroups :one
SELECT COUNT(*) FROM commander_ship_groups WHERE commander_id = ?;

-- name: ListTrophies :many
SELECT * FROM commander_trophies WHERE commander_id = ?;

-- name: SetTrophy :exec
INSERT INTO commander_trophies (commander_id, trophy_id, progress, timestamp) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, trophy_id) DO UPDATE SET progress = excluded.progress, timestamp = excluded.timestamp;

-- name: SetShipSkin :exec
UPDATE ships SET skin_id = ? WHERE id = ? AND commander_id = ?;

-- name: CountShipsByCommander :one
SELECT COUNT(*) FROM ships WHERE commander_id = ?;
