package admin

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// DROP_TYPE_* (const.lua) the commander screens show besides names' kinds.
const (
	dropEquipSkin = 9  // equip_skin_template
	dropIconFrame = 14 // item_data_frame
	dropChatFrame = 15 // item_data_chat
	dropSkinTimed = 23 // DROP_TYPE_SKIN_TIMELIMIT, a skin for count seconds
	dropSpWeapon  = 21 // spweapon_data_statistics
)

// spweaponRow is a spweapon_data_statistics level, its base row's columns
// filled in (the config tables' base lookup in include.lua).
type spweaponRow struct {
	Name       string `json:"name"`
	Rarity     uint32 `json:"rarity"`
	Icon       string `json:"icon"`
	Level      uint32 `json:"level"`
	Unique     uint32 `json:"unique"`
	Attribute1 string `json:"attribute_1"`
	Attribute2 string `json:"attribute_2"`
	Value1     uint32 `json:"value_1"`
	Value2     uint32 `json:"value_2"`
}

// playerTables are the config tables the commander screens read past names:
// the gear skins, frames and augment modules, gear enhancement levels,
// items' caps and skins' ships.
type playerTables struct {
	names     map[uint32]map[uint32]named
	spweapons map[uint32]spweaponRow
	// equip_data_template level (1 is +0), its next level and its +0 row
	equipLevels map[uint32]uint32
	equipNext   map[uint32]uint32
	equipBase   map[uint32]uint32
	// item_data_statistics max_num, 0 for none
	itemCaps map[uint32]uint32
	// ship_skin_template ship_group
	skinGroups map[uint32]uint32
}

var loadPlayerTables = sync.OnceValues(func() (*playerTables, error) {
	tables := &playerTables{names: map[uint32]map[uint32]named{}}
	for kind, table := range map[uint32]string{dropEquipSkin: "equip_skin_template", dropIconFrame: "item_data_frame", dropChatFrame: "item_data_chat"} {
		rows, err := gamedata.Table[tableRow](table)
		if err != nil {
			return nil, err
		}
		byID := make(map[uint32]named, len(rows))
		for id, row := range rows {
			switch {
			case kind == dropEquipSkin && row.Icon != "":
				row.Icon = "equips/" + strings.ToLower(row.Icon)
			case kind == dropIconFrame:
				// IconFrame.GetIcon: the IconFrame/<id> prefab's texture
				row.Icon = "iconframe/" + itoa(id)
			default:
				// a chat frame's prefab has no texture of its own
				row.Icon = ""
			}
			byID[id] = namedRow(row)
		}
		tables.names[kind] = byID
	}
	raw, err := gamedata.Table[json.RawMessage]("spweapon_data_statistics")
	if err != nil {
		return nil, err
	}
	tables.spweapons = make(map[uint32]spweaponRow, len(raw))
	tables.names[dropSpWeapon] = make(map[uint32]named, len(raw))
	for id, row := range raw {
		var base struct {
			Base uint32 `json:"base"`
		}
		var spweapon spweaponRow
		json.Unmarshal(row, &base)
		if base.Base != 0 {
			json.Unmarshal(raw[base.Base], &spweapon)
		}
		if json.Unmarshal(row, &spweapon) != nil {
			continue
		}
		if spweapon.Icon != "" {
			spweapon.Icon = "spweapon/" + strings.ToLower(spweapon.Icon)
		}
		tables.spweapons[id] = spweapon
		tables.names[dropSpWeapon][id] = named{Name: spweapon.Name, Rarity: spweapon.Rarity, Icon: spweapon.Icon}
	}
	levels, err := gamedata.Table[struct {
		Level uint32 `json:"level"`
		Next  uint32 `json:"next"`
		Base  uint32 `json:"base"`
	}]("equip_data_template")
	if err != nil {
		return nil, err
	}
	tables.equipLevels = make(map[uint32]uint32, len(levels))
	tables.equipNext = make(map[uint32]uint32, len(levels))
	tables.equipBase = make(map[uint32]uint32, len(levels))
	for id, row := range levels {
		tables.equipLevels[id], tables.equipNext[id], tables.equipBase[id] = row.Level, row.Next, row.Base
	}
	caps, err := gamedata.Table[struct {
		MaxNum uint32 `json:"max_num"`
	}]("item_data_statistics")
	if err != nil {
		return nil, err
	}
	tables.itemCaps = make(map[uint32]uint32, len(caps))
	for id, row := range caps {
		tables.itemCaps[id] = row.MaxNum
	}
	groups, err := gamedata.Table[struct {
		ShipGroup uint32 `json:"ship_group"`
	}]("ship_skin_template")
	if err != nil {
		return nil, err
	}
	tables.skinGroups = make(map[uint32]uint32, len(groups))
	for id, row := range groups {
		tables.skinGroups[id] = row.ShipGroup
	}
	return tables, nil
})

// playerAmount is amount for the kinds playerTables names.
func (tables *playerTables) amount(kind, id, count uint32) amount {
	row := tables.names[kind][id]
	return amount{ID: id, Name: row.Name, Rarity: row.Rarity, Icon: row.Icon, Count: count}
}

// search lists the things of kind whose name or id holds query, every one
// for an empty query (names.search for the kinds playerTables names).
func (tables *playerTables) search(kind uint32, query string) []catalogEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	found := []catalogEntry{}
	for id, row := range tables.names[kind] {
		// an augment module is granted at its first level
		if row.Name == "" || (kind == dropSpWeapon && tables.spweapons[id].Level > 1) {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(row.Name), query) || strings.Contains(itoa(id), query) {
			found = append(found, catalogEntry{ID: id, Name: row.Name, Rarity: row.Rarity, Icon: row.Icon})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })
	return found
}

// equipAmount is amount for a gear, an enhanced one named as its +0 row
// (its statistics row only lists what differs, its base's has the name).
func (tables *playerTables) equipAmount(names names, id, count uint32) amount {
	row, ok := names[dropEquip][id]
	if !ok || row.Name == "" {
		row = names[dropEquip][tables.equipBase[id]]
	}
	return amount{ID: id, Name: row.Name, Rarity: row.Rarity, Icon: row.Icon, Count: count}
}

// equipChain is a +0 gear's ids from +0 up, by equip_data_template next.
func (tables *playerTables) equipChain(id uint32) []uint32 {
	chain := []uint32{}
	for ; id != 0 && len(chain) < 30; id = tables.equipNext[id] {
		if _, ok := tables.equipLevels[id]; !ok {
			break
		}
		chain = append(chain, id)
	}
	return chain
}
