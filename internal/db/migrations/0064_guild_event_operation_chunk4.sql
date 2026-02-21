CREATE TABLE IF NOT EXISTS guild_operation_states (
    guild_id bigint PRIMARY KEY REFERENCES guilds(id) ON DELETE CASCADE,
    chapter_id bigint NOT NULL,
    start_time bigint NOT NULL,
    end_time bigint NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS guild_operation_events (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    event_tid bigint NOT NULL,
    position bigint NOT NULL DEFAULT 1,
    start_time bigint NOT NULL DEFAULT 0,
    complete_time bigint NOT NULL DEFAULT 0,
    efficiency bigint NOT NULL DEFAULT 0,
    completed boolean NOT NULL DEFAULT false,
    shipinevent jsonb NOT NULL DEFAULT '[]'::jsonb,
    attr_acc_list jsonb NOT NULL DEFAULT '[]'::jsonb,
    attr_count_list jsonb NOT NULL DEFAULT '[]'::jsonb,
    eventnodes jsonb NOT NULL DEFAULT '[]'::jsonb,
    personship jsonb NOT NULL DEFAULT '[]'::jsonb,
    formation_time bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (guild_id, event_tid)
);

CREATE TABLE IF NOT EXISTS guild_operation_perfs (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    event_tid bigint NOT NULL,
    perf_index bigint NOT NULL,
    PRIMARY KEY (guild_id, event_tid)
);

CREATE TABLE IF NOT EXISTS guild_operation_participants (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    join_times bigint NOT NULL DEFAULT 0,
    is_participant bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (guild_id, commander_id)
);

CREATE TABLE IF NOT EXISTS guild_reports (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    id bigserial PRIMARY KEY,
    event_id bigint NOT NULL,
    event_type bigint NOT NULL,
    score bigint NOT NULL,
    status bigint NOT NULL,
    claimed boolean NOT NULL DEFAULT false,
    drop_type bigint NOT NULL DEFAULT 1,
    drop_id bigint NOT NULL DEFAULT 1,
    drop_count bigint NOT NULL DEFAULT 0,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_guild_reports_guild_id_id ON guild_reports (guild_id, id);

CREATE TABLE IF NOT EXISTS guild_report_nodes (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    report_id bigint NOT NULL REFERENCES guild_reports(id) ON DELETE CASCADE,
    node_id bigint NOT NULL,
    status bigint NOT NULL,
    PRIMARY KEY (guild_id, report_id, node_id)
);

CREATE TABLE IF NOT EXISTS guild_report_ranks (
    guild_id bigint NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    report_id bigint NOT NULL REFERENCES guild_reports(id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    damage bigint NOT NULL,
    PRIMARY KEY (guild_id, report_id, user_id)
);
