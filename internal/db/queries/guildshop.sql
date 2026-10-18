-- name: GetGuildShop :one
SELECT * FROM commander_guild_shops WHERE commander_id = ?;

-- name: SetGuildShop :exec
INSERT INTO commander_guild_shops (commander_id, next_refresh_time, refresh_count) VALUES (?, ?, ?)
ON CONFLICT (commander_id) DO UPDATE SET
	next_refresh_time = excluded.next_refresh_time, refresh_count = excluded.refresh_count;

-- name: ListGuildShopPurchases :many
SELECT slot, count FROM commander_guild_shop_purchases WHERE commander_id = ? ORDER BY slot;

-- name: AddGuildShopPurchase :exec
INSERT INTO commander_guild_shop_purchases (commander_id, slot, count) VALUES (?, ?, ?)
ON CONFLICT (commander_id, slot) DO UPDATE SET count = count + excluded.count;

-- name: ClearGuildShopPurchases :exec
DELETE FROM commander_guild_shop_purchases WHERE commander_id = ?;
