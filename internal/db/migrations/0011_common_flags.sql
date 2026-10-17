-- one-time client flags (UpdateCommonFlagCommand, CS_11019), sent back in
-- SC_11003.flag_list
CREATE TABLE commander_common_flags (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	flag_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, flag_id)
);
