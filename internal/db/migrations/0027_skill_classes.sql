-- the Tactical Class (NavalAcademyProxy students, SC_22001.skill_class_list):
-- a ship studying one of its skills with a lesson book (item_data_statistics
-- type 10) in a room, 1 to the commander's skill_class_num. exp is what the
-- whole lesson gives, the book's type bonus included.
CREATE TABLE commander_skill_classes (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	room_id      INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL UNIQUE REFERENCES ships (id) ON DELETE CASCADE,
	skill_id     INTEGER NOT NULL,
	item_id      INTEGER NOT NULL,
	start_time   BIGINT  NOT NULL, -- unix seconds
	finish_time  BIGINT  NOT NULL,
	exp          INTEGER NOT NULL,
	PRIMARY KEY (commander_id, room_id)
);

-- rooms open (two to start with, captured; shop_template skill_room_pos adds
-- one) and the lessons finished instantly on quick_finish_day (unix seconds of
-- its midnight, server time) with a skill_learn_time buff
ALTER TABLE commanders ADD COLUMN skill_class_num INTEGER NOT NULL DEFAULT 2;
ALTER TABLE commanders ADD COLUMN quick_finish_day BIGINT NOT NULL DEFAULT 0;
ALTER TABLE commanders ADD COLUMN quick_finishes INTEGER NOT NULL DEFAULT 0;
