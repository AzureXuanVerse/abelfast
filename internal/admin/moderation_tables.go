package admin

import (
	"sync"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// moderationTables are the config rows the moderation and guild screens
// name things with, read at their first request.
type moderationTables struct {
	// ship groups (ship_data_template group_type) by their first template
	groups map[uint32]named
	// i18n strings the screens show
	tips map[string]string

	operations map[uint32]struct{ Name string }
	events     map[uint32]struct{ Name string }
	bosses     map[uint32]struct{ Name string }
	techs      map[uint32]techRow
	missions   map[uint32]struct {
		Name   string `json:"name"`
		MaxNum uint32 `json:"max_num"`
	}
	contributions map[uint32]struct {
		Name         string `json:"name"`
		AwardCapital uint32 `json:"award_capital"`
	}
	levels map[uint32]struct {
		Exp uint32 `json:"exp"`
	}
}

// guild_technology_template: one level of a technology group
type techRow struct {
	Name     string `json:"name"`
	Group    uint32 `json:"group"`
	Level    uint32 `json:"level"`
	LevelMax uint32 `json:"level_max"`
	Exp      uint32 `json:"exp"`
	NextTech uint32 `json:"next_tech"`
}

var (
	modTablesOnce sync.Once
	modTables     moderationTables
	modTablesErr  error
)

// tables loads the moderation tables once.
func (server *Server) tables() (*moderationTables, error) {
	modTablesOnce.Do(func() { modTablesErr = modTables.load(server.names) })
	return &modTables, modTablesErr
}

func (tables *moderationTables) load(names names) error {
	templates, err := gamedata.Table[struct {
		GroupType uint32 `json:"group_type"`
	}]("ship_data_template")
	if err != nil {
		return err
	}
	first := map[uint32]uint32{}
	for id, row := range templates {
		if at, ok := first[row.GroupType]; row.GroupType != 0 && (!ok || id < at) {
			first[row.GroupType] = id
		}
	}
	tables.groups = make(map[uint32]named, len(first))
	for group, id := range first {
		tables.groups[group] = names[dropShip][id]
	}
	if tables.tips, err = gamedata.Gametip(); err != nil {
		return err
	}
	for _, load := range []func() error{
		gamedata.Load("guild_operation_template", &tables.operations),
		gamedata.Load("guild_base_event", &tables.events),
		gamedata.Load("guild_boss_event", &tables.bosses),
		gamedata.Load("guild_technology_template", &tables.techs),
		gamedata.Load("guild_mission_template", &tables.missions),
		gamedata.Load("guild_contribution_template", &tables.contributions),
		gamedata.Load("guild_data_level", &tables.levels),
	} {
		if err := load(); err != nil {
			return err
		}
	}
	return nil
}

// ship is a ship group's name and icon.
func (tables *moderationTables) ship(group uint32) named {
	if row, ok := tables.groups[group]; ok {
		return row
	}
	return named{Name: "Ship group " + itoa(group)}
}
