CREATE TABLE IF NOT EXISTS commander_coloring_states (
  commander_id BIGINT NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  activity_id BIGINT NOT NULL,
  start_time BIGINT NOT NULL DEFAULT 0,
  cells JSONB NOT NULL DEFAULT '[]'::jsonb,
  awards JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, activity_id)
);
