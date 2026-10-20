-- City Rebuild activities (ACTIVITY_TYPE_CITY_REBUILD, CityRebuildProxy,
-- SC_26061 NINJA_INFO): a commander's city per activity. pt is the Ninja
-- Koban held, summary_pt the koban the defense earned not collected yet
-- (CS_26068). cur_level is the wave fought, max_level the highest wave
-- defeated, max_display the highest wave chosen. The wave fought started at
-- fight_time (unix seconds, 0 until CS_26072) with left_hp HP left (0: full).
-- awarded_level is the highest wave whose first defeat rewards were given.
CREATE TABLE commander_city_rebuild (
	commander_id  INTEGER NOT NULL REFERENCES commanders (id),
	activity_id   INTEGER NOT NULL,
	pt            BIGINT  NOT NULL DEFAULT 0,
	summary_pt    BIGINT  NOT NULL DEFAULT 0,
	max_level     INTEGER NOT NULL DEFAULT 0,
	cur_level     INTEGER NOT NULL DEFAULT 1,
	max_display   INTEGER NOT NULL DEFAULT 1,
	fight_time    BIGINT  NOT NULL DEFAULT 0,
	left_hp       BIGINT  NOT NULL DEFAULT 0,
	awarded_level INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, activity_id)
);

-- the facilities repaired and the troops recruited (activity_ninja_building
-- ids): done once repaired / recruited, otherwise recruiting since
-- start_time (unix seconds)
CREATE TABLE commander_city_rebuild_units (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	unit_id      INTEGER NOT NULL,
	done         BOOLEAN NOT NULL DEFAULT 0,
	start_time   BIGINT  NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, activity_id, unit_id)
);

-- the level of each defense buff (activity_ninja_buff group); a group
-- without a row is at level 1
CREATE TABLE commander_city_rebuild_buffs (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	buff_group   INTEGER NOT NULL,
	level        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, buff_group)
);
