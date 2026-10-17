-- tutorial progress, the old (SG001) and new (SG002) guides count separately
ALTER TABLE commanders ADD COLUMN guide_index INTEGER NOT NULL DEFAULT 1;
ALTER TABLE commanders ADD COLUMN new_guide_index INTEGER NOT NULL DEFAULT 1;

-- stories the commander has seen, so they don't replay
CREATE TABLE commander_stories (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	story_id     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, story_id)
);
