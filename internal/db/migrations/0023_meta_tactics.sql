-- META ships' Tactical Research (MetaTacticsInfo): the skill being studied
-- (0 when none) and what the ship did on its last day of study, day being
-- the unix start of that server day: the research exp earned (day_exp, held
-- against gameset meta_skill_exp_max), the part of it earned at double rate
-- (double_exp, against meta_skill_exp_double) and the skill switches made.
-- Each skill's exp towards its next level is ship_skills.exp.
CREATE TABLE ship_meta_tactics (
	ship_id    INTEGER PRIMARY KEY REFERENCES ships (id) ON DELETE CASCADE,
	skill_id   INTEGER NOT NULL DEFAULT 0,
	day        BIGINT  NOT NULL DEFAULT 0,
	day_exp    INTEGER NOT NULL DEFAULT 0,
	double_exp INTEGER NOT NULL DEFAULT 0,
	switches   INTEGER NOT NULL DEFAULT 0
);
