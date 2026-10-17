-- name: ListMedalShopPurchases :many
SELECT goods_id, count FROM commander_medal_shop_purchases
WHERE commander_id = ? AND flash_time = ?
ORDER BY goods_id;

-- name: AddMedalShopPurchase :exec
INSERT INTO commander_medal_shop_purchases (commander_id, goods_id, flash_time, count) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, goods_id, flash_time) DO UPDATE SET count = count + excluded.count;
