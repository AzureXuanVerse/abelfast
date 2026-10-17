-- the backyard's food (SC_19001.food), filled by the usage_food items
-- (UseItemCommand, CS_15002)
ALTER TABLE commanders ADD COLUMN dorm_food INTEGER NOT NULL DEFAULT 0;
