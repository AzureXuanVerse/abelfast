-- How many daily-slot commissions were drawn on the server day of
-- refreshed_at: the wiki's Commissions page says the first 10 commissions
-- a day brings are Daily Resource Extraction ones, the extra ones after.
ALTER TABLE commander_commission_refresh ADD COLUMN daily_drawn INTEGER NOT NULL DEFAULT 0;
