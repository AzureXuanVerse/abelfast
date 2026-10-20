-- ACTIVITYINFO.str_data1, the text ActivityStoreDataCommand (CS_26160)
-- stores along with data1 (e.g. the name given to the car of the 2026
-- monopoly event).
ALTER TABLE commander_activities ADD COLUMN str_data1 TEXT NOT NULL DEFAULT '';

-- what the commander bought in the new commander shop
-- (ACTIVITY_TYPE_NEWSERVER_SHOP, newserver_shop_template) and the Black
-- Friday shop (ACTIVITY_TYPE_BLACK_FRIDAY_SHOP, blackfriday_shop_template)
-- of an activity: bought times of each goods.
CREATE TABLE new_server_shop_goods (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	goods_id     INTEGER NOT NULL,
	bought       INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, goods_id)
);

-- the sub-goods taken from a goods that sells each of them once
-- (goods_type 4, ACT_GOODS_INFO.bought_record)
CREATE TABLE new_server_shop_sub_goods (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	goods_id     INTEGER NOT NULL,
	item_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, goods_id, item_id)
);
