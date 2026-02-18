CREATE TABLE IF NOT EXISTS island_book_conds (
  commander_id BIGINT NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  type BIGINT NOT NULL,
  unlock_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, type, unlock_id)
);
