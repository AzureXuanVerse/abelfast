-- name: AddMonthShopPurchase :exec
INSERT INTO commander_month_shop_purchases (commander_id, goods_id, month, count) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, goods_id, month) DO UPDATE SET count = count + excluded.count;

-- name: ListMonthShopPurchases :many
SELECT goods_id, count FROM commander_month_shop_purchases
WHERE commander_id = sqlc.arg(commander_id) AND month IN (sqlc.arg(month), 0)
ORDER BY goods_id;
