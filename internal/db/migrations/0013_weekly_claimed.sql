-- a weekly mission claimed (SubmitWeekTaskCommand) keeps its row, so its
-- chain isn't given again from its first step on a level up
ALTER TABLE commander_weekly_tasks ADD COLUMN claimed BOOLEAN NOT NULL DEFAULT 0;
