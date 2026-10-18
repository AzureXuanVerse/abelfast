-- a commander's guild state (USER_GUILD_INFO), in a guild or the public
-- one: today's donation tasks (guild_contribution_template ids) and how many
-- donations were made on donate_day (the server midnight they were drawn),
-- the donations and operations bought beyond the daily ones, when the guild
-- supplies were last collected (benefit_time, the client's benefitTime) and
-- the week (its monday_0clock) whose guild mission personal reward was taken.
CREATE TABLE guild_users (
	commander_id     INTEGER PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	donate_day       INTEGER NOT NULL DEFAULT 0,
	donate_count     INTEGER NOT NULL DEFAULT 0,
	donate_tasks     TEXT    NOT NULL DEFAULT '[]',
	extra_donate     INTEGER NOT NULL DEFAULT 0,
	extra_operation  INTEGER NOT NULL DEFAULT 0,
	benefit_time     INTEGER NOT NULL DEFAULT 0,
	weekly_task_week INTEGER NOT NULL DEFAULT 0
);

-- a guild's logistics (GUILD_EXPANSION_INFO): the weekly mission
-- (guild_mission_template id, its progress and the monday_0clock of its
-- week), the supplies (benefit_finish_time and the previous purchase's), and
-- the funds and tech points donations brought on donate_day, for the daily
-- caps.
CREATE TABLE guild_logistics (
	guild_id                 INTEGER PRIMARY KEY REFERENCES guilds (id) ON DELETE CASCADE,
	weekly_task_id           INTEGER NOT NULL DEFAULT 0,
	weekly_task_progress     INTEGER NOT NULL DEFAULT 0,
	weekly_task_week         INTEGER NOT NULL DEFAULT 0,
	benefit_finish_time      INTEGER NOT NULL DEFAULT 0,
	last_benefit_finish_time INTEGER NOT NULL DEFAULT 0,
	donate_day               INTEGER NOT NULL DEFAULT 0,
	donated_capital          INTEGER NOT NULL DEFAULT 0,
	donated_tech             INTEGER NOT NULL DEFAULT 0
);

-- the guild funds log (CAPITAL_LOG, GuildCapitalLog): event_type is
-- GuildConst.TYPE_DONATE, TYPE_SUPPLY, WEEKLY_TASK..., event_target its
-- arguments (the contribution id, the funds a mission brought...).
CREATE TABLE guild_capital_logs (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	member_id    INTEGER NOT NULL,
	name         TEXT    NOT NULL,
	event_type   INTEGER NOT NULL,
	event_target TEXT    NOT NULL DEFAULT '[]',
	time         INTEGER NOT NULL
);
CREATE INDEX guild_capital_logs_guild ON guild_capital_logs (guild_id, id);

-- what a member did in its guild, for the member ranks (GetGuildRankCommand):
-- kind 1 donations, 2 guild missions completed, 3 operations joined; the
-- count this week (since week_start, a monday_0clock), this month (since
-- month_start) and in all.
CREATE TABLE guild_rank_scores (
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL,
	kind         INTEGER NOT NULL,
	week_start   INTEGER NOT NULL,
	week         INTEGER NOT NULL DEFAULT 0,
	month_start  INTEGER NOT NULL,
	month        INTEGER NOT NULL DEFAULT 0,
	total        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (guild_id, commander_id, kind)
);
