-- Love letters (LoveLetterProxy, SC_12407): a medal per ship group (exp and
-- level, raised by realizing Valentine gifts and LoveLetterLevelUpCommand),
-- the letters opened (CS_12400), the gifts recorded (CS_12404, replaced as
-- a whole, kept in the client's order) and the total level rewards claimed
-- (CS_12402).
CREATE TABLE commander_love_letter_medals (
	commander_id INTEGER NOT NULL,
	group_id     INTEGER NOT NULL,
	exp          INTEGER NOT NULL,
	level        INTEGER NOT NULL,
	PRIMARY KEY (commander_id, group_id)
);

CREATE TABLE commander_love_letters (
	commander_id INTEGER NOT NULL,
	letter_id    INTEGER NOT NULL, -- lover_letter_content id
	PRIMARY KEY (commander_id, letter_id)
);

CREATE TABLE commander_love_letter_gifts (
	commander_id INTEGER NOT NULL,
	pos          INTEGER NOT NULL, -- index in CS_12404.item_list
	item_id      INTEGER NOT NULL,
	group_id     INTEGER NOT NULL, -- PT_OLD_LOVER_ITEM.group_id as sent
	year         INTEGER NOT NULL,
	PRIMARY KEY (commander_id, pos)
);

CREATE TABLE commander_love_letter_rewards (
	commander_id INTEGER NOT NULL,
	reward_id    INTEGER NOT NULL, -- lover_reward id
	PRIMARY KEY (commander_id, reward_id)
);
