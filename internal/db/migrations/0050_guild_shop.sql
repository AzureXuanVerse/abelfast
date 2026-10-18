-- a commander's guild shop stock (GetGuildShopCommand, SC_60034):
-- next_refresh_time the unix time it refreshes on its own (guildset
-- store_refresh), refresh_count the manual refreshes bought since
-- (GuildShop.refreshCount, at most guildset store_reset_time). The goods
-- are drawn again from both, so only the purchases are kept.
CREATE TABLE commander_guild_shops (
	commander_id      INTEGER PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	next_refresh_time BIGINT  NOT NULL,
	refresh_count     INTEGER NOT NULL
);

-- what the commander bought from its current stock (GuildShopCommand,
-- CS_60035): slot is the goods' GOODS_INFO_P60 index, count the purchases,
-- counted against guild_store goods_purchase_limit. Cleared with the stock.
CREATE TABLE commander_guild_shop_purchases (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	slot         INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, slot)
);
