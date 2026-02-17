CREATE TABLE IF NOT EXISTS island_wild_gather_sign_states (
  island_id bigint NOT NULL,
  gather_id bigint NOT NULL,
  signer_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  mark bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (island_id, gather_id, signer_commander_id)
);
