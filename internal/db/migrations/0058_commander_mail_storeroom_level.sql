ALTER TABLE commanders
ADD COLUMN IF NOT EXISTS mail_storeroom_lv bigint NOT NULL DEFAULT 1;
