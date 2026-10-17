-- honormedal_goods_list goods bought in the medal exchange (MedalShopCommand,
-- CS_16108), per stock: flash_time is the SC_16107 item_flash_time of the
-- stock they were bought from, count the goods bought (the selected counts
-- summed), counted against goods_purchase_limit until it refreshes
CREATE TABLE commander_medal_shop_purchases (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	goods_id     INTEGER NOT NULL, -- honormedal_goods_list id
	flash_time   BIGINT  NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, goods_id, flash_time)
);
