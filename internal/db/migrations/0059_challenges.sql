-- an activity's own fleets (ACTIVITYINFO.group_list, EditActivityFleetCommand
-- CS_11204): ships in fleet order, meowfficers by position
CREATE TABLE activity_fleet_ships (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id INTEGER NOT NULL,
	fleet_id INTEGER NOT NULL,
	pos INTEGER NOT NULL,
	ship_id INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, fleet_id, pos)
);

CREATE TABLE activity_fleet_meowfficers (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id INTEGER NOT NULL,
	fleet_id INTEGER NOT NULL,
	pos INTEGER NOT NULL,
	meowfficer_id INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, fleet_id, pos)
);

-- a challenge mode run (USERCHALLENGEINFO) of an activity, one per mode
-- (ChallengeProxy.MODE_CASUAL 0, MODE_INFINITE 1): the season it started
-- in, the level being fought, the score so far; battling is set by a battle
-- started and never settled (issl, the save/load the client offers to reset)
CREATE TABLE challenge_runs (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id INTEGER NOT NULL,
	mode INTEGER NOT NULL,
	season INTEGER NOT NULL,
	level INTEGER NOT NULL DEFAULT 1,
	score INTEGER NOT NULL DEFAULT 0,
	battling BOOLEAN NOT NULL DEFAULT FALSE,
	PRIMARY KEY (commander_id, activity_id, mode)
);

-- the fleets a run was started with (GROUPINFOINCHALLENGE) and their ships'
-- hp, carried from battle to battle
CREATE TABLE challenge_ships (
	commander_id INTEGER NOT NULL,
	activity_id INTEGER NOT NULL,
	mode INTEGER NOT NULL,
	fleet_id INTEGER NOT NULL,
	pos INTEGER NOT NULL,
	ship_id INTEGER NOT NULL,
	hp_rant INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, mode, fleet_id, pos),
	FOREIGN KEY (commander_id, activity_id, mode) REFERENCES challenge_runs (commander_id, activity_id, mode) ON DELETE CASCADE
);

CREATE TABLE challenge_meowfficers (
	commander_id INTEGER NOT NULL,
	activity_id INTEGER NOT NULL,
	mode INTEGER NOT NULL,
	fleet_id INTEGER NOT NULL,
	pos INTEGER NOT NULL,
	meowfficer_id INTEGER NOT NULL,
	PRIMARY KEY (commander_id, activity_id, mode, fleet_id, pos),
	FOREIGN KEY (commander_id, activity_id, mode) REFERENCES challenge_runs (commander_id, activity_id, mode) ON DELETE CASCADE
);

-- the best casual score and levels cleared (CHALLENGEINFO), of the activity
-- (season 0) and of each season
CREATE TABLE challenge_records (
	commander_id INTEGER NOT NULL REFERENCES commanders (id),
	activity_id INTEGER NOT NULL,
	season INTEGER NOT NULL,
	max_score INTEGER NOT NULL DEFAULT 0,
	max_level INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (commander_id, activity_id, season)
);
