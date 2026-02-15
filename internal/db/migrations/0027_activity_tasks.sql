CREATE TABLE IF NOT EXISTS commander_activity_tasks (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  act_id bigint NOT NULL,
  task_id bigint NOT NULL,
  progress bigint NOT NULL DEFAULT 0,
  submitted boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (commander_id, act_id, task_id)
);
