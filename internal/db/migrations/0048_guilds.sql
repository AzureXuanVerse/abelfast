-- a guild (Guild.New, GUILD_BASE_INFO): policy 1 power / 2 relaxation,
-- faction 1 Azur Lane / 2 Crimson Axis (GuildConst), exp and level from
-- guild_data_level, capital the guild funds (GUILD_EXPANSION_INFO.capital).
-- change_faction_cd and kick_leader_cd are unix times (0 none).
CREATE TABLE guilds (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	name              TEXT    NOT NULL UNIQUE,
	policy            INTEGER NOT NULL,
	faction           INTEGER NOT NULL,
	manifesto         TEXT    NOT NULL DEFAULT '',
	announce          TEXT    NOT NULL DEFAULT '',
	level             INTEGER NOT NULL DEFAULT 1,
	exp               INTEGER NOT NULL DEFAULT 0,
	capital           INTEGER NOT NULL DEFAULT 0,
	change_faction_cd INTEGER NOT NULL DEFAULT 0,
	kick_leader_cd    INTEGER NOT NULL DEFAULT 0,
	created_at        INTEGER NOT NULL
);

-- guild ids start where the official ones do (6 digits), not at 1
INSERT INTO sqlite_sequence (name, seq) VALUES ('guilds', 100000);

-- a commander's membership (MEMBER_INFO): one guild at a time. duty is
-- GuildConst.DUTY_* (1 commander, 2 deputy, 3 picked, 4 ordinary, 5
-- recruit), liveness the member's activity points.
CREATE TABLE guild_members (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	duty         INTEGER NOT NULL,
	liveness     INTEGER NOT NULL DEFAULT 0,
	join_time    INTEGER NOT NULL
);
CREATE INDEX guild_members_guild ON guild_members (guild_id);

-- a commander's application to a guild (GuildApplyCommand), until an
-- administrator accepts or rejects it
CREATE TABLE guild_requests (
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	content      TEXT    NOT NULL,
	time         INTEGER NOT NULL,
	PRIMARY KEY (guild_id, commander_id)
);

-- the guild's log (LOG_INFO, GuildLogInfo): cmd is GuildConst.CMD_TYPE_*,
-- name the member's name at the time, arg1 the new duty of a SET_DUTY
CREATE TABLE guild_logs (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	cmd      INTEGER NOT NULL,
	time     INTEGER NOT NULL,
	user_id  INTEGER NOT NULL,
	name     TEXT    NOT NULL,
	arg1     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX guild_logs_guild ON guild_logs (guild_id, id);

-- the guild chat (GUIDE_CHAT), newest kept up to GuildConst.CHAT_LOG_MAX_COUNT
CREATE TABLE guild_chat (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL,
	content      TEXT    NOT NULL,
	time         INTEGER NOT NULL
);
CREATE INDEX guild_chat_guild ON guild_chat (guild_id, id);
