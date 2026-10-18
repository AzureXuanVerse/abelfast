-- name: ListBagSpWeapons :many
SELECT * FROM spweapons
WHERE commander_id = ? AND id NOT IN (SELECT spweapon_id FROM ship_spweapons)
ORDER BY id;

-- name: CountBagSpWeapons :one
SELECT COUNT(*) FROM spweapons
WHERE commander_id = ? AND id NOT IN (SELECT spweapon_id FROM ship_spweapons);

-- name: ListShipSpWeapons :many
SELECT sqlc.embed(spweapons), ship_spweapons.ship_id FROM spweapons
JOIN ship_spweapons ON ship_spweapons.spweapon_id = spweapons.id
WHERE spweapons.commander_id = ?;

-- name: GetSpWeapon :one
SELECT * FROM spweapons WHERE commander_id = ? AND id = ?;

-- name: GetSpWeaponShip :one
SELECT ship_id FROM ship_spweapons WHERE spweapon_id = ?;

-- name: GetShipSpWeapon :one
SELECT spweapon_id FROM ship_spweapons WHERE ship_id = ?;

-- name: CreateSpWeapon :one
INSERT INTO spweapons (commander_id, template_id, attr_1, attr_2, pt)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateSpWeapon :exec
UPDATE spweapons SET template_id = ?, attr_1 = ?, attr_2 = ?, attr_temp_1 = ?, attr_temp_2 = ?, pt = ?
WHERE commander_id = ? AND id = ?;

-- name: DeleteSpWeapon :exec
DELETE FROM spweapons WHERE commander_id = ? AND id = ?;

-- name: EquipSpWeapon :exec
INSERT INTO ship_spweapons (ship_id, spweapon_id) VALUES (?, ?);

-- name: UnequipShipSpWeapon :exec
DELETE FROM ship_spweapons WHERE ship_id = ?;
