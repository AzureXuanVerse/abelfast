-- name: ListAtelierItems :many
SELECT item_id, count FROM commander_atelier_items WHERE commander_id = ? AND count > 0 ORDER BY item_id;

-- name: AddAtelierItem :exec
INSERT INTO commander_atelier_items (commander_id, item_id, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, item_id) DO UPDATE SET count = count + excluded.count;

-- name: SpendAtelierItem :execrows
UPDATE commander_atelier_items SET count = count - sqlc.arg(count)
WHERE commander_id = sqlc.arg(commander_id) AND item_id = sqlc.arg(item_id) AND count >= sqlc.arg(count);

-- name: ListAtelierRecipes :many
SELECT recipe_id, used FROM commander_atelier_recipes WHERE commander_id = ? ORDER BY recipe_id;

-- name: AddAtelierRecipeUse :exec
INSERT INTO commander_atelier_recipes (commander_id, recipe_id, used) VALUES (?, ?, ?)
ON CONFLICT (commander_id, recipe_id) DO UPDATE SET used = used + excluded.used;

-- name: ListAtelierSlots :many
SELECT pos, item_id, item_num FROM commander_atelier_slots WHERE commander_id = ? ORDER BY pos;

-- name: ClearAtelierSlots :exec
DELETE FROM commander_atelier_slots WHERE commander_id = ?;

-- name: SetAtelierSlot :exec
INSERT INTO commander_atelier_slots (commander_id, pos, item_id, item_num) VALUES (?, ?, ?, ?);
