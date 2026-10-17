-- SC_11003 commander state. The defaults are what the official server gives
-- a new commander: the capacities are gameset shipyard_capacity,
-- equip_capacity and commander_capacity, the rest is captured.
ALTER TABLE commanders ADD COLUMN level INTEGER NOT NULL DEFAULT 1;
ALTER TABLE commanders ADD COLUMN exp INTEGER NOT NULL DEFAULT 0;
ALTER TABLE commanders ADD COLUMN ship_bag_max INTEGER NOT NULL DEFAULT 150;
ALTER TABLE commanders ADD COLUMN equip_bag_max INTEGER NOT NULL DEFAULT 300;
ALTER TABLE commanders ADD COLUMN commander_bag_max INTEGER NOT NULL DEFAULT 40;
ALTER TABLE commanders ADD COLUMN rank INTEGER NOT NULL DEFAULT 1;
ALTER TABLE commanders ADD COLUMN chat_room_id INTEGER NOT NULL DEFAULT 2;
ALTER TABLE commanders ADD COLUMN mail_storeroom_lv INTEGER NOT NULL DEFAULT 1;
ALTER TABLE commanders ADD COLUMN manifesto TEXT NOT NULL DEFAULT '';
-- display.icon (a ship template), set to the starter ship at creation
ALTER TABLE commanders ADD COLUMN icon INTEGER NOT NULL DEFAULT 0;
-- the backyard's last collection time (SC_19001.load_time)
ALTER TABLE commanders ADD COLUMN dorm_load_time INTEGER NOT NULL DEFAULT 0;

-- secretaries (SC_11003.character), position 1 is the main one
CREATE TABLE commander_secretaries (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	position     INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	phantom_id   INTEGER NOT NULL,
	PRIMARY KEY (commander_id, position)
);

-- owned backyard furniture (furniture_data_template ids)
CREATE TABLE commander_furniture (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	furniture_id INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, furniture_id)
);
