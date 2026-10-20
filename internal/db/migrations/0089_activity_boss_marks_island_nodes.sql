-- the activity bosses every commander of the server fights together
-- (ActivityBossActivity, CollabrateBossRushSeriesData): when a boss's HP was
-- first seen at or below a mark, in 1/10000 of its HP (7500, 5000, 2500 and
-- 0 for SC_26032 milestones, 0 for the death times). Unix seconds.
CREATE TABLE activity_boss_marks (
	activity_id INTEGER NOT NULL,
	boss_id     INTEGER NOT NULL,
	mark        INTEGER NOT NULL,
	reached_at  BIGINT  NOT NULL,
	PRIMARY KEY (activity_id, boss_id, mark)
);

-- the island map's nodes (activity_map_event_list, IslandNode, SC_26109):
-- the event each node is at (0 once completed), whether the commander saw
-- it (is_new) and when it was last reset to its first event (refresh nodes
-- start over every day). Unix seconds.
CREATE TABLE commander_island_nodes (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id  INTEGER NOT NULL,
	node_id      INTEGER NOT NULL,
	event_id     INTEGER NOT NULL,
	is_new       BOOLEAN NOT NULL,
	refreshed_at BIGINT  NOT NULL,
	PRIMARY KEY (commander_id, activity_id, node_id)
);
