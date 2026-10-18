-- Daily challenges (DailyLevelProxy, SC_13201)

-- the attempts spent on a daily challenge (expedition_daily_template id) in
-- the reset period starting at period (unix seconds: the server day for
-- limit_type 1, the server week for limit_type 2); a row of an older period
-- counts as none (count_list)
CREATE TABLE daily_level_counts (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	daily_id     INTEGER NOT NULL,
	count        INTEGER NOT NULL,
	period       BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, daily_id)
);

-- the daily challenge stages (expedition_data_template ids) cleared with an
-- S rank, which can be quick battled (quick_expedition_list)
CREATE TABLE daily_quick_stages (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	stage_id     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, stage_id)
);
