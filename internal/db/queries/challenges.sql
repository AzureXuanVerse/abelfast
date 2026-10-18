-- name: ListActivityFleetShips :many
SELECT * FROM activity_fleet_ships WHERE commander_id = ? ORDER BY activity_id, fleet_id, pos;

-- name: ListActivityFleetMeowfficers :many
SELECT * FROM activity_fleet_meowfficers WHERE commander_id = ? ORDER BY activity_id, fleet_id, pos;

-- name: ClearActivityFleetShips :exec
DELETE FROM activity_fleet_ships WHERE commander_id = ? AND activity_id = ?;

-- name: ClearActivityFleetMeowfficers :exec
DELETE FROM activity_fleet_meowfficers WHERE commander_id = ? AND activity_id = ?;

-- name: AddActivityFleetShip :exec
INSERT INTO activity_fleet_ships (commander_id, activity_id, fleet_id, pos, ship_id) VALUES (?, ?, ?, ?, ?);

-- name: AddActivityFleetMeowfficer :exec
INSERT INTO activity_fleet_meowfficers (commander_id, activity_id, fleet_id, pos, meowfficer_id) VALUES (?, ?, ?, ?, ?);

-- name: ListChallengeRuns :many
SELECT * FROM challenge_runs WHERE commander_id = ? AND activity_id = ? ORDER BY mode;

-- name: GetChallengeRun :one
SELECT * FROM challenge_runs WHERE commander_id = ? AND activity_id = ? AND mode = ?;

-- name: CreateChallengeRun :exec
INSERT INTO challenge_runs (commander_id, activity_id, mode, season) VALUES (?, ?, ?, ?);

-- name: SaveChallengeRun :exec
UPDATE challenge_runs SET level = ?, score = ?, battling = ?
WHERE commander_id = ? AND activity_id = ? AND mode = ?;

-- name: DeleteChallengeRun :execrows
DELETE FROM challenge_runs WHERE commander_id = ? AND activity_id = ? AND mode = ?;

-- name: ListChallengeShips :many
SELECT * FROM challenge_ships WHERE commander_id = ? AND activity_id = ? ORDER BY mode, fleet_id, pos;

-- name: AddChallengeShip :exec
INSERT INTO challenge_ships (commander_id, activity_id, mode, fleet_id, pos, ship_id, hp_rant) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: SetChallengeShipHP :exec
UPDATE challenge_ships SET hp_rant = ?
WHERE commander_id = ? AND activity_id = ? AND mode = ? AND ship_id = ?;

-- name: ListChallengeMeowfficers :many
SELECT * FROM challenge_meowfficers WHERE commander_id = ? AND activity_id = ? ORDER BY mode, fleet_id, pos;

-- name: AddChallengeMeowfficer :exec
INSERT INTO challenge_meowfficers (commander_id, activity_id, mode, fleet_id, pos, meowfficer_id) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListChallengeRecords :many
SELECT * FROM challenge_records WHERE commander_id = ? AND activity_id = ?;

-- name: RecordChallenge :exec
INSERT INTO challenge_records (commander_id, activity_id, season, max_score, max_level) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, season) DO UPDATE SET
	max_score = max(challenge_records.max_score, excluded.max_score),
	max_level = max(challenge_records.max_level, excluded.max_level);
