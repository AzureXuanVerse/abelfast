-- name: CreateShip :one
INSERT INTO ships (commander_id, template_id, level, exp, energy, intimacy, max_level, is_locked, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListShipsByCommander :many
SELECT * FROM ships WHERE commander_id = ? ORDER BY id;

-- name: GetShip :one
SELECT * FROM ships WHERE id = ? AND commander_id = ?;

-- name: SetShipLocked :exec
UPDATE ships SET is_locked = ? WHERE id = ? AND commander_id = ?;

-- name: SetShipEquipment :exec
INSERT INTO ship_equipments (ship_id, pos, equip_id, skin_id) VALUES (?, ?, ?, ?)
ON CONFLICT (ship_id, pos) DO UPDATE SET equip_id = excluded.equip_id, skin_id = excluded.skin_id;

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

-- name: ListEquipments :many
SELECT equip_id, count FROM commander_equipments WHERE commander_id = ? AND count > 0 ORDER BY equip_id;

-- name: AddEquipment :exec
INSERT INTO commander_equipments (commander_id, equip_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, equip_id) DO UPDATE SET count = count + excluded.count;

-- name: SpendEquipment :execrows
UPDATE commander_equipments SET count = count - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND equip_id = sqlc.arg(equip_id) AND count >= sqlc.arg(count);

-- name: SpendItem :execrows
UPDATE commander_items SET count = count - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND item_id = sqlc.arg(item_id) AND count >= sqlc.arg(count);

-- name: SpendResource :execrows
UPDATE commander_resources SET amount = amount - sqlc.arg(amount)
WHERE commander_id = sqlc.arg(commander_id) AND resource_id = sqlc.arg(resource_id) AND amount >= sqlc.arg(amount);

-- name: CreateBuild :one
INSERT INTO builds (commander_id, pool_id, template_id, duration, start_time, finish_time)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListBuilds :many
SELECT * FROM builds WHERE commander_id = ? ORDER BY id;

-- name: SetBuildTimes :exec
UPDATE builds SET start_time = ?, finish_time = ? WHERE id = ?;

-- name: DeleteBuild :exec
DELETE FROM builds WHERE id = ?;

-- name: GetBuildState :one
SELECT * FROM commander_build_states WHERE commander_id = ?;

-- name: AddBuildCounts :exec
INSERT INTO commander_build_states (commander_id, build_count, exchange_count) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	build_count = build_count + excluded.build_count,
	exchange_count = exchange_count + excluded.exchange_count;

-- name: UpsertShipGroup :exec
INSERT INTO commander_ship_groups (commander_id, group_id, star, intimacy_max, lv_max) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET
	star = MAX(star, excluded.star),
	intimacy_max = MAX(intimacy_max, excluded.intimacy_max),
	lv_max = MAX(lv_max, excluded.lv_max);

-- name: ListShipGroups :many
SELECT * FROM commander_ship_groups WHERE commander_id = ? ORDER BY rowid;

-- name: CountShipGroups :one
SELECT COUNT(*) FROM commander_ship_groups WHERE commander_id = ?;

-- name: ListTrophies :many
SELECT * FROM commander_trophies WHERE commander_id = ?;

-- name: SetTrophy :exec
INSERT INTO commander_trophies (commander_id, trophy_id, progress, timestamp) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, trophy_id) DO UPDATE SET progress = excluded.progress, timestamp = excluded.timestamp;
