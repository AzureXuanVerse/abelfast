-- accounts of the client's inner login (LoginType.PLATFORM_INNER: the
-- username / password panels, used instead of the SDK when the gateway is on
-- 10.0.x.x or the device has server_config.txt). CS_10001 registers one,
-- CS_10020 with login_type 2 logs into it. account_id is what SC_10021 hands
-- out, the key of commanders.account_id like an SDK account's.
CREATE TABLE inner_accounts (
	username      TEXT    PRIMARY KEY,
	password_hash BLOB    NOT NULL, -- sha256 of the password (the register panel always sends "")
	mail_box      TEXT    NOT NULL DEFAULT '',
	account_id    INTEGER NOT NULL UNIQUE,
	registered_at BIGINT  NOT NULL -- unix seconds
);
