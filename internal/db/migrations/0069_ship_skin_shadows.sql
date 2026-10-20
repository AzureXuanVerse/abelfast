-- the skin shadows (technology phantoms) of a research ship the commander
-- unlocked by finishing its phantom quest (CS_12210, technology_shadow_unlock
-- id = shadow id), and the skin each wears (CS_12202 skin_shadow), 0 for the
-- ship's default one. Sent back in SHIPINFO.skin_shadow_list.
CREATE TABLE ship_skin_shadows (
	ship_id   INTEGER NOT NULL REFERENCES ships (id) ON DELETE CASCADE,
	shadow_id INTEGER NOT NULL,
	skin_id   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ship_id, shadow_id)
);
