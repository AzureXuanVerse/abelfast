-- name: ProposeShip :exec
UPDATE ships SET propose = ?, is_locked = 1 WHERE id = ? AND commander_id = ?;

-- name: SetMarryShip :exec
UPDATE commanders SET marry_ship = ? WHERE id = ?;

-- name: RenameShip :exec
UPDATE ships SET name = ?, change_name_timestamp = ? WHERE id = ? AND commander_id = ?;

-- name: SetShipCommonFlag :exec
UPDATE ships SET common_flag = ? WHERE id = ? AND commander_id = ?;

-- name: RenameFleet :exec
INSERT INTO fleets (commander_id, fleet_id, name) VALUES (?, ?, ?)
ON CONFLICT (commander_id, fleet_id) DO UPDATE SET name = excluded.name;
