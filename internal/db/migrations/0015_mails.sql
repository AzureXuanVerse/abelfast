-- a commander's mails (MAIL_INFO, model/vo/mail.lua). Ids grow with arrival:
-- the client pages the inbox by position in id order (MailProxy:GetNewIndex
-- / GetNextIndex); mails moved to the Archive (CS_30006 "move") leave the
-- inbox for the archive list (CS_30004).
CREATE TABLE mails (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	date         BIGINT  NOT NULL, -- unix seconds it arrived
	title        TEXT    NOT NULL,
	sender       TEXT    NOT NULL DEFAULT '', -- '' shows mail_sender_default
	content      TEXT    NOT NULL,
	attachments  TEXT    NOT NULL DEFAULT '[]', -- [[drop type, id, number], ...]
	is_read      BOOLEAN NOT NULL DEFAULT 0,
	is_claimed   BOOLEAN NOT NULL DEFAULT 0, -- attachments taken
	is_important BOOLEAN NOT NULL DEFAULT 0,
	archived     BOOLEAN NOT NULL DEFAULT 0
);

CREATE INDEX mails_by_commander ON mails (commander_id, archived, id);
