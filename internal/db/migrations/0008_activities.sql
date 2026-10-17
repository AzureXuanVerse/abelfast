-- per-commander state (ACTIVITYINFO data fields) of the activities the
-- server changes, e.g. the 7-day login and the monthly sign-in.
-- data1_list is comma separated.
CREATE TABLE commander_activities (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	data1        INTEGER NOT NULL,
	data2        INTEGER NOT NULL,
	data3        INTEGER NOT NULL,
	data4        INTEGER NOT NULL,
	data1_list   TEXT    NOT NULL,
	PRIMARY KEY (commander_id, activity_id)
);

-- Juus posts (activity_ins_template) pushed to the commander, with what they
-- did on them
CREATE TABLE commander_juus_posts (
	commander_id    INTEGER NOT NULL REFERENCES commanders (id),
	post_id         INTEGER NOT NULL,
	is_good         BOOLEAN NOT NULL DEFAULT 0, -- liked
	is_read         BOOLEAN NOT NULL DEFAULT 0,
	comment_discuss INTEGER NOT NULL DEFAULT 0, -- player_discuss answered, 0 if none
	comment_index   INTEGER NOT NULL DEFAULT 0, -- 1-based option picked
	comment_time    INTEGER NOT NULL DEFAULT 0, -- unix seconds
	PRIMARY KEY (commander_id, post_id)
);

-- Juus chat topics (activity_ins_chat_group) the commander unlocked
CREATE TABLE commander_juus_topics (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	topic_id     INTEGER NOT NULL,
	ship_group   INTEGER NOT NULL,
	op_time      INTEGER NOT NULL, -- unix seconds
	read_flag    BOOLEAN NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, topic_id)
);

-- the commander's answers in a topic, in order: chat word id -> option picked
CREATE TABLE commander_juus_replies (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	topic_id     INTEGER NOT NULL,
	chat_id      INTEGER NOT NULL,
	value        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, topic_id, chat_id)
);
