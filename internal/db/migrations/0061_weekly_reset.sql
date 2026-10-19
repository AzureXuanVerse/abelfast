-- the week (unix seconds of its Monday 00:00, server time) the weekly
-- missions and bonus tasks were handed out for; 0 until it is first known,
-- then the commander's tasks are reset when a new week starts
ALTER TABLE commander_weekly ADD COLUMN week_start INTEGER NOT NULL DEFAULT 0;
