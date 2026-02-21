CREATE TABLE IF NOT EXISTS technology_research_states (
  commander_id BIGINT PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  refresh_flag BIGINT NOT NULL DEFAULT 0,
  refresh_day BIGINT NOT NULL DEFAULT 0,
  catchup_version BIGINT NOT NULL DEFAULT 0,
  catchup_target BIGINT NOT NULL DEFAULT 0,
  refresh_pools JSONB NOT NULL DEFAULT '[]'::jsonb,
  queue JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
