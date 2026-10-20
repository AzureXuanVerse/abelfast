-- name: GetCityRebuild :one
SELECT * FROM commander_city_rebuild WHERE commander_id = ? AND activity_id = ?;

-- name: CreateCityRebuild :exec
INSERT INTO commander_city_rebuild (commander_id, activity_id) VALUES (?, ?)
ON CONFLICT (commander_id, activity_id) DO NOTHING;

-- name: SetCityRebuild :exec
UPDATE commander_city_rebuild SET
	pt = sqlc.arg(pt), summary_pt = sqlc.arg(summary_pt), max_level = sqlc.arg(max_level), cur_level = sqlc.arg(cur_level),
	max_display = sqlc.arg(max_display), fight_time = sqlc.arg(fight_time), left_hp = sqlc.arg(left_hp),
	awarded_level = sqlc.arg(awarded_level)
WHERE commander_id = sqlc.arg(commander_id) AND activity_id = sqlc.arg(activity_id);

-- name: AddCityRebuildPt :exec
UPDATE commander_city_rebuild SET pt = pt + ? WHERE commander_id = ? AND activity_id = ?;

-- name: ListCityRebuildUnits :many
SELECT unit_id, done, start_time FROM commander_city_rebuild_units
WHERE commander_id = ? AND activity_id = ? ORDER BY unit_id;

-- name: SetCityRebuildUnit :exec
INSERT INTO commander_city_rebuild_units (commander_id, activity_id, unit_id, done, start_time) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, unit_id) DO UPDATE SET done = excluded.done, start_time = excluded.start_time;

-- name: ListCityRebuildBuffs :many
SELECT buff_group, level FROM commander_city_rebuild_buffs WHERE commander_id = ? AND activity_id = ?;

-- name: SetCityRebuildBuff :exec
INSERT INTO commander_city_rebuild_buffs (commander_id, activity_id, buff_group, level) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, buff_group) DO UPDATE SET level = excluded.level;

-- name: SpendCityRebuildItem :execrows
UPDATE commander_virtual_items SET amount = amount - sqlc.arg(amount)
WHERE commander_id = sqlc.arg(commander_id) AND item_id = sqlc.arg(item_id) AND amount >= sqlc.arg(amount);
