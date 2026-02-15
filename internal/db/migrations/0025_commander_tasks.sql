CREATE TABLE IF NOT EXISTS commander_tasks (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  task_id bigint NOT NULL,
  progress bigint NOT NULL DEFAULT 0,
  accept_time bigint NOT NULL DEFAULT 0,
  submit_time bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, task_id)
);

CREATE TABLE IF NOT EXISTS commander_activity_task_progresses (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  activity_id bigint NOT NULL,
  task_id bigint NOT NULL,
  progress bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (commander_id, activity_id, task_id)
);
