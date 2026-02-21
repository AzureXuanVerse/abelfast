ALTER TABLE guilds
ADD COLUMN IF NOT EXISTS benefit_finish_time bigint NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_benefit_finish_time bigint NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS tech_cancel_cnt bigint NOT NULL DEFAULT 0;

ALTER TABLE guild_user_infos
ADD COLUMN IF NOT EXISTS donate_tasks jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS guild_weekly_tasks (
    guild_id bigint PRIMARY KEY REFERENCES guilds(id) ON DELETE CASCADE,
    task_id bigint NOT NULL DEFAULT 0,
    progress bigint NOT NULL DEFAULT 0,
    monday_0clock bigint NOT NULL DEFAULT 0,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS guild_capital_logs (
    id bigserial PRIMARY KEY,
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    category bigint NOT NULL,
    member_id bigint NOT NULL,
    name varchar(64) NOT NULL DEFAULT '',
    event_type bigint NOT NULL,
    event_target jsonb NOT NULL DEFAULT '[]'::jsonb,
    event_time bigint NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_guild_capital_logs_guild_category_time
ON guild_capital_logs (guild_id, category, event_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS guild_member_ranks (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    rank_type bigint NOT NULL,
    period bigint NOT NULL,
    user_id bigint NOT NULL,
    count bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (guild_id, rank_type, period, user_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_member_ranks_query
ON guild_member_ranks (guild_id, rank_type, period, count DESC, user_id ASC);

CREATE TABLE IF NOT EXISTS guild_user_technology_states (
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    tech_group bigint NOT NULL,
    tech_id bigint NOT NULL,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commander_id, tech_group)
);
