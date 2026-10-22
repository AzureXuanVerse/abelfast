package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
)

type playerItem struct {
	amount
	// item_data_statistics max_num, 0 for no cap
	Max uint32 `json:"max,omitempty"`
}

// handlePlayerItems lists a commander's bag items.
func (server *Server) handlePlayerItems(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	items, err := server.Store.ListItems(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	list := make([]playerItem, 0, len(items))
	for _, row := range items {
		list = append(list, playerItem{amount: server.amount(dropItem, row.ItemID, row.Count), Max: tables.itemCaps[row.ItemID]})
	}
	return list, nil
}

// readCount is a request's {"count": n}.
func readCount(r *http.Request) (uint32, error) {
	var body struct {
		Count *uint32 `json:"count"`
	}
	if err := readJSON(r, &body); err != nil {
		return 0, err
	}
	if body.Count == nil {
		return 0, badRequest("count: missing")
	}
	return *body.Count, nil
}

// setStack sets how many of a stacked thing a commander has, 0 removing it.
func (server *Server) setStack(r *http.Request, known func(item, count uint32) error, set func(*db.Queries, context.Context, uint32, uint32, uint32) error, remove func(*db.Queries, context.Context, uint32, uint32) error) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	count, err := readCount(r)
	if err != nil {
		return nil, err
	}
	if err := known(item, count); err != nil {
		return nil, err
	}
	return kicked(server.Game.EditCommanderRows(r.Context(), commander.ID, func(queries *db.Queries) error {
		if count == 0 {
			return remove(queries, r.Context(), commander.ID, item)
		}
		return set(queries, r.Context(), commander.ID, item, count)
	}))
}

// handlePlayerSetItem sets a bag item's count (at most its max_num), 0
// taking it out of the bag.
func (server *Server) handlePlayerSetItem(r *http.Request) (any, error) {
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	return server.setStack(r, func(item, count uint32) error {
		if _, ok := server.names[dropItem][item]; !ok {
			return badRequest("no item " + itoa(item))
		}
		if limit := tables.itemCaps[item]; limit > 0 && count > limit {
			return badRequest("item " + itoa(item) + " holds at most " + itoa(limit))
		}
		return nil
	}, func(q *db.Queries, ctx context.Context, id, item, n uint32) error {
		return q.AdminSetItemCount(ctx, db.AdminSetItemCountParams{CommanderID: id, ItemID: item, Count: n})
	}, func(q *db.Queries, ctx context.Context, id, item uint32) error {
		return q.AdminDeleteItem(ctx, db.AdminDeleteItemParams{CommanderID: id, ItemID: item})
	})
}

type playerGearRow struct {
	amount
	// the enhancement, equip_data_template level - 1
	Enhance uint32 `json:"enhance"`
}

type playerSpWeapon struct {
	ID         uint32                `json:"id"`
	TemplateID uint32                `json:"template_id"`
	Name       string                `json:"name"`
	Rarity     uint32                `json:"rarity"`
	Icon       string                `json:"icon,omitempty"`
	Unique     bool                  `json:"unique"`
	Level      int                   `json:"level"`
	Levels     []playerSpWeaponLevel `json:"levels"`
	Attribute1 string                `json:"attribute_1"`
	Attribute2 string                `json:"attribute_2"`
	Value1     uint32                `json:"value_1"`
	Value2     uint32                `json:"value_2"`
	Attr1      uint32                `json:"attr_1"`
	Attr2      uint32                `json:"attr_2"`
	Pt         uint32                `json:"pt"`
	ShipID     uint32                `json:"ship_id,omitempty"`
	ShipName   string                `json:"ship_name,omitempty"`
}

// playerSpWeaponLevel is a level of an augment module's chain with what its
// row says: its enhancement (level - 1, back to +0 at a new tier) and name.
type playerSpWeaponLevel struct {
	game.SpWeaponLevel
	Plus   uint32 `json:"plus"`
	Name   string `json:"name"`
	Rarity uint32 `json:"rarity"`
}

