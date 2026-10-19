-- name: EnsureWorldBoss :exec
INSERT INTO world_bosses (commander_id) VALUES (?) ON CONFLICT (commander_id) DO NOTHING;

-- name: GetWorldBoss :one
SELECT * FROM world_bosses WHERE commander_id = ?;

-- name: SaveWorldBoss :exec
UPDATE world_bosses SET
	summon_pt = ?, summon_pt_old = ?, summon_pt_daily_acc = ?, summon_pt_old_daily_acc = ?, acc_day = ?,
	summon_free = ?, archives_id = ?, self_boss_lv = ?, self_boss_template_id = ?,
	boss_template_id = ?, boss_lv = ?, boss_hp = ?, boss_last_time = ?, boss_kill_time = ?,
	boss_fight_count = ?, boss_damage = ?,
	auto_fight_finish_time = ?, auto_fight_max_damage = ?,
	guild_support = ?, friend_support = ?, world_support = ?
WHERE commander_id = ?;

-- name: ListWorldBossShips :many
SELECT ship_id FROM world_boss_ships WHERE commander_id = ? ORDER BY pos;

-- name: ClearWorldBossShips :exec
DELETE FROM world_boss_ships WHERE commander_id = ?;

-- name: AddWorldBossShip :exec
INSERT INTO world_boss_ships (commander_id, pos, ship_id) VALUES (?, ?, ?);

-- name: ListMetaPts :many
SELECT * FROM meta_pts WHERE commander_id = ? ORDER BY group_id;

-- name: AddMetaPt :exec
INSERT INTO meta_pts (commander_id, group_id, pt) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id) DO UPDATE SET pt = pt + excluded.pt;

-- name: ListMetaPtAwards :many
SELECT * FROM meta_pt_awards WHERE commander_id = ? ORDER BY group_id, target;

-- name: AddMetaPtAward :exec
INSERT INTO meta_pt_awards (commander_id, group_id, target) VALUES (?, ?, ?);
