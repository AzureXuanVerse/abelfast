-- the Private Quarters (ApartmentProxy, SC_28000): the energy (vigor) spent
-- today, the server day it was spent on (unix time of its start), and the
-- last server day the commander visited a room (for the weekly task)
CREATE TABLE commander_apartment (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	vigor_used   INTEGER NOT NULL DEFAULT 0,
	vigor_day    INTEGER NOT NULL DEFAULT 0,
	visit_day    INTEGER NOT NULL DEFAULT 0
);

-- a shipgirl whose personal room is open (APARTMENT_SHIP): her Intimacy
-- level and points, the points gained on favor_day, the non-repeatable
-- favor triggers done (dorm3d_favor_trigger is_repeat 0), the dialogues
-- played, the dorm3d skins obtained, the skin worn (0 the default), what
-- she calls the commander and until when it can't be changed, the last
-- visit
CREATE TABLE apartment_ships (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	ship_group   INTEGER NOT NULL,
	favor_lv     INTEGER NOT NULL DEFAULT 1,
	favor_exp    INTEGER NOT NULL DEFAULT 0,
	daily_favor  INTEGER NOT NULL DEFAULT 0,
	favor_day    INTEGER NOT NULL DEFAULT 0,
	triggers     TEXT    NOT NULL DEFAULT '[]',
	dialogues    TEXT    NOT NULL DEFAULT '[]',
	skins        TEXT    NOT NULL DEFAULT '[]',
	cur_skin     INTEGER NOT NULL DEFAULT 0,
	name         TEXT    NOT NULL DEFAULT '',
	name_cd      INTEGER NOT NULL DEFAULT 0,
	visit_time   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, ship_group)
);

-- the parts of a skin a shipgirl wears hidden (SKIN_HIDDEN_INFO)
CREATE TABLE apartment_hidden_parts (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	ship_group   INTEGER NOT NULL,
	skin_id      INTEGER NOT NULL,
	parts        TEXT    NOT NULL DEFAULT '[]',
	PRIMARY KEY (commander_id, ship_group, skin_id)
);

-- an unlocked room (APARTMENT_ROOM): the collection items found in it and
-- the shipgirls invited to it (a common area's character_pay)
CREATE TABLE apartment_rooms (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	room_id      INTEGER NOT NULL,
	collections  TEXT    NOT NULL DEFAULT '[]',
	ships        TEXT    NOT NULL DEFAULT '[]',
	PRIMARY KEY (commander_id, room_id)
);

-- the furniture owned for a room and the slot it is placed in (0 stored)
CREATE TABLE apartment_furniture (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	room_id      INTEGER NOT NULL,
	furniture_id INTEGER NOT NULL,
	slot_id      INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, room_id, furniture_id)
);

-- the gifts owned (dorm3d_gift) and how many were given
CREATE TABLE apartment_gifts (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	gift_id      INTEGER NOT NULL,
	number       INTEGER NOT NULL DEFAULT 0,
	used_number  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, gift_id)
);
