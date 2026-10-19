-- a commander's META Showdown state (Operation Siren's world boss,
-- WorldBossProxy): the META Coordinates (virtual item 100000, summon_pt) and
-- Dossier Coordinate Data (100002, summon_pt_old) stored and gained today
-- (the *_daily_acc, counted from acc_day, the unix time of that day's
-- midnight), the Dossier Analysis target (archives_id,
-- world_joint_boss_template id) and the level of the next boss summoned of
-- the current season (self_boss_lv, for self_boss_template_id). The boss
-- the commander summoned, if any (boss_template_id 0: none): its level, hp,
-- expiry and when it was killed (unix seconds), the battles fought against
-- it and the damage dealt, the Dossier auto-deploy end
-- (auto_fight_finish_time, 0 when not running) and the highest damage of one
-- battle (auto_fight_max_damage), and when the support requests sent for it
-- end (unix seconds).
CREATE TABLE world_bosses (
	commander_id            INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	summon_pt               INTEGER NOT NULL DEFAULT 0,
	summon_pt_old           INTEGER NOT NULL DEFAULT 0,
	summon_pt_daily_acc     INTEGER NOT NULL DEFAULT 0,
	summon_pt_old_daily_acc INTEGER NOT NULL DEFAULT 0,
	acc_day                 BIGINT  NOT NULL DEFAULT 0,
	summon_free             INTEGER NOT NULL DEFAULT 0,
	archives_id             INTEGER NOT NULL DEFAULT 0,
	self_boss_lv            INTEGER NOT NULL DEFAULT 1,
	self_boss_template_id   INTEGER NOT NULL DEFAULT 0,
	boss_template_id        INTEGER NOT NULL DEFAULT 0,
	boss_lv                 INTEGER NOT NULL DEFAULT 0,
	boss_hp                 INTEGER NOT NULL DEFAULT 0,
	boss_last_time          BIGINT  NOT NULL DEFAULT 0,
	boss_kill_time          BIGINT  NOT NULL DEFAULT 0,
	boss_fight_count        INTEGER NOT NULL DEFAULT 0,
	boss_damage             INTEGER NOT NULL DEFAULT 0,
	auto_fight_finish_time  BIGINT  NOT NULL DEFAULT 0,
	auto_fight_max_damage   INTEGER NOT NULL DEFAULT 0,
	guild_support           BIGINT  NOT NULL DEFAULT 0,
	friend_support          BIGINT  NOT NULL DEFAULT 0,
	world_support           BIGINT  NOT NULL DEFAULT 0
);

-- the ships of the commander's latest battle against their boss, by
-- position: their formation in the boss's damage ranking (CS_34519).
CREATE TABLE world_boss_ships (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	pos          INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, pos)
);

-- a commander's Sync Points (DROP_TYPE_META_PT) for a META ship group
-- (ship_strengthen_meta id, MetaPTData.count).
CREATE TABLE meta_pts (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	group_id     INTEGER NOT NULL,
	pt           INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, group_id)
);

-- the Sync Point rewards a commander took: the ship_strengthen_meta target
-- (a pt count) of a group (META_SHIP_INFO.fetch_list).
CREATE TABLE meta_pt_awards (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	group_id     INTEGER NOT NULL,
	target       INTEGER NOT NULL,
	PRIMARY KEY (commander_id, group_id, target)
);
