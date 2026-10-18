-- when a commander may join a guild again (SC_11003.guild_wait_time, a unix
-- time): GuildQuitCommand and GuildDissolveCommand set it to a day after
-- leaving (guild_join_cd, guild_player_in_cd_time)
CREATE TABLE guild_wait_times (
	commander_id INTEGER PRIMARY KEY REFERENCES commanders (id) ON DELETE CASCADE,
	until        INTEGER NOT NULL
);
