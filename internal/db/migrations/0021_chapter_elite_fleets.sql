-- the fleets set up for the chapters that pick their own (chapter_template
-- type 2) or take a support fleet, per chapter_template.formation: the
-- chapters of a formation share them (ChapterProxy.mapEliteFleetCache)
CREATE TABLE chapter_elite_fleets (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	formation    INTEGER NOT NULL,
	fleet        BLOB    NOT NULL, -- FLEET_INFO (CS_13107 / SC_13001 fleet_list), its id is the formation
	PRIMARY KEY (commander_id, formation)
);
