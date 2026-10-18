-- activity_shop_template goods bought in the monthly shops (CS_16201): the
-- Core Data exchange and the Prototype shop, counted per month (year * 100 +
-- month, server time), 0 for the Prototype shop's limited goods that never
-- reset (FragmentShop:Reset). Sent back in SC_16200.
CREATE TABLE commander_month_shop_purchases (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	goods_id     INTEGER NOT NULL,
	month        INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, goods_id, month)
);
