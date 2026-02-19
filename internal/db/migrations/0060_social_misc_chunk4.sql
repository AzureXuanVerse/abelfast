-- 0059_social_misc_chunk4.sql

CREATE TABLE IF NOT EXISTS friend_relationships (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  friend_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  created_at bigint NOT NULL,
  PRIMARY KEY (commander_id, friend_id),
  CHECK (commander_id < friend_id)
);

CREATE INDEX IF NOT EXISTS idx_friend_relationships_friend_id ON friend_relationships(friend_id);

CREATE TABLE IF NOT EXISTS friend_direct_messages (
  id bigserial PRIMARY KEY,
  sender_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  receiver_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  content text NOT NULL,
  created_at bigint NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_friend_direct_messages_sender_receiver_created_at
  ON friend_direct_messages(sender_id, receiver_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_friend_direct_messages_receiver_created_at
  ON friend_direct_messages(receiver_id, created_at DESC);

CREATE TABLE IF NOT EXISTS player_informs (
  id bigserial PRIMARY KEY,
  reporter_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  target_id bigint NOT NULL,
  info text NOT NULL,
  content text NOT NULL,
  created_at bigint NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_player_informs_reporter_id_created_at ON player_informs(reporter_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_player_informs_target_id_created_at ON player_informs(target_id, created_at DESC);
