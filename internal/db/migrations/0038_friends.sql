-- unix seconds a commander last logged in or out, sent to their friends as
-- FRIEND_INFO.pre_online_time
ALTER TABLE commanders ADD COLUMN last_online BIGINT NOT NULL DEFAULT 0;

-- friendships (FriendProxy, protocol p50), one row per side
CREATE TABLE friends (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	friend_id    INTEGER NOT NULL REFERENCES commanders (id),
	since        BIGINT  NOT NULL, -- unix seconds the request was accepted
	PRIMARY KEY (commander_id, friend_id)
);

-- friend requests waiting for commander_id to accept or reject them
-- (NotificationProxy, SC_50000 request_list / SC_50005), one per sender
CREATE TABLE friend_requests (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	sender_id    INTEGER NOT NULL REFERENCES commanders (id),
	content      TEXT    NOT NULL, -- the note typed with the request
	sent_at      BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (commander_id, sender_id)
);

-- the players commander_id blocked (FriendProxy blackList, CS_50016)
CREATE TABLE blacklist (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	blocked_id   INTEGER NOT NULL REFERENCES commanders (id),
	blocked_at   BIGINT  NOT NULL, -- unix seconds, the list is sent in this order
	PRIMARY KEY (commander_id, blocked_id)
);
