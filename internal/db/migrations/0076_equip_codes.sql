-- gear codes shared for a ship group (EquipCodeShareLayer,
-- EQCODE_SHARE_INFO): seen by every commander of the server, keyed by ship
-- group.
CREATE TABLE equip_codes (
	id           INTEGER PRIMARY KEY AUTOINCREMENT, -- EQCODE_SHARE_INFO id
	group_id     INTEGER NOT NULL, -- ship_data_template.group_type
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	eqcode       TEXT    NOT NULL, -- <gear>&<group, base 32>&<tag>&<tag>
	likes        INTEGER NOT NULL DEFAULT 0, -- EQCODE_SHARE_INFO like
	shared_at    BIGINT  NOT NULL -- unix seconds
);

CREATE INDEX equip_codes_group ON equip_codes (group_id, shared_at);
CREATE INDEX equip_codes_commander ON equip_codes (commander_id, shared_at);

-- a commander's latest like of a code, one a day (equipcode_like_limited)
CREATE TABLE equip_code_likes (
	share_id     INTEGER NOT NULL REFERENCES equip_codes (id),
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	liked_at     BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (share_id, commander_id)
);

CREATE INDEX equip_code_likes_time ON equip_code_likes (share_id, liked_at);

-- reports (CS_17607), kept for whoever runs the server: a commander's
-- latest report_type per code
CREATE TABLE equip_code_reports (
	share_id     INTEGER NOT NULL REFERENCES equip_codes (id),
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	report_type  INTEGER NOT NULL, -- 1 nonsense, 2 outdated
	reported_at  BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (share_id, commander_id)
);

CREATE INDEX equip_code_reports_commander ON equip_code_reports (commander_id, reported_at);
