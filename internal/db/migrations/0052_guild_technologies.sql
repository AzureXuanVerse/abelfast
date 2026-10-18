-- a guild's research (GUILD_TECHNOLOGY, GuildTechnologyGroup): per
-- guild_technology_template group, the template id of the level the guild
-- reached (tech_id) and the points toward the next one. A group without a
-- row is at its level 0. The group being researched is the one active (at
-- most one per guild); started_at is the unix time an administrator chose
-- it (CS_62020), 0 for the default one a new guild starts with: a choice
-- made today is the day's tech_cancel_cnt.
CREATE TABLE guild_technologies (
	guild_id   INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	group_id   INTEGER NOT NULL,
	tech_id    INTEGER NOT NULL,
	progress   INTEGER NOT NULL DEFAULT 0,
	active     BOOLEAN NOT NULL DEFAULT FALSE,
	started_at BIGINT  NOT NULL DEFAULT 0,
	PRIMARY KEY (guild_id, group_id)
);

-- the technologies a commander bought (USER_GUILD_INFO.tech_id), per
-- guild_technology_template group the template id of the level reached;
-- kept when the commander leaves a guild (the public guild reads the same
-- list)
CREATE TABLE commander_guild_technologies (
	commander_id INTEGER NOT NULL REFERENCES commanders (id) ON DELETE CASCADE,
	group_id     INTEGER NOT NULL,
	tech_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, group_id)
);
