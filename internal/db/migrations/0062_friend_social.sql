CREATE TABLE IF NOT EXISTS friend_requests (
    id bigserial PRIMARY KEY,
    requester_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    target_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    content text NOT NULL DEFAULT '',
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (requester_id, target_id)
);

CREATE INDEX IF NOT EXISTS idx_friend_requests_target_id ON friend_requests (target_id);

CREATE TABLE IF NOT EXISTS friend_links (
    commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    friend_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commander_id, friend_id)
);

CREATE INDEX IF NOT EXISTS idx_friend_links_friend_id ON friend_links (friend_id);
