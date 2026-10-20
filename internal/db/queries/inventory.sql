-- name: ListResources :many
SELECT resource_id, amount FROM commander_resources WHERE commander_id = ? ORDER BY resource_id;

-- name: AddResource :exec
INSERT INTO commander_resources (commander_id, resource_id, amount) VALUES (?, ?, ?)
ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = amount + excluded.amount;

-- name: SpendResource :execrows
UPDATE commander_resources SET amount = amount - sqlc.arg(amount)
WHERE commander_id = sqlc.arg(commander_id) AND resource_id = sqlc.arg(resource_id) AND amount >= sqlc.arg(amount);

-- name: ListItems :many
SELECT item_id, count FROM commander_items WHERE commander_id = ? AND count > 0 ORDER BY item_id;

-- name: AddItem :exec
INSERT INTO commander_items (commander_id, item_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, item_id) DO UPDATE SET count = count + excluded.count;

-- name: SpendItem :execrows
UPDATE commander_items SET count = count - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND item_id = sqlc.arg(item_id) AND count >= sqlc.arg(count);

-- name: ListEquipments :many
SELECT equip_id, count FROM commander_equipments WHERE commander_id = ? AND count > 0 ORDER BY equip_id;

-- name: AddEquipment :exec
INSERT INTO commander_equipments (commander_id, equip_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, equip_id) DO UPDATE SET count = count + excluded.count;

-- name: SpendEquipment :execrows
UPDATE commander_equipments SET count = count - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND equip_id = sqlc.arg(equip_id) AND count >= sqlc.arg(count);

-- name: GetVirtualItem :one
SELECT amount FROM commander_virtual_items WHERE commander_id = ? AND item_id = ?;

-- name: SpendVirtualItem :execrows
UPDATE commander_virtual_items SET amount = amount - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND item_id = sqlc.arg(item_id) AND amount >= sqlc.arg(count);

-- name: AddVirtualItem :exec
INSERT INTO commander_virtual_items (commander_id, item_id, amount) VALUES (?, ?, ?)
ON CONFLICT (commander_id, item_id) DO UPDATE SET amount = amount + excluded.amount;

-- name: CountEquipments :one
SELECT CAST(COALESCE(SUM(count), 0) AS INTEGER) FROM commander_equipments WHERE commander_id = ?;
