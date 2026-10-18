-- Cryptolalia voice stories unlocked (UnlockCryptolaliaCommand, CS_16205),
-- soundstory_template ids sent back in SC_11003.soundstory
CREATE TABLE commander_cryptolalia (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	story_id     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, story_id)
);
