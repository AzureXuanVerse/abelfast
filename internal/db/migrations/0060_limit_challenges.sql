-- a commander's limit challenges (expedition_constellation_challenge_template
-- ids, LimitChallengeProxy): the best clear time in seconds (the client's
-- passTimeDict), the month (YYYYMM) of the latest clear, for this month's
-- pass_ids, and whether the clear reward was taken (awardedDict).
CREATE TABLE limit_challenges (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	challenge_id INTEGER NOT NULL,
	best_time    INTEGER NOT NULL DEFAULT 0,
	pass_month   INTEGER NOT NULL DEFAULT 0,
	awarded      BOOLEAN NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, challenge_id)
);

-- the icon frames (DROP_TYPE_ICON_FRAME, item_data_frame) and chat frames
-- (DROP_TYPE_CHAT_FRAME, item_data_chat) a commander owns, for good: the
-- limit challenge rewards (SC_11003 icon_frame_list / chat_frame_list).
CREATE TABLE attire_frames (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	drop_type    INTEGER NOT NULL,
	frame_id     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, drop_type, frame_id)
);