type playerGear struct {
	Equipments  []playerGearRow  `json:"equipments"`
	EquipBagMax uint32           `json:"equip_bag_max"`
	SpWeapons   []playerSpWeapon `json:"spweapons"`
	EquipSkins  []amount         `json:"equip_skins"`
}

// handlePlayerGear lists a commander's depot gear, augment modules and gear
// skin box.
func (server *Server) handlePlayerGear(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	gear := playerGear{Equipments: []playerGearRow{}, EquipBagMax: commander.EquipBagMax, SpWeapons: []playerSpWeapon{}, EquipSkins: []amount{}}
	equips, err := server.Store.ListEquipments(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range equips {
		enhance := tables.equipLevels[row.EquipID]
		if enhance > 0 {
			enhance--
		}
		gear.Equipments = append(gear.Equipments, playerGearRow{amount: tables.equipAmount(server.names, row.EquipID, row.Count), Enhance: enhance})
	}
	spweapons, err := server.Store.AdminListSpWeapons(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range spweapons {
		config := tables.spweapons[row.TemplateID]
		var levels []playerSpWeaponLevel
		level := 0
		for i, l := range game.SpWeaponLevels(row.TemplateID) {
			if l.TemplateID == row.TemplateID {
				level = i
			}
			at := tables.spweapons[l.TemplateID]
			levels = append(levels, playerSpWeaponLevel{SpWeaponLevel: l, Plus: max(at.Level, 1) - 1, Name: at.Name, Rarity: at.Rarity})
		}
		shipName := row.ShipName
		if shipName == "" && row.ShipTemplateID != 0 {
			shipName = server.names[dropShip][uint32(row.ShipTemplateID)].Name
		}
		gear.SpWeapons = append(gear.SpWeapons, playerSpWeapon{
			ID: row.ID, TemplateID: row.TemplateID, Name: config.Name, Rarity: config.Rarity, Icon: config.Icon, Unique: config.Unique != 0,
			Level: level, Levels: levels, Attribute1: config.Attribute1, Attribute2: config.Attribute2, Value1: config.Value1, Value2: config.Value2,
			Attr1: row.Attr1, Attr2: row.Attr2, Pt: row.Pt, ShipID: uint32(row.ShipID), ShipName: shipName,
		})
	}
	skins, err := server.Store.ListEquipSkins(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range skins {
		gear.EquipSkins = append(gear.EquipSkins, tables.amount(dropEquipSkin, row.SkinID, row.Count))
	}
	return gear, nil
}

// handlePlayerSetGear sets how many of a gear the depot holds, 0 for none.
func (server *Server) handlePlayerSetGear(r *http.Request) (any, error) {
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	return server.setStack(r, func(item, _ uint32) error {
		if _, ok := tables.equipLevels[item]; !ok {
			return badRequest("no gear " + itoa(item))
		}
		return nil
	}, func(q *db.Queries, ctx context.Context, id, item, n uint32) error {
		return q.AdminSetEquipmentCount(ctx, db.AdminSetEquipmentCountParams{CommanderID: id, EquipID: item, Count: n})
	}, func(q *db.Queries, ctx context.Context, id, item uint32) error {
		return q.AdminDeleteEquipment(ctx, db.AdminDeleteEquipmentParams{CommanderID: id, EquipID: item})
	})
}

// handlePlayerSetEquipSkin sets how many of a gear skin the box holds.
func (server *Server) handlePlayerSetEquipSkin(r *http.Request) (any, error) {
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	return server.setStack(r, func(item, _ uint32) error {
		if _, ok := tables.names[dropEquipSkin][item]; !ok {
			return badRequest("no gear skin " + itoa(item))
		}
		return nil
	}, func(q *db.Queries, ctx context.Context, id, item, n uint32) error {
		return q.AdminSetEquipSkinCount(ctx, db.AdminSetEquipSkinCountParams{CommanderID: id, SkinID: item, Count: n})
	}, func(q *db.Queries, ctx context.Context, id, item uint32) error {
		return q.AdminDeleteEquipSkin(ctx, db.AdminDeleteEquipSkinParams{CommanderID: id, SkinID: item})
	})
}

// handlePlayerSetSpWeapon sets an augment module's level and rolls.
func (server *Server) handlePlayerSetSpWeapon(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	var body struct {
		TemplateID uint32 `json:"template_id"`
		Attr1      uint32 `json:"attr_1"`
		Attr2      uint32 `json:"attr_2"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if _, err := server.Store.GetSpWeapon(r.Context(), db.GetSpWeaponParams{CommanderID: commander.ID, ID: item}); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("no augment module " + itoa(item))
	} else if err != nil {
		return nil, err
	}
	return kicked(server.Game.SetSpWeapon(r.Context(), commander.ID, item, body.TemplateID, body.Attr1, body.Attr2))
}

// handlePlayerDeleteSpWeapon deletes an augment module, off its ship.
func (server *Server) handlePlayerDeleteSpWeapon(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	if _, err := server.Store.GetSpWeapon(ctx, db.GetSpWeaponParams{CommanderID: commander.ID, ID: item}); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("no augment module " + itoa(item))
	} else if err != nil {
		return nil, err
	}
	return kicked(server.Game.EditCommanderRows(ctx, commander.ID, func(queries *db.Queries) error {
		return queries.DeleteSpWeapon(ctx, db.DeleteSpWeaponParams{CommanderID: commander.ID, ID: item})
	}))
}

// handlePlayerGrantCosmetics grants what /grant can't: frames and augment
// modules (and anything it can).
func (server *Server) handlePlayerGrantCosmetics(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Drops []drop `json:"drops"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if len(body.Drops) == 0 {
		return nil, badRequest("nothing to grant")
	}
	drops, err := dropInfos(body.Drops)
	if err != nil {
		return nil, err
	}
	return kicked(server.Game.GrantCosmetics(r.Context(), commander.ID, drops))
}

// handlePlayerCatalog lists every gear skin, frame and augment module
// (/catalog/all lists the other kinds), and every gear at +0 with its
// enhanced ids, for the pickers to search; q narrows it by name or id.
func (server *Server) handlePlayerCatalog(r *http.Request) (any, error) {
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	query := r.URL.Query().Get("q")
	kind, err := strconv.ParseUint(r.URL.Query().Get("type"), 10, 32)
	if err == nil && kind == dropEquip {
		found := server.names.search(dropEquip, query, len(server.names[dropEquip]))
		list := make([]playerCatalogEntry, 0, len(found))
		for _, entry := range found {
			// a piece without an equip_data_template row can't be stored,
			// an enhanced one is granted from its +0's levels
			if level, ok := tables.equipLevels[entry.ID]; !ok || level > 1 {
				continue
			}
			list = append(list, playerCatalogEntry{catalogEntry: entry, Levels: tables.equipChain(entry.ID)})
		}
		return list, nil
	}
	if err != nil || tables.names[uint32(kind)] == nil {
		return nil, badRequest("type: want 3, 9, 14, 15 or 21")
	}
	return tables.search(uint32(kind), query), nil
}

// playerCatalogEntry is a gear to grant with its ids from +0 up.
type playerCatalogEntry struct {
	catalogEntry
	Levels []uint32 `json:"levels"`
}

type playerSkin struct {
	ID      uint32 `json:"id"`
	Name    string `json:"name"`
	Ship    string `json:"ship"`
	Icon    string `json:"icon,omitempty"`
	EndTime int64  `json:"end_time"`
}

type playerFrame struct {
	Type  uint32 `json:"type"`
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	InUse bool   `json:"in_use"`
}

type playerWardrobe struct {
	Skins     []playerSkin  `json:"skins"`
	Furniture []amount      `json:"furniture"`
	Frames    []playerFrame `json:"frames"`
	Points    []amount      `json:"points"`
}

// handlePlayerWardrobe lists a commander's ship skins, furniture, frames and
// virtual item points.
func (server *Server) handlePlayerWardrobe(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	wardrobe := playerWardrobe{Skins: []playerSkin{}, Furniture: []amount{}, Frames: []playerFrame{}, Points: []amount{}}
	skins, err := server.Store.AdminListSkins(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range skins {
		skin := server.names[dropSkin][row.SkinID]
		ship := server.names[dropShip][tables.skinGroups[row.SkinID]*10+1].Name
		wardrobe.Skins = append(wardrobe.Skins, playerSkin{ID: row.SkinID, Name: skin.Name, Ship: ship, Icon: skin.Icon, EndTime: row.EndTime})
	}
	furniture, err := server.Store.ListFurniture(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range furniture {
		wardrobe.Furniture = append(wardrobe.Furniture, server.amount(dropFurniture, row.FurnitureID, row.Count))
	}
	appearance, err := server.Store.GetAppearance(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	frames, err := server.Store.ListAttireFrames(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range frames {
		if row.DropType != dropIconFrame && row.DropType != dropChatFrame {
			continue
		}
		inUse := (row.DropType == dropIconFrame && appearance.IconFrame == row.FrameID) || (row.DropType == dropChatFrame && appearance.ChatFrame == row.FrameID)
		frame := tables.names[row.DropType][row.FrameID]
		wardrobe.Frames = append(wardrobe.Frames, playerFrame{Type: row.DropType, ID: row.FrameID, Name: frame.Name, Icon: frame.Icon, InUse: inUse})
	}
	points, err := server.Store.AdminListVirtualItems(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range points {
		wardrobe.Points = append(wardrobe.Points, server.amount(dropVItem, row.ItemID, row.Amount))
	}
	return wardrobe, nil
}

// handlePlayerSetSkin sets when a skin the commander has runs out: 0 for
// good, else a time to come.
func (server *Server) handlePlayerSetSkin(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	var body struct {
		EndTime int64 `json:"end_time"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if body.EndTime != 0 && body.EndTime <= time.Now().Unix() {
		return nil, badRequest("end_time: 0 or a time to come")
	}
	ctx := r.Context()
	if err := server.ownsSkin(ctx, commander.ID, item); err != nil {
		return nil, err
	}
	return kicked(server.Game.EditCommanderRows(ctx, commander.ID, func(queries *db.Queries) error {
		return queries.SetSkin(ctx, db.SetSkinParams{CommanderID: commander.ID, SkinID: item, EndTime: body.EndTime})
	}))
}

func (server *Server) ownsSkin(ctx context.Context, id, skin uint32) error {
	skins, err := server.Store.AdminListSkins(ctx, id)
	if err != nil {
		return err
	}
	for _, row := range skins {
		if row.SkinID == skin {
			return nil
		}
	}
	return notFound("no skin " + itoa(skin) + " owned")
}

// handlePlayerDeleteSkin takes a skin away, the ships wearing it back in
// their default one (as when a timed skin runs out).
func (server *Server) handlePlayerDeleteSkin(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	if err := server.ownsSkin(ctx, commander.ID, item); err != nil {
		return nil, err
	}
	return kicked(server.Game.EditCommanderRows(ctx, commander.ID, func(queries *db.Queries) error {
		if err := queries.AdminUnwearSkin(ctx, db.AdminUnwearSkinParams{CommanderID: commander.ID, SkinID: item}); err != nil {
			return err
		}
		return queries.AdminDeleteSkin(ctx, db.AdminDeleteSkinParams{CommanderID: commander.ID, SkinID: item})
	}))
}

// handlePlayerSetVirtualItem sets a virtual item counter the commander has.
func (server *Server) handlePlayerSetVirtualItem(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	var body struct {
		Amount *uint32 `json:"amount"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if body.Amount == nil {
		return nil, badRequest("amount: missing")
	}
	ctx := r.Context()
	points, err := server.Store.AdminListVirtualItems(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, row := range points {
		found = found || row.ItemID == item
	}
	if !found {
		return nil, notFound("no virtual item " + itoa(item) + " counted")
	}
	return kicked(server.Game.EditCommanderRows(ctx, commander.ID, func(queries *db.Queries) error {
		return queries.AdminSetVirtualItem(ctx, db.AdminSetVirtualItemParams{Amount: *body.Amount, CommanderID: commander.ID, ItemID: item})
	}))
}
