-- belfast's own announcements, managed with `belfast notice`. kind 'board'
-- is the in-game bulletin board (SC_11300 NOTICEINFO_P11, ServerNotice in
-- model/vo/notice/servernotice.lua), 'gateway' the popup shown after the
-- gateway login (SC_10021 NOTICEINFO_P10, GatewayNoticeLayer), which only
-- uses id, title and content.
CREATE TABLE notices (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	kind        TEXT    NOT NULL DEFAULT 'board' CHECK (kind IN ('board', 'gateway')),
	version     INTEGER NOT NULL DEFAULT 1, -- the client's image cache key, with id
	btn_title   TEXT    NOT NULL DEFAULT '', -- the board's tab label
	title       TEXT    NOT NULL, -- "list title&page title", or one for both
	title_image TEXT    NOT NULL DEFAULT '', -- [<config type = N param = .../>]url
	time_desc   TEXT    NOT NULL DEFAULT '',
	content     TEXT    NOT NULL DEFAULT '', -- rich text, <banner>url</banner>, <segment/>
	tag_type    INTEGER NOT NULL DEFAULT 1, -- 1 event, 2 system, 3 news tab
	icon        INTEGER NOT NULL DEFAULT 7, -- NewBulletinBoardLayer.ICON_NAME index
	track       TEXT    NOT NULL DEFAULT '',
	priority    INTEGER NOT NULL DEFAULT 0, -- highest first, its tab opens first
	need_level  INTEGER NOT NULL DEFAULT 0 -- shown above this commander level
);
