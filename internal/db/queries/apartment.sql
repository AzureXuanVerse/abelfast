-- name: GetApartmentState :one
SELECT * FROM commander_apartment WHERE commander_id = ?;

-- name: SetApartmentVigor :exec
INSERT INTO commander_apartment (commander_id, vigor_used, vigor_day) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET vigor_used = excluded.vigor_used, vigor_day = excluded.vigor_day;

-- name: SetApartmentVisitDay :exec
INSERT INTO commander_apartment (commander_id, visit_day) VALUES (?, ?)
ON CONFLICT (commander_id) DO UPDATE SET visit_day = excluded.visit_day;

-- name: ListApartmentShips :many
SELECT * FROM apartment_ships WHERE commander_id = ? ORDER BY ship_group;

-- name: GetApartmentShip :one
SELECT * FROM apartment_ships WHERE commander_id = ? AND ship_group = ?;

-- name: AddApartmentShip :exec
INSERT INTO apartment_ships (commander_id, ship_group) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: SetApartmentShip :exec
UPDATE apartment_ships SET favor_lv = ?, favor_exp = ?, daily_favor = ?, favor_day = ?, triggers = ?, dialogues = ?,
	skins = ?, cur_skin = ?, name = ?, name_cd = ?, visit_time = ?
WHERE commander_id = ? AND ship_group = ?;

-- name: ListApartmentHiddenParts :many
SELECT * FROM apartment_hidden_parts WHERE commander_id = ? ORDER BY ship_group, skin_id;

-- name: SetApartmentHiddenParts :exec
INSERT INTO apartment_hidden_parts (commander_id, ship_group, skin_id, parts) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, ship_group, skin_id) DO UPDATE SET parts = excluded.parts;

-- name: ListApartmentRooms :many
SELECT * FROM apartment_rooms WHERE commander_id = ? ORDER BY room_id;

-- name: GetApartmentRoom :one
SELECT * FROM apartment_rooms WHERE commander_id = ? AND room_id = ?;

-- name: AddApartmentRoom :exec
INSERT INTO apartment_rooms (commander_id, room_id) VALUES (?, ?);

-- name: SetApartmentRoom :exec
UPDATE apartment_rooms SET collections = ?, ships = ? WHERE commander_id = ? AND room_id = ?;

-- name: ListApartmentFurniture :many
SELECT * FROM apartment_furniture WHERE commander_id = ? ORDER BY room_id, furniture_id;

-- name: AddApartmentFurniture :exec
INSERT INTO apartment_furniture (commander_id, room_id, furniture_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: SetApartmentFurnitureSlot :exec
UPDATE apartment_furniture SET slot_id = ? WHERE commander_id = ? AND room_id = ? AND furniture_id = ?;

-- name: ListApartmentGifts :many
SELECT * FROM apartment_gifts WHERE commander_id = ? ORDER BY gift_id;

-- name: AddApartmentGift :exec
INSERT INTO apartment_gifts (commander_id, gift_id, number) VALUES (?, ?, ?)
ON CONFLICT (commander_id, gift_id) DO UPDATE SET number = number + excluded.number;

-- name: GiveApartmentGift :execrows
UPDATE apartment_gifts SET number = number - sqlc.arg(count), used_number = used_number + sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND gift_id = sqlc.arg(gift_id) AND number >= sqlc.arg(count);

-- name: GetShipGroupMarried :one
SELECT marry_flag FROM commander_ship_groups WHERE commander_id = ? AND group_id = ?;
