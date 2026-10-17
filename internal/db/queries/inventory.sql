-- name: ListResources :many
SELECT resource_id, amount FROM commander_resources WHERE commander_id = ? ORDER BY resource_id;

-- name: AddResource :exec
INSERT INTO commander_resources (commander_id, resource_id, amount) VALUES (?, ?, ?)
ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = amount + excluded.amount;

-- name: ListItems :many
SELECT item_id, count FROM commander_items WHERE commander_id = ? AND count > 0 ORDER BY item_id;

-- name: AddItem :exec
INSERT INTO commander_items (commander_id, item_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, item_id) DO UPDATE SET count = count + excluded.count;
