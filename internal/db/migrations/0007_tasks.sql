-- TaskProxy tasks (SC_20001 / SC_20003). A claimed task keeps its row with
-- submit_time set, so it is neither sent again nor accepted twice.
CREATE TABLE commander_tasks (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	task_id      INTEGER NOT NULL, -- task_data_template id
	progress     INTEGER NOT NULL,
	accept_time  INTEGER NOT NULL, -- unix seconds
	submit_time  INTEGER NOT NULL DEFAULT 0, -- unix seconds, 0 until claimed
	PRIMARY KEY (commander_id, task_id)
);

-- weekly missions (SC_20101): the current step of each weekly_task_template
-- chain, and the weekly reward level reached (pt is virtual item 59012)
CREATE TABLE commander_weekly_tasks (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	task_id      INTEGER NOT NULL, -- weekly_task_template id
	progress     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, task_id)
);

CREATE TABLE commander_weekly (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	reward_lv    INTEGER NOT NULL
);

-- progress of the tasks of task activities (SC_20201), the task list itself
-- comes from activity_template
CREATE TABLE commander_activity_tasks (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	act_id       INTEGER NOT NULL,
	task_id      INTEGER NOT NULL,
	progress     INTEGER NOT NULL,
	finished     BOOLEAN NOT NULL DEFAULT 0, -- reward claimed
	PRIMARY KEY (commander_id, act_id, task_id)
);

-- what the commander did so far, per task sub_type and target (0 = any
-- target), so tasks accepted later can start from it (builds, levels, stars)
CREATE TABLE commander_task_counters (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	sub_type     INTEGER NOT NULL,
	target       INTEGER NOT NULL,
	value        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, sub_type, target)
);

-- commander manual pages (SC_22300): tasks finished on each page, pt (one
-- per finished page task) and how many pt rewards were claimed
CREATE TABLE commander_handbook_pages (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	page_id      INTEGER NOT NULL, -- tutorial_handbook_task id
	pt           INTEGER NOT NULL DEFAULT 0,
	award        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, page_id)
);

CREATE TABLE commander_handbook_finished (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	page_id      INTEGER NOT NULL,
	task_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, page_id, task_id)
);
