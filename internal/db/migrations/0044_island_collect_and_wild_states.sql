CREATE TABLE IF NOT EXISTS island_wild_gather_collect_states (
  island_id bigint NOT NULL,
  gather_id bigint NOT NULL,
  collector_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  collected_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (island_id, gather_id)
);

CREATE TABLE IF NOT EXISTS island_collect_fragment_states (
  island_id bigint NOT NULL,
  fragment_id bigint NOT NULL,
  collector_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  mark bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (island_id, fragment_id)
);

CREATE TABLE IF NOT EXISTS island_collect_fragment_sign_states (
  island_id bigint NOT NULL,
  fragment_id bigint NOT NULL,
  signer_commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  mark bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (island_id, fragment_id, signer_commander_id)
);

CREATE TABLE IF NOT EXISTS island_collection_complete_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  collect_id bigint NOT NULL,
  completed_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, collect_id)
);

CREATE TABLE IF NOT EXISTS island_slot_collect_states (
  commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
  build_id bigint NOT NULL,
  area_id bigint NOT NULL,
  slot_type bigint NOT NULL,
  next_refresh_time bigint NOT NULL DEFAULT 0,
  collected_count bigint NOT NULL DEFAULT 0,
  consumed boolean NOT NULL DEFAULT FALSE,
  updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (commander_id, build_id, area_id, slot_type)
);
