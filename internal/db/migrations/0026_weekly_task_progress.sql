CREATE TABLE IF NOT EXISTS weekly_task_progresses (
  commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
  week_start_unix bigint NOT NULL DEFAULT 0,
  pt bigint NOT NULL DEFAULT 0,
  reward_lv bigint NOT NULL DEFAULT 0,
  tasks jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
