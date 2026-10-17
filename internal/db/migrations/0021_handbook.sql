-- commander manual pages (SC_22300): tasks finished on each page, pt (one
-- per finished page task) and how many pt rewards were claimed
CREATE TABLE commander_handbook_pages (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	page_id      INTEGER NOT NULL, -- tutorial_handbook_task id
	pt           INTEGER NOT NULL DEFAULT 0,
	award        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, page_id)
);

CREATE TABLE commander_handbook_finished (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	page_id      INTEGER NOT NULL,
	task_id      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, page_id, task_id)
);
