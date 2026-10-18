-- Exercises (MilitaryExerciseProxy, SC_18002). season is the start of the
-- season the rest counts for (unix seconds): a new season starts over.
-- score is the season's points (the client adds arena_data_rank[1].point),
-- best_rank the highest arena_data_rank id reached this season (its
-- promotions are paid). fight_count is the attempts left, refilled_at the
-- last refill counted. flash_count is the rivals replaced on flash_day (unix
-- seconds, server day start). vanguard / main are the defense fleet's ship
-- ids, empty until the commander sets one; rivals the commander ids of the
-- current opponents. shop_refreshes is the merit shop's manual refreshes on
-- shop_day.
CREATE TABLE commander_exercise (
	commander_id   INTEGER PRIMARY KEY REFERENCES commanders (id),
	season         BIGINT  NOT NULL,
	score          INTEGER NOT NULL DEFAULT 0,
	best_rank      INTEGER NOT NULL DEFAULT 1,
	fight_count    INTEGER NOT NULL,
	refilled_at    BIGINT  NOT NULL,
	flash_count    INTEGER NOT NULL DEFAULT 0,
	flash_day      BIGINT  NOT NULL DEFAULT 0,
	vanguard       TEXT    NOT NULL DEFAULT '[]',
	main           TEXT    NOT NULL DEFAULT '[]',
	rivals         TEXT    NOT NULL DEFAULT '[]',
	shop_refreshes INTEGER NOT NULL DEFAULT 0,
	shop_day       BIGINT  NOT NULL DEFAULT 0
);
