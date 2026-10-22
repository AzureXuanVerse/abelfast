-- The back-office's dock, fleets and meowfficers (internal/admin, game/admin_dock.go).

-- name: AdminDockSetShip :exec
UPDATE ships SET template_id = ?, level = ?, exp = ?, max_level = ?, intimacy = ?, energy = ?, is_locked = ?, propose = ?
WHERE id = ? AND commander_id = ?;

-- name: AdminDockClearMarryShip :exec
UPDATE commanders SET marry_ship = 0 WHERE id = ? AND marry_ship = ?;

-- name: AdminDockDeleteEliteFleet :execrows
DELETE FROM chapter_elite_fleets WHERE commander_id = ? AND formation = ?;

-- name: AdminDockClearActivityFleetShips :exec
DELETE FROM activity_fleet_ships WHERE commander_id = ? AND activity_id = ? AND fleet_id = ?;

-- name: AdminDockClearActivityFleetMeowfficers :exec
DELETE FROM activity_fleet_meowfficers WHERE commander_id = ? AND activity_id = ? AND fleet_id = ?;

-- name: AdminDockDeleteSecretary :exec
DELETE FROM commander_secretaries WHERE commander_id = ? AND ship_id = ?;

-- name: AdminDockDeleteChallengeShip :exec
DELETE FROM challenge_ships WHERE commander_id = ? AND ship_id = ?;

-- name: AdminDockDeleteWorldBossShip :exec
DELETE FROM world_boss_ships WHERE commander_id = ? AND ship_id = ?;

-- name: AdminDockDeleteGuildBossShip :exec
DELETE FROM guild_boss_fleet_ships WHERE user_id = ? AND ship_id = ?;

-- name: AdminDockCountGuildMissionShip :one
SELECT COUNT(*) FROM guild_operation_ships WHERE commander_id = ? AND ship_id = ?;

-- name: AdminDockBlueprintOfShip :one
SELECT COUNT(*) FROM commander_blueprints WHERE commander_id = ? AND ship_id = ?;

-- name: AdminDockSetMeowfficerName :exec
UPDATE meowfficers SET name = ? WHERE commander_id = ? AND id = ?;
