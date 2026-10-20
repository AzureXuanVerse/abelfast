-- Private Quarters (apartment) Juustagram side, APARTMENT_INS: what the
-- commander set in a ship group's chat (Dorm3dChat: care_flag, cur_back the
-- chat background, cur_comm_id the current topic)
CREATE TABLE commander_apartment_chats (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	ship_group   INTEGER NOT NULL,
	care_flag    INTEGER NOT NULL DEFAULT 0,
	cur_back     INTEGER NOT NULL DEFAULT 0,
	cur_comm_id  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, ship_group)
);

-- the dorm3d_ins_unlock counters (trigger_type): per ship group, the
-- events counted (or the highest value, for the absolute ones)
CREATE TABLE commander_apartment_events (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	ship_group   INTEGER NOT NULL,
	event_type   INTEGER NOT NULL,
	value        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, ship_group, event_type)
);

-- the unlocked contents (dorm3d_ins_unlock type: 1 chat topic
-- dorm3d_ins_chat_group, 2 phone call dorm3d_ins_telephone_group, 3 post
-- dorm3d_ins_template), unlocked at time; good_flag and exit_time are the
-- posts' only
CREATE TABLE commander_apartment_ins_contents (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	type         INTEGER NOT NULL,
	content_id   INTEGER NOT NULL,
	ship_group   INTEGER NOT NULL,
	time         INTEGER NOT NULL,
	read_flag    INTEGER NOT NULL DEFAULT 0,
	good_flag    INTEGER NOT NULL DEFAULT 0,
	exit_time    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, type, content_id)
);

-- the replies to a content: a chat topic's (reply_key the word, value the
-- option) or a post's (reply_key the dorm3d_ins_player_template comment,
-- value the message index), in the order given
CREATE TABLE commander_apartment_ins_replies (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	type         INTEGER NOT NULL,
	content_id   INTEGER NOT NULL,
	reply_key    INTEGER NOT NULL,
	value        INTEGER NOT NULL,
	time         INTEGER NOT NULL,
	PRIMARY KEY (commander_id, type, content_id, reply_key)
);
