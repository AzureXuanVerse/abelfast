-- name: ListShopPurchases :many
SELECT shop_id, count FROM commander_shop_purchases
WHERE commander_id = sqlc.arg(commander_id) AND (expires_at = 0 OR expires_at > sqlc.arg(now))
ORDER BY shop_id;

-- name: AddShopPurchase :exec
INSERT INTO commander_shop_purchases (commander_id, shop_id, expires_at, count) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, shop_id, expires_at) DO UPDATE SET count = count + excluded.count;

-- name: ListShopGroupPurchases :many
SELECT group_id, count FROM commander_shop_group_purchases
WHERE commander_id = sqlc.arg(commander_id) AND (expires_at = 0 OR expires_at > sqlc.arg(now))
ORDER BY group_id;

-- name: AddShopGroupPurchase :exec
INSERT INTO commander_shop_group_purchases (commander_id, group_id, expires_at, count) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, group_id, expires_at) DO UPDATE SET count = count + excluded.count;
