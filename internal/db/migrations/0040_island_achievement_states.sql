CREATE TABLE IF NOT EXISTS island_achievement_states (
    commander_id BIGINT PRIMARY KEY,
    progress_list JSONB NOT NULL DEFAULT '[]'::jsonb,
    finish_list JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
