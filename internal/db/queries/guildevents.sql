-- name: CreateGuildOperation :one
INSERT INTO guild_operations (guild_id, operation_id, start_time, end_time, boss_id, boss_hp)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetOpenGuildOperation :one
SELECT * FROM guild_operations
WHERE guild_id = ? AND NOT ended AND end_time > sqlc.arg(now)
ORDER BY id DESC LIMIT 1;

-- name: CountGuildOperationsSince :one
SELECT COUNT(*) FROM guild_operations WHERE guild_id = ? AND start_time >= sqlc.arg(since);

-- name: ActivateGuildBoss :exec
UPDATE guild_operations SET boss_active = 1 WHERE id = ?;

-- name: DamageGuildBoss :exec
UPDATE guild_operations SET boss_damage = ?, ended = ? WHERE id = ?;

-- name: AddGuildMission :exec
INSERT INTO guild_operation_missions (operation_id, event_id, position) VALUES (?, ?, ?);

-- name: ListGuildMissions :many
SELECT * FROM guild_operation_missions WHERE operation_id = ? ORDER BY position, event_id;

-- name: SaveGuildMission :exec
UPDATE guild_operation_missions
SET start_time = ?, work = ?, updated_at = ?, efficiency = ?, finished_at = ?, score = ?
WHERE operation_id = ? AND event_id = ?;

-- name: AddGuildMissionNode :exec
INSERT INTO guild_operation_nodes (operation_id, event_id, position, node_id) VALUES (?, ?, ?, ?);

-- name: ListGuildMissionNodes :many
SELECT * FROM guild_operation_nodes WHERE operation_id = ? ORDER BY event_id, position;

-- name: SetGuildMissionNodeStatus :exec
UPDATE guild_operation_nodes SET status = ? WHERE operation_id = ? AND event_id = ? AND position = ?;

-- name: AddGuildMissionShip :exec
INSERT INTO guild_operation_ships (
	operation_id, event_id, commander_id, fleet, slot, ship_id, template_id, skin_id, level, efficiency, dispatched_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListGuildMissionShips :many
SELECT * FROM guild_operation_ships WHERE operation_id = ? ORDER BY event_id, commander_id, fleet, slot;

-- name: JoinGuildOperation :exec
INSERT INTO guild_operation_members (operation_id, commander_id, joined_at, extra) VALUES (?, ?, ?, ?);

-- name: GetGuildOperationMember :one
SELECT * FROM guild_operation_members WHERE operation_id = ? AND commander_id = ?;

-- name: ListGuildOperationMembers :many
SELECT * FROM guild_operation_members WHERE operation_id = ? ORDER BY boss_damage DESC, commander_id;

-- name: SaveGuildBossFight :exec
UPDATE guild_operation_members SET boss_day = ?, boss_count = ?, boss_damage = ?, boss_battle_at = 0
WHERE operation_id = ? AND commander_id = ?;

-- name: StartGuildBossBattle :exec
UPDATE guild_operation_members SET boss_battle_at = ? WHERE operation_id = ? AND commander_id = ?;

-- name: SetGuildMissionPerf :exec
INSERT INTO guild_operation_perfs (operation_id, commander_id, event_id, idx) VALUES (?, ?, ?, ?)
ON CONFLICT (operation_id, commander_id, event_id) DO UPDATE SET idx = excluded.idx;

-- name: ListGuildMissionPerfs :many
SELECT * FROM guild_operation_perfs WHERE operation_id = ? AND commander_id = ? ORDER BY event_id;

-- name: ClearGuildBossFleet :exec
DELETE FROM guild_boss_fleet_ships WHERE operation_id = ? AND commander_id = ? AND fleet_id = ?;

-- name: ClearGuildBossFleetMeowfficers :exec
DELETE FROM guild_boss_fleet_meowfficers WHERE operation_id = ? AND commander_id = ? AND fleet_id = ?;

-- name: AddGuildBossFleetShip :exec
INSERT INTO guild_boss_fleet_ships (operation_id, commander_id, fleet_id, slot, user_id, ship_id) VALUES (?, ?, ?, ?, ?, ?);

-- name: AddGuildBossFleetMeowfficer :exec
INSERT INTO guild_boss_fleet_meowfficers (operation_id, commander_id, fleet_id, pos, meowfficer_id) VALUES (?, ?, ?, ?, ?);

-- name: ListGuildBossFleetShips :many
SELECT * FROM guild_boss_fleet_ships WHERE operation_id = ? AND commander_id = ? ORDER BY fleet_id, slot;

-- name: ListGuildBossFleetMeowfficers :many
SELECT * FROM guild_boss_fleet_meowfficers WHERE operation_id = ? AND commander_id = ? ORDER BY fleet_id, pos;

-- name: AddGuildReport :one
INSERT INTO guild_reports (guild_id, operation_id, event_id, event_type, score, time)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetGuildReport :one
SELECT * FROM guild_reports WHERE id = ? AND guild_id = ?;

-- name: ListGuildReportsAfter :many
SELECT * FROM (
	SELECT * FROM guild_reports WHERE guild_id = ? AND id > sqlc.arg(after) ORDER BY id DESC LIMIT sqlc.arg(count)
) ORDER BY id;

-- name: SetGuildReportClaim :exec
INSERT INTO guild_report_claims (report_id, commander_id, status) VALUES (?, ?, ?)
ON CONFLICT (report_id, commander_id) DO UPDATE SET status = excluded.status;

-- name: GetGuildReportClaim :one
SELECT status FROM guild_report_claims WHERE report_id = ? AND commander_id = ?;

-- name: UseGuildUserExtraOperation :execrows
UPDATE guild_users SET extra_operation = extra_operation - 1 WHERE commander_id = ? AND extra_operation > 0;

-- name: DeleteGuildMissionShipsOf :exec
DELETE FROM guild_operation_ships WHERE operation_id = ? AND event_id = ? AND commander_id = ?;

-- name: DeleteGuildBossFleetShipsOf :exec
DELETE FROM guild_boss_fleet_ships
WHERE operation_id = sqlc.arg(operation_id) AND (commander_id = sqlc.arg(commander_id) OR user_id = sqlc.arg(commander_id));

-- name: DeleteGuildBossFleetMeowfficersOf :exec
DELETE FROM guild_boss_fleet_meowfficers WHERE operation_id = ? AND commander_id = ?;

-- name: DeleteGuildOperationMember :exec
DELETE FROM guild_operation_members WHERE operation_id = ? AND commander_id = ?;
