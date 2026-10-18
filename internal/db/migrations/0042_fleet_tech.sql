-- Fleet Technology (TechnologyNationProxy, SC_64000)

-- a fleet_tech_group's state (tech_list): effect_tech_id the last level
-- researched (0 none), study_tech_id the level being researched (0 none) and
-- study_finish_at when it ends (unix seconds, 0 none), rewarded_tech_id the
-- last level whose level_award_display was claimed (0 none)
CREATE TABLE fleet_techs (
	commander_id     INTEGER NOT NULL REFERENCES commanders (id),
	group_id         INTEGER NOT NULL,
	effect_tech_id   INTEGER NOT NULL DEFAULT 0,
	study_tech_id    INTEGER NOT NULL DEFAULT 0,
	study_finish_at  BIGINT  NOT NULL DEFAULT 0,
	rewarded_tech_id INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, group_id)
);

-- the attribute bonuses set below their maximum (techset_list): ship_type a
-- ShipType, attr_type an index of TechnologyConst.TECH_NATION_ATTRS
CREATE TABLE fleet_tech_attrs (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	ship_type    INTEGER NOT NULL,
	attr_type    INTEGER NOT NULL,
	set_value    INTEGER NOT NULL,
	PRIMARY KEY (commander_id, ship_type, attr_type)
);
