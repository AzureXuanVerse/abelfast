-- name: AdminBackyardSetFoodMax :exec
UPDATE commander_dorms SET food_max_increase = ?, food_max_increase_count = ? WHERE commander_id = ?;

-- name: AdminBackyardSetShipFloor :execrows
UPDATE dorm_ships SET floor = ? WHERE ship_id = ? AND commander_id = ?;

-- name: AdminBackyardSetFurniture :exec
INSERT INTO commander_furniture (commander_id, furniture_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, furniture_id) DO UPDATE SET count = excluded.count;

-- name: AdminBackyardDeleteFurniture :exec
DELETE FROM commander_furniture WHERE commander_id = ? AND furniture_id = ?;

-- name: AdminBackyardDeleteFloor :exec
DELETE FROM dorm_floors WHERE commander_id = ? AND floor = ?;
