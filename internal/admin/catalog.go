package admin

import (
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The pickers' catalogs: everything of a kind at once, with icons, for the
// web app to search fuzzily on its side. They only change with the game
// data, so the app keeps them for the session.

// shipEntry is a ship as the ship picker lists it: one per ship group, as
// the template a new ship of it starts as (its lowest).
type shipEntry struct {
	ID       uint32 `json:"id"`
	Group    uint32 `json:"group"`
	Name     string `json:"name"`
	Rarity   uint32 `json:"rarity"`
	Icon     string `json:"icon,omitempty"`
	Type     uint32 `json:"type"`
	TypeName string `json:"type_name"`
	Nation   string `json:"nation"`
}

// skinEntry is a ship skin with the ship it's for.
type skinEntry struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Group    uint32 `json:"group"`
	ShipName string `json:"ship_name"`
	Icon     string `json:"icon,omitempty"`
}

// shipStats and skinShip are what the pickers read of a ship's and a
// skin's template beyond its name.
type shipStats struct {
	Type        uint32 `json:"type"`
	Nationality uint32 `json:"nationality"`
}

type skinShip struct {
	ShipGroup uint32 `json:"ship_group"`
	// -1 for a ship's own look (its template's), not a skin to own
	SkinType int32 `json:"skin_type"`
}

// equipLevel is a piece of gear's enhancement level + 1: the +0 pieces,
// the ones granted, are level 1.
type equipLevel struct {
	Level uint32 `json:"level"`
}

type pickerTables struct {
	ships  map[uint32]shipStats
	skins  map[uint32]skinShip
	equips map[uint32]equipLevel
}

var catalogTables = sync.OnceValues(func() (*pickerTables, error) {
	ships, err := gamedata.Table[shipStats]("ship_data_statistics")
	if err != nil {
		return nil, err
	}
	skins, err := gamedata.Table[skinShip]("ship_skin_template")
	if err != nil {
		return nil, err
	}
	equips, err := gamedata.Table[equipLevel]("equip_data_template")
	if err != nil {
		return nil, err
	}
	return &pickerTables{ships: ships, skins: skins, equips: equips}, nil
})

// handleCatalogShips lists every ship group, for the ship picker.
func (server *Server) handleCatalogShips(r *http.Request) (any, error) {
	tables, err := catalogTables()
	if err != nil {
		return nil, err
	}
	labels, err := dockNames()
	if err != nil {
		return nil, err
	}
	byGroup := map[uint32]shipEntry{}
	for id, ship := range server.names[dropShip] {
		group := id / 10
		if ship.Name == "" {
			continue
		}
		if known, ok := byGroup[group]; ok && known.ID < id {
			continue
		}
		stats := tables.ships[id]
		byGroup[group] = shipEntry{
			ID: id, Group: group, Name: ship.Name, Rarity: ship.Rarity, Icon: ship.Icon,
			Type: stats.Type, TypeName: labels.shipTypes[stats.Type], Nation: labels.nations[stats.Nationality],
		}
	}
	list := make([]shipEntry, 0, len(byGroup))
	for _, entry := range byGroup {
		list = append(list, entry)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

// handleCatalogSkins lists every ship skin with its ship, for the skin
// picker.
func (server *Server) handleCatalogSkins(r *http.Request) (any, error) {
	tables, err := catalogTables()
	if err != nil {
		return nil, err
	}
	list := []skinEntry{}
	for id, skin := range server.names[dropSkin] {
		if skin.Name == "" {
			continue
		}
		if tables.skins[id].SkinType == -1 {
			continue
		}
		group := tables.skins[id].ShipGroup
		list = append(list, skinEntry{ID: id, Name: skin.Name, Group: group, ShipName: server.names.name(dropShip, group*10+1), Icon: skin.Icon})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

// handleCatalogAll lists every named thing of a drop type; gear as its +0
// pieces (an upgraded piece is granted as its +0 and enhanced).
func (server *Server) handleCatalogAll(r *http.Request) (any, error) {
	kind, err := strconv.ParseUint(r.URL.Query().Get("type"), 10, 32)
	if err != nil || server.names[uint32(kind)] == nil {
		return nil, badRequest("type: want a drop type the back-office names")
	}
	tables, err := catalogTables()
	if err != nil {
		return nil, err
	}
	list := []catalogEntry{}
	for id, row := range server.names[uint32(kind)] {
		// equip_data_statistics has placeholder rows named "0"
		if row.Name == "" || row.Name == "0" {
			continue
		}
		if kind == dropEquip && tables.equips[id].Level != 1 {
			continue
		}
		list = append(list, catalogEntry{ID: id, Name: row.Name, Rarity: row.Rarity, Icon: row.Icon})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}
