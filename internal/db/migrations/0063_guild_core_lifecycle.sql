CREATE TABLE IF NOT EXISTS guilds (
    id bigserial PRIMARY KEY,
    policy bigint NOT NULL,
    faction bigint NOT NULL,
    name varchar(64) NOT NULL,
    level bigint NOT NULL DEFAULT 1,
    announce text NOT NULL DEFAULT '',
    manifesto text NOT NULL DEFAULT '',
    exp bigint NOT NULL DEFAULT 0,
    member_count bigint NOT NULL DEFAULT 0,
    change_faction_cd bigint NOT NULL DEFAULT 0,
    kick_leader_cd bigint NOT NULL DEFAULT 0,
    capital bigint NOT NULL DEFAULT 0,
    tech_id bigint NOT NULL DEFAULT 1000,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_guilds_name_lower_active
ON guilds (LOWER(name))
WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS guild_members (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    duty bigint NOT NULL,
    liveness bigint NOT NULL DEFAULT 0,
    pre_online_time bigint NOT NULL DEFAULT 0,
    join_time bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (guild_id, commander_id),
    UNIQUE (commander_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_members_guild_id ON guild_members (guild_id);

CREATE TABLE IF NOT EXISTS guild_user_infos (
    commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
    guild_id bigint NOT NULL DEFAULT 0,
    donate_count bigint NOT NULL DEFAULT 0,
    benefit_time bigint NOT NULL DEFAULT 0,
    weekly_task_flag bigint NOT NULL DEFAULT 0,
    extra_donate bigint NOT NULL DEFAULT 0,
    extra_operation bigint NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_guild_user_infos_guild_id ON guild_user_infos (guild_id);

CREATE TABLE IF NOT EXISTS commander_guild_states (
    commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
    guild_wait_time bigint NOT NULL DEFAULT 0
);
