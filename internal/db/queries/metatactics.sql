-- name: ListMetaTactics :many
SELECT t.* FROM ship_meta_tactics t
JOIN ships s ON s.id = t.ship_id
WHERE s.commander_id = ? AND t.ship_id IN (sqlc.slice(ids));

-- name: SetMetaTactics :exec
INSERT INTO ship_meta_tactics (ship_id, skill_id, day, day_exp, double_exp, switches)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (ship_id) DO UPDATE SET skill_id = excluded.skill_id, day = excluded.day,
	day_exp = excluded.day_exp, double_exp = excluded.double_exp, switches = excluded.switches;

-- name: ListShipSkillsByID :many
SELECT k.ship_id, k.skill_id, k.level, k.exp FROM ship_skills k
JOIN ships s ON s.id = k.ship_id
WHERE s.commander_id = ? AND k.ship_id IN (sqlc.slice(ids)) ORDER BY k.ship_id, k.rowid;

-- name: ListMetaRepairsByCommander :many
SELECT r.ship_id, r.repair_id FROM ship_meta_repairs r
JOIN ships s ON s.id = r.ship_id
WHERE s.commander_id = ? ORDER BY r.ship_id, r.repair_id;

-- name: ListMetaRepairs :many
SELECT repair_id FROM ship_meta_repairs WHERE ship_id = ? ORDER BY repair_id;

-- name: AddMetaRepair :exec
INSERT INTO ship_meta_repairs (ship_id, repair_id) VALUES (?, ?) ON CONFLICT DO NOTHING;
