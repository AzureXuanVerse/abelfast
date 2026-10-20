-- medal_template tiers claimed (CS_17301, TrophyClaimCommand): the time is
-- the Trophy timestamp, a tier's next one is unlocked once it is claimed
CREATE TABLE commander_trophy_claims (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	trophy_id    INTEGER NOT NULL,
	claimed_at   BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (commander_id, trophy_id)
);

-- the medals shown on the commander's profile (CS_17401,
-- ChangeMedalDisplayCommand; SC_11003.medal_id), in the chosen order:
-- claimed medal_template ids or love letter medals (LoveLetterTrophy ids)
CREATE TABLE commander_display_medals (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	position     INTEGER NOT NULL,
	medal_id     INTEGER NOT NULL,
	PRIMARY KEY (commander_id, position)
);
