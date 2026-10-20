-- what the commander shows (SC_11003, DISPLAYINFO): the icon and chat frames
-- applied (CS_11005 types 2, 3; 0 the default appearance), the combat UI
-- (type 4, SC_11003.battle_ui), the port wallpaper (CS_11030,
-- SC_11003.cover.id, 0 the default) and the loading screen pictures
-- (CS_11034: loading_pic_open is the custom mode, gallery_pics
-- gallery_config ids, manga_pics cartoon ids); no row is all defaults.
-- The wallpapers and combat UIs owned are attire_frames rows of their drop
-- type (DROP_TYPE_LIVINGAREA_COVER 29, DROP_TYPE_COMBAT_UI_STYLE 31).
CREATE TABLE commander_appearance (
	commander_id     INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	icon_frame       INTEGER NOT NULL DEFAULT 0,
	chat_frame       INTEGER NOT NULL DEFAULT 0,
	battle_ui        INTEGER NOT NULL DEFAULT 0,
	cover            INTEGER NOT NULL DEFAULT 0,
	loading_pic_open BOOLEAN NOT NULL DEFAULT FALSE,
	gallery_pics     TEXT    NOT NULL DEFAULT '[]',
	manga_pics       TEXT    NOT NULL DEFAULT '[]'
);

-- the secondary password (SecondaryPWDProxy, CS_11605): password_hash is
-- pbkdf2-sha256 "<salt hex>$<hash hex>", notice the hint, system_list the
-- protected operations (SecondaryPWDMgr ids) in the client's order;
-- fail_count the wrong passwords in a row, fail_cd until when (unix
-- seconds) it is locked out after too many. No row is no password.
CREATE TABLE commander_secondary_passwords (
	commander_id  INTEGER NOT NULL PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	password_hash TEXT    NOT NULL,
	notice        TEXT    NOT NULL DEFAULT '',
	system_list   TEXT    NOT NULL DEFAULT '[]',
	fail_count    INTEGER NOT NULL DEFAULT 0,
	fail_cd       BIGINT  NOT NULL DEFAULT 0
);
