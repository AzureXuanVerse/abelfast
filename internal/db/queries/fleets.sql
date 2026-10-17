-- name: CreateFleet :exec
INSERT INTO fleets (commander_id, fleet_id, name) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: ListFleets :many
SELECT fleet_id, name FROM fleets WHERE commander_id = ? ORDER BY fleet_id DESC;

-- name: ListFleetShips :many
SELECT fleet_id, ship_id FROM fleet_ships WHERE commander_id = ? ORDER BY fleet_id, position;

-- name: ListShipsOfFleet :many
SELECT ship_id FROM fleet_ships WHERE commander_id = ? AND fleet_id = ? ORDER BY position;

-- name: ClearFleetShips :exec
DELETE FROM fleet_ships WHERE commander_id = ? AND fleet_id = ?;

-- name: AddFleetShip :exec
INSERT INTO fleet_ships (commander_id, fleet_id, position, ship_id) VALUES (?, ?, ?, ?);
