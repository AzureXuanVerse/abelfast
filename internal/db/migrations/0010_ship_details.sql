-- the rest of a SHIPINFO
ALTER TABLE ships ADD COLUMN state INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ships ADD COLUMN proficiency INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ships ADD COLUMN skin_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ships ADD COLUMN propose INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ships ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE ships ADD COLUMN change_name_timestamp INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ships ADD COLUMN common_flag INTEGER NOT NULL DEFAULT 0;

-- what a ship has equipped, one row per slot (1-5), equip_id 0 when empty;
-- equip_id is an equip_data_template id
CREATE TABLE ship_equipments (
	ship_id  INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	pos      INTEGER NOT NULL,
	equip_id INTEGER NOT NULL,
	skin_id  INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ship_id, pos)
);

-- learnt skills (skill ids from ship_data_template.buff_list)
CREATE TABLE ship_skills (
	ship_id  INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	skill_id INTEGER NOT NULL,
	level    INTEGER NOT NULL,
	exp      INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ship_id, skill_id)
);
