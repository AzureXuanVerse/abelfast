-- pt rewards claimed on a commander manual page (CS_22304), sent as
-- TUTHANDBOOK.award: the next one is tutorial_handbook_task target[award]
ALTER TABLE commander_handbook_pages ADD COLUMN award INTEGER NOT NULL DEFAULT 0;
