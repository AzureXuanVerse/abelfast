-- shop_template goods bought (ShoppingCommand, CS_16001), one per purchase,
-- counted until expires_at (unix seconds, when the shop refreshes; 0 never):
-- the general shop's stock, the merit shop's day, the oil bought today, the
-- gift packages' purchase limits
CREATE TABLE commander_shop_purchases (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	shop_id      INTEGER NOT NULL,
	expires_at   BIGINT  NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, shop_id, expires_at)
);

-- shop_template groups' purchases (group_buy_count each), counted against
-- the group's group_limit until expires_at (0 never)
CREATE TABLE commander_shop_group_purchases (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	group_id     INTEGER NOT NULL,
	expires_at   BIGINT  NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, group_id, expires_at)
);
