-- name: EnsureDorm :exec
INSERT INTO commander_dorms (commander_id, load_time) VALUES (?, ?)
ON CONFLICT (commander_id) DO NOTHING;

-- name: GetDorm :one
SELECT * FROM commander_dorms WHERE commander_id = ?;

-- name: SetDormSettlement :exec
UPDATE commander_dorms SET load_time = ?, next_timestamp = ? WHERE commander_id = ?;

-- name: SetDormName :exec
UPDATE commander_dorms SET name = ? WHERE commander_id = ?;

-- name: SetDormLevel :exec
UPDATE commander_dorms SET level = ? WHERE commander_id = ?;

-- name: SetDormFloorNum :exec
UPDATE commander_dorms SET floor_num = ? WHERE commander_id = ?;

-- name: SetDormExpPos :exec
UPDATE commander_dorms SET exp_pos = ? WHERE commander_id = ?;

-- name: AddDormFoodMax :exec
UPDATE commander_dorms SET food_max_increase = food_max_increase + ?, food_max_increase_count = food_max_increase_count + 1
WHERE commander_id = ?;

-- name: ListDormShips :many
SELECT * FROM dorm_ships WHERE commander_id = ? ORDER BY ship_id;

-- name: AddDormShip :exec
INSERT INTO dorm_ships (ship_id, commander_id, floor, pop_at) VALUES (?, ?, ?, ?);

-- name: DeleteDormShip :execrows
DELETE FROM dorm_ships WHERE ship_id = ? AND commander_id = ?;

-- name: SetDormShipPop :exec
UPDATE dorm_ships SET pop_icon = ?, pop_intimacy = ?, pop_at = ? WHERE ship_id = ? AND commander_id = ?;

-- name: ListDormFloors :many
SELECT floor, layout FROM dorm_floors WHERE commander_id = ? ORDER BY floor;

-- name: SetDormFloor :exec
INSERT INTO dorm_floors (commander_id, floor, layout) VALUES (?, ?, ?)
ON CONFLICT (commander_id, floor) DO UPDATE SET layout = excluded.layout;

-- name: ListDormThemes :many
SELECT pos, theme FROM dorm_themes WHERE commander_id = ? ORDER BY pos;

-- name: SetDormTheme :exec
INSERT INTO dorm_themes (commander_id, pos, theme) VALUES (?, ?, ?)
ON CONFLICT (commander_id, pos) DO UPDATE SET theme = excluded.theme;

-- name: DeleteDormTheme :execrows
DELETE FROM dorm_themes WHERE commander_id = ? AND pos = ?;
