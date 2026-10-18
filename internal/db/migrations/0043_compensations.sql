-- a commander's compensations (TIME_REWARD_INFO, model/vo/compensatedata.lua):
-- mails with an expiry, listed apart from the mailbox (CompensateScene) and
-- gone once expired.
CREATE TABLE compensations (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	date         BIGINT  NOT NULL, -- unix seconds it was sent (send_time)
	expires      BIGINT  NOT NULL, -- unix seconds it disappears (timestamp)
	title        TEXT    NOT NULL,
	sender       TEXT    NOT NULL DEFAULT '', -- '' shows mail_sender_default
	content      TEXT    NOT NULL,
	attachments  TEXT    NOT NULL DEFAULT '[]', -- [[drop type, id, number], ...]
	is_claimed   BOOLEAN NOT NULL DEFAULT 0
);

CREATE INDEX compensations_by_commander ON compensations (commander_id, expires);
