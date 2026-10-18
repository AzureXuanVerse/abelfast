package game

import (
	"github.com/ggmolly/belfast/internal/gamedata"
)

// guild_data_level row: the exp a level needs to go to the next one and the
// members it holds before technologies
type guildLevelConfig struct {
	Level     uint32 `json:"level"`
	Exp       uint32 `json:"exp"`
	MemberNum uint32 `json:"member_num"`
	Deputies  uint32 `json:"assistant_commander"`
}

var (
	// guildSet is the guildset table (pg.guildset[key]), by key
	guildSet map[string]gamedata.Setting
	// guildLevels are the guild_data_level rows, by level
	guildLevels map[uint32]guildLevelConfig
	// guildCreateCost and guildRenameCost are the gems creating a guild
	// and renaming it take (gameset create_guild_cost, modify_guild_cost)
	guildCreateCost, guildRenameCost uint32
)

func loadGuildData() error {
	var err error
	if guildSet, err = gamedata.Settings("guildset"); err != nil {
		return err
	}
	if guildLevels, err = gamedata.Table[guildLevelConfig]("guild_data_level"); err != nil {
		return err
	}
	if _, ok := guildLevels[1]; !ok {
		return gamedata.Missing("guild_data_level", "level 1")
	}
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	var ok bool
	if guildCreateCost, ok = gameset["create_guild_cost"]; !ok {
		return gamedata.Missing("gameset", "create_guild_cost")
	}
	if guildRenameCost, ok = gameset["modify_guild_cost"]; !ok {
		return gamedata.Missing("gameset", "modify_guild_cost")
	}
	return nil
}

// guildSetting reads guildset key's key_value at load time, into the
// typed values handlers use.
func guildSetting(key string, value *uint32) error {
	setting, ok := guildSet[key]
	if !ok {
		return gamedata.Missing("guildset", "%s", key)
	}
	*value = setting.KeyValue
	return nil
}
