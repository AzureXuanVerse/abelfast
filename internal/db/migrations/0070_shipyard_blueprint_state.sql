CREATE TABLE IF NOT EXISTS commander_shipyard_states (
    commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
    cold_time bigint NOT NULL DEFAULT 0,
    daily_catchup_strengthen bigint NOT NULL DEFAULT 0,
    daily_catchup_strengthen_ur bigint NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS commander_shipyard_blueprints (
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    blueprint_id bigint NOT NULL,
    ship_id bigint NOT NULL DEFAULT 0,
    start_time bigint NOT NULL DEFAULT 0,
    blue_print_level bigint NOT NULL DEFAULT 0,
    exp bigint NOT NULL DEFAULT 0,
    start_duration bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (commander_id, blueprint_id)
);

CREATE INDEX IF NOT EXISTS idx_commander_shipyard_blueprints_commander
    ON commander_shipyard_blueprints (commander_id);
