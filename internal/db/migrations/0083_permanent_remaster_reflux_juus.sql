-- mini-events replayed from the Mini-Event Gallery and the boss rush
-- archives (activity_task_permanent, ActivityPermanentProxy, SC_11210): one
-- row per activity the commander started. running: in progress
-- (permanent_now), finished: completed (permanent_activity).
CREATE TABLE commander_permanent_activities (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	running      BOOLEAN NOT NULL DEFAULT 0,
	finished     BOOLEAN NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, activity_id)
);

-- Capsule Reruns (activity_re, ActivityRemasterProxy, SC_11213): each
-- event the commander chose, during which activity_re_timer period and when
-- (unix seconds). The latest one is the running rerun.
CREATE TABLE commander_remasters (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	remaster_id  INTEGER NOT NULL,
	timer_id     INTEGER NOT NULL,
	activated_at BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, remaster_id)
);

-- returning player campaign (RefluxProxy, SC_11752): the commander's latest
-- return. return_time / last_offline_time / sign_last_time are unix seconds,
-- return_lv the level and ship_number the ships owned when they came back,
-- pt the points the campaign's tasks gave, sign_cnt the days signed in and
-- pt_stage the return_pt_template rewards claimed.
CREATE TABLE commander_reflux (
	commander_id      INTEGER PRIMARY KEY REFERENCES commanders (id),
	return_lv         INTEGER NOT NULL,
	return_time       BIGINT  NOT NULL,
	ship_number       INTEGER NOT NULL,
	last_offline_time BIGINT  NOT NULL,
	pt                INTEGER NOT NULL DEFAULT 0,
	sign_cnt          INTEGER NOT NULL DEFAULT 0,
	sign_last_time    BIGINT  NOT NULL DEFAULT 0,
	pt_stage          INTEGER NOT NULL DEFAULT 0
);

-- a Juus chat's settings (JUUS_GROUP, activity_ins_ship_group_template id):
-- the background skin (0 the default), the care flag and the topic shown
-- (0: the latest one unlocked)
CREATE TABLE commander_juus_groups (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	group_id     INTEGER NOT NULL,
	skin_id      INTEGER NOT NULL DEFAULT 0,
	favorite     INTEGER NOT NULL DEFAULT 0,
	cur_topic    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, group_id)
);
