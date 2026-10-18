-- the operations a commander joined this month without an extra operation
-- (GuildEvent.joinCnt, guildset efficiency_param_times a month): kept on the
-- commander, so a guild disbanded or left with its operations doesn't give
-- them back. operation_month is the month's start (0 none yet).
ALTER TABLE guild_users ADD COLUMN operation_month INTEGER NOT NULL DEFAULT 0;
ALTER TABLE guild_users ADD COLUMN operation_joins INTEGER NOT NULL DEFAULT 0;

-- this month's joins so far, from the operations still stored (months
-- start at midnight UTC-7, the EN servers' clock)
INSERT OR IGNORE INTO guild_users (commander_id)
SELECT DISTINCT commander_id FROM guild_operation_members WHERE NOT extra;
UPDATE guild_users SET
	operation_month = CAST(strftime('%s', datetime('now', '-7 hours', 'start of month', '+7 hours')) AS INTEGER),
	operation_joins = (
		SELECT COUNT(*) FROM guild_operation_members m
		WHERE m.commander_id = guild_users.commander_id AND NOT m.extra
			AND m.joined_at >= CAST(strftime('%s', datetime('now', '-7 hours', 'start of month', '+7 hours')) AS INTEGER)
	)
WHERE commander_id IN (SELECT commander_id FROM guild_operation_members WHERE NOT extra);
