-- Operational Handover (ChapterAutoProxy, SC_13001 chapter_auto_*)

-- a stage's fastest uninterrupted Auto-Repeat clear (chapter_auto_record_list),
-- the raw seconds: the client applies chapter_auto_statistics' time_rate
-- and time_correction itself. type is ChapterAutoProxy.TYPE (1 campaign).
CREATE TABLE chapter_auto_records (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	type         INTEGER NOT NULL,
	chapter_id   INTEGER NOT NULL,
	seconds      INTEGER NOT NULL,
	PRIMARY KEY (commander_id, type, chapter_id)
);

-- Handover Permits by kind (ChapterAutoTicket.TYPE: 1 campaign, 2 Operation
-- Siren, 3 time) and expiry (unix seconds, 4294967295 never), as
-- chapter_auto_ticket_list sends them
CREATE TABLE chapter_auto_tickets (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	type         INTEGER NOT NULL,
	expire_at    BIGINT  NOT NULL,
	count        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, type, expire_at)
);

-- the running handover, one row per sortie (chapter_auto_battle_list):
-- finish_at its end, ticket_expire_at the expiry of the permit it took (0
-- none), seconds how long it takes
CREATE TABLE chapter_auto_battles (
	commander_id     INTEGER NOT NULL REFERENCES commanders (id),
	seq              INTEGER NOT NULL,
	type             INTEGER NOT NULL,
	chapter_id       INTEGER NOT NULL,
	finish_at        BIGINT  NOT NULL,
	ticket_expire_at BIGINT  NOT NULL,
	seconds          INTEGER NOT NULL,
	PRIMARY KEY (commander_id, seq)
);

-- the refunded oil spent first by the next permits (oil), and the day's
-- handover time used and added by permits (time_used, extra_time, in
-- seconds), which count for the server day starting at day (unix seconds)
CREATE TABLE chapter_auto_states (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id),
	oil          INTEGER NOT NULL DEFAULT 0,
	time_used    INTEGER NOT NULL DEFAULT 0,
	extra_time   INTEGER NOT NULL DEFAULT 0,
	day          BIGINT  NOT NULL DEFAULT 0
);
