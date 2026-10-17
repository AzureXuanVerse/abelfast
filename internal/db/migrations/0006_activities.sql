-- per-commander state (ACTIVITYINFO data fields) of the activities the
-- server changes, e.g. the 7-day login and the monthly sign-in.
-- data1_list is comma separated.
CREATE TABLE commander_activities (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	data1        INTEGER NOT NULL,
	data2        INTEGER NOT NULL,
	data3        INTEGER NOT NULL,
	data4        INTEGER NOT NULL,
	data1_list   TEXT    NOT NULL,
	PRIMARY KEY (commander_id, activity_id)
);
