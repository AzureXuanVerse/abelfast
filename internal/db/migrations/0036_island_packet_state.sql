CREATE TABLE IF NOT EXISTS island_snapshots (
    commander_id BIGINT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    level BIGINT NOT NULL DEFAULT 1,
    exp BIGINT NOT NULL DEFAULT 0,
    storage_level BIGINT NOT NULL DEFAULT 1,
    prosperity BIGINT NOT NULL DEFAULT 0,
    agora_level BIGINT NOT NULL DEFAULT 1,
    map_id BIGINT NOT NULL DEFAULT 0,
    position_x REAL NOT NULL DEFAULT 0,
    position_y REAL NOT NULL DEFAULT 0,
    position_z REAL NOT NULL DEFAULT 0,
    rotation_x REAL NOT NULL DEFAULT 0,
    rotation_y REAL NOT NULL DEFAULT 0,
    rotation_z REAL NOT NULL DEFAULT 0,
    open_flag BIGINT NOT NULL DEFAULT 0,
    invite_code TEXT NOT NULL DEFAULT '',
    daily_timestamp BIGINT NOT NULL DEFAULT 0,
    follow_ships JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS island_signin_states (
    commander_id BIGINT PRIMARY KEY,
    day_start_unix BIGINT NOT NULL DEFAULT 0,
    signed_in BOOLEAN NOT NULL DEFAULT FALSE,
    external_claim_count BIGINT NOT NULL DEFAULT 0,
    claimed_slots JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS island_technology_states (
    commander_id BIGINT PRIMARY KEY,
    unlocked_tech_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ability_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    finish_counts JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS island_shop_states (
    commander_id BIGINT NOT NULL,
    shop_id BIGINT NOT NULL,
    exist_time BIGINT NOT NULL DEFAULT 0,
    refresh_time BIGINT NOT NULL DEFAULT 0,
    refresh_count BIGINT NOT NULL DEFAULT 0,
    goods JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commander_id, shop_id)
);

CREATE TABLE IF NOT EXISTS island_commander_dresses (
    commander_id BIGINT NOT NULL,
    dress_id BIGINT NOT NULL,
    state BIGINT NOT NULL DEFAULT 0,
    color BIGINT NOT NULL DEFAULT 0,
    color_list JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commander_id, dress_id)
);
