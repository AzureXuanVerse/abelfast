CREATE TABLE IF NOT EXISTS educate_shop_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  shop_id bigint NOT NULL,
  refresh_key bigint NOT NULL DEFAULT 0,
  goods jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, shop_id)
);
