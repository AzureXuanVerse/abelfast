-- name: ListNewServerShopGoods :many
SELECT goods_id, bought FROM new_server_shop_goods WHERE commander_id = ? AND activity_id = ?;

-- name: AddNewServerShopGoods :exec
INSERT INTO new_server_shop_goods (commander_id, activity_id, goods_id, bought) VALUES (?, ?, ?, ?)
ON CONFLICT (commander_id, activity_id, goods_id) DO UPDATE SET bought = bought + excluded.bought;

-- name: ListNewServerShopSubGoods :many
SELECT goods_id, item_id FROM new_server_shop_sub_goods WHERE commander_id = ? AND activity_id = ? ORDER BY rowid;

-- name: AddNewServerShopSubGoods :execrows
INSERT INTO new_server_shop_sub_goods (commander_id, activity_id, goods_id, item_id) VALUES (?, ?, ?, ?)
ON CONFLICT DO NOTHING;
