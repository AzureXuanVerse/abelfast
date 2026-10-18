-- when a meowfficer preset was last renamed (unix seconds):
-- CommnaderFleet.RENAME_CODE_TIME apart
ALTER TABLE meowfficer_presets ADD COLUMN rename_time BIGINT NOT NULL DEFAULT 0;

-- when the cattery's cleanliness last lost its daily flower[2]
-- (CommanderHome:ReduceClean, unix seconds; 0 never)
ALTER TABLE meowfficer_states ADD COLUMN clean_time BIGINT NOT NULL DEFAULT 0;
