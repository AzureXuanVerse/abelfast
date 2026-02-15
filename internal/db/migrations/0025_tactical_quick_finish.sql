CREATE TABLE IF NOT EXISTS commander_tactics_quick_finishes (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  used_count bigint NOT NULL DEFAULT 0,
  reset_day bigint NOT NULL DEFAULT 0
);
