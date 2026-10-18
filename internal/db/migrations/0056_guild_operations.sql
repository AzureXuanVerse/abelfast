-- a guild operation (CURRENT_OPERATION, GuildEvent): operation_id the
-- guild_operation_template row, open from start_time to end_time
-- (guildset operation_duration_time later). boss_id is the
-- guild_boss_event that appears once the last main mission is done
-- (boss_active), with boss_hp to deal; ended once the boss is defeated.
CREATE TABLE guild_operations (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	operation_id INTEGER NOT NULL,
	start_time   INTEGER NOT NULL,
	end_time     INTEGER NOT NULL,
	boss_id      INTEGER NOT NULL,
	boss_hp      INTEGER NOT NULL,
	boss_damage  INTEGER NOT NULL DEFAULT 0,
	boss_active  BOOLEAN NOT NULL DEFAULT 0,
	ended        BOOLEAN NOT NULL DEFAULT 0
);
CREATE INDEX guild_operations_guild ON guild_operations (guild_id, id);

-- an operation's missions (EVENT_BASE, GuildMission): event_id the
-- guild_base_event row at a position of the tree. Progress is computed
-- lazily: work is the workload done when updated_at was reached, going on
-- at the speed efficiency gives (guildMissionSpeed). start_time is the first
-- dispatch (0 not started), finished_at when work reached the workload
-- (0 not yet), score the report's grade (GuildReport.SCORE_TYPE_*).
CREATE TABLE guild_operation_missions (
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	event_id     INTEGER NOT NULL,
	position     INTEGER NOT NULL,
	start_time   INTEGER NOT NULL DEFAULT 0,
	work         REAL    NOT NULL DEFAULT 0,
	updated_at   INTEGER NOT NULL DEFAULT 0,
	efficiency   INTEGER NOT NULL DEFAULT 0,
	finished_at  INTEGER NOT NULL DEFAULT 0,
	score        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (operation_id, event_id)
);

-- the nodes along a mission's progress bar (EVENT_NODE): node_id the
-- guild_event_node row, position the percentage it sits at, status
-- GuildMissionNode.STATE_* (0 ahead, 1 success, 2 failure)
CREATE TABLE guild_operation_nodes (
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	event_id     INTEGER NOT NULL,
	position     INTEGER NOT NULL,
	node_id      INTEGER NOT NULL,
	status       INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (operation_id, event_id, position)
);

-- the ships a member dispatched to a mission (SHIP_IN_EVENT,
-- PERSON_SHIP_IN_PAGE): fleet the page (1 to GuildConst.MISSION_MAX_FLEET_CNT),
-- the ship as it was then, and the fleet's efficiency kept on its first ship
CREATE TABLE guild_operation_ships (
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	event_id     INTEGER NOT NULL,
	commander_id INTEGER NOT NULL,
	fleet        INTEGER NOT NULL,
	slot         INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	template_id  INTEGER NOT NULL,
	skin_id      INTEGER NOT NULL,
	level        INTEGER NOT NULL,
	efficiency   INTEGER NOT NULL DEFAULT 0,
	dispatched_at INTEGER NOT NULL,
	PRIMARY KEY (operation_id, event_id, commander_id, fleet, slot)
);

-- a member taking part in an operation (GuildJoinEventCommand): extra when
-- the join took an extra operation (USER_GUILD_INFO.extra_operation), the
-- boss fights of boss_day (a server day's start), the damage dealt and
-- when the boss battle in progress began (0 none)
CREATE TABLE guild_operation_members (
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL,
	joined_at    INTEGER NOT NULL,
	extra        BOOLEAN NOT NULL DEFAULT 0,
	boss_day     INTEGER NOT NULL DEFAULT 0,
	boss_count   INTEGER NOT NULL DEFAULT 0,
	boss_damage  INTEGER NOT NULL DEFAULT 0,
	boss_battle_at INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (operation_id, commander_id)
);
CREATE INDEX guild_operation_members_commander ON guild_operation_members (commander_id, joined_at);

-- the node a member last watched a mission's animation up to
-- (EVENT_PERFORMANCE, GuildUpdateNodeAnimFlagCommand)
CREATE TABLE guild_operation_perfs (
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL,
	event_id     INTEGER NOT NULL,
	idx          INTEGER NOT NULL,
	PRIMARY KEY (operation_id, commander_id, event_id)
);

-- a member's boss fleets (BOSSEVENTFLEET, fleet_id GuildBossMission
-- MAIN_FLEET_ID 1 / SUB_FLEET_ID 11): ships (user_id the owner, another
-- member's for a support ship) and meowfficers by position
CREATE TABLE guild_boss_fleet_ships (
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL,
	fleet_id     INTEGER NOT NULL,
	slot         INTEGER NOT NULL,
	user_id      INTEGER NOT NULL,
	ship_id      INTEGER NOT NULL,
	PRIMARY KEY (operation_id, commander_id, fleet_id, slot)
);
CREATE TABLE guild_boss_fleet_meowfficers (
	operation_id  INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	commander_id  INTEGER NOT NULL,
	fleet_id      INTEGER NOT NULL,
	pos           INTEGER NOT NULL,
	meowfficer_id INTEGER NOT NULL,
	PRIMARY KEY (operation_id, commander_id, fleet_id, pos)
);

-- a guild's operation reports (REPORT): a finished mission's (event_type
-- GuildConst.REPORT_TYPE_MISSION, event_id its guild_base_event) or the
-- defeated boss's (REPORT_TYPE_BOSS, its guild_boss_event)
CREATE TABLE guild_reports (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	guild_id     INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
	operation_id INTEGER NOT NULL REFERENCES guild_operations (id) ON DELETE CASCADE,
	event_id     INTEGER NOT NULL,
	event_type   INTEGER NOT NULL,
	score        INTEGER NOT NULL,
	time         INTEGER NOT NULL
);
CREATE INDEX guild_reports_guild ON guild_reports (guild_id, id);

-- who may claim a report (GuildConst.REPORT_STATE_UNlOCK 1, the members
-- that took part in it) and who did (REPORT_STATE_SUBMITED 2); the others
-- see it locked
CREATE TABLE guild_report_claims (
	report_id    INTEGER NOT NULL REFERENCES guild_reports (id) ON DELETE CASCADE,
	commander_id INTEGER NOT NULL,
	status       INTEGER NOT NULL,
	PRIMARY KEY (report_id, commander_id)
);
