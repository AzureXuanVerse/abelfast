CREATE TABLE IF NOT EXISTS commander_friend_relations (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  friend_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, friend_commander_id),
  CHECK (commander_id <> friend_commander_id)
);

CREATE INDEX IF NOT EXISTS idx_commander_friend_relations_friend_commander_id
  ON commander_friend_relations(friend_commander_id);
