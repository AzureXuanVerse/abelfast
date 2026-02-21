CREATE TABLE IF NOT EXISTS island_treasure_states (
    commander_id BIGINT PRIMARY KEY,
    week_buy_num BIGINT NOT NULL DEFAULT 0,
    sell_list JSONB NOT NULL DEFAULT '[]'::jsonb,
    price_list JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
