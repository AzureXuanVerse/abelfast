-- the theme shop (NewBackYardThemeTemplateLayer): a custom template
-- (dorm_themes) is found by its DORMTHEME id, the commander id then pos
-- (BackYardBaseThemeTemplate.BuildId), and is shared with every commander
-- of the server from upload_time (unix seconds, 0: not shared,
-- BackYardUploadThemeTemplateCommand CS_19111)
ALTER TABLE dorm_themes ADD COLUMN theme_id TEXT NOT NULL DEFAULT '';
ALTER TABLE dorm_themes ADD COLUMN upload_time BIGINT NOT NULL DEFAULT 0;

UPDATE dorm_themes SET theme_id = CAST(commander_id AS TEXT) || CAST(pos AS TEXT);

CREATE INDEX dorm_themes_theme_id ON dorm_themes (theme_id);

-- the likes (CS_19121) of a shared template, for as long as it is shared
CREATE TABLE dorm_theme_likes (
	theme_id     TEXT    NOT NULL,
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	PRIMARY KEY (theme_id, commander_id)
);

-- a commander's collection of shared templates (CS_19119, at most
-- BackYardConst.MAX_COLLECTION_CNT), with the template's upload_time when
-- collected (DORMTHEME_PROFILE); dropped when the template stops being shared
CREATE TABLE dorm_theme_favs (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	theme_id     TEXT    NOT NULL,
	upload_time  BIGINT  NOT NULL,
	faved_at     BIGINT  NOT NULL, -- unix seconds
	PRIMARY KEY (commander_id, theme_id)
);

CREATE INDEX dorm_theme_favs_theme_id ON dorm_theme_favs (theme_id);
