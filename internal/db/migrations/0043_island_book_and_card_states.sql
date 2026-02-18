CREATE TABLE IF NOT EXISTS island_card_states (
    commander_id BIGINT PRIMARY KEY,
    picture TEXT NOT NULL DEFAULT '',
    visit_word TEXT NOT NULL DEFAULT '',
    social_flag BIGINT NOT NULL DEFAULT 1,
    label_view_flag BIGINT NOT NULL DEFAULT 1,
    label_counts JSONB NOT NULL DEFAULT '[]'::jsonb,
    achieve_display_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    visit_num BIGINT NOT NULL DEFAULT 0,
    good_num BIGINT NOT NULL DEFAULT 0,
    ship_num BIGINT NOT NULL DEFAULT 0,
    book_num BIGINT NOT NULL DEFAULT 0,
    achieve_num BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS island_card_likes (
    from_commander_id BIGINT NOT NULL,
    to_commander_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (from_commander_id, to_commander_id)
);

CREATE TABLE IF NOT EXISTS island_card_label_gifts (
    from_commander_id BIGINT NOT NULL,
    to_commander_id BIGINT NOT NULL,
    label_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (from_commander_id, to_commander_id)
);

CREATE TABLE IF NOT EXISTS island_book_states (
    commander_id BIGINT PRIMARY KEY,
    book_list JSONB NOT NULL DEFAULT '[]'::jsonb,
    book_awards JSONB NOT NULL DEFAULT '[]'::jsonb,
    book_collects JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
