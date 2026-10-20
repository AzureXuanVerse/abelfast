package game

import (
	"encoding/json"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The Private Quarters' tables (dorm3d_*, model/vo/apartment.lua,
// apartmentroom.lua, model/proxy/apartmentproxy.lua).

// dorm3d_rooms type: a shipgirl's own room or a common area she can be
// invited to (ApartmentRoom.isPersonalRoom)
const dorm3dRoomPersonal = 2

// dorm3dRoom is a dorm3d_rooms row: a personal room's character[1] is the
// shipgirl living in it, a common area's character_pay the ones it invites
// with invite_cost ([[ship group, shop_template id], ...]); unlock_item is
// what opening it takes.
type dorm3dRoom struct {
	Type         uint32          `json:"type"`
	Character    json.RawMessage `json:"character"`
	CharacterPay json.RawMessage `json:"character_pay"`
	UnlockItem   json.RawMessage `json:"unlock_item"`
	InviteCost   json.RawMessage `json:"invite_cost"`

	characters, invitable []uint32
	unlockItems           [][3]uint32
	inviteGoods           map[uint32]uint32 // ship group -> shop_template id
}

// personalGroup is the shipgirl living in a personal room, 0 for a common
// area (ApartmentRoom.getPersonalGroupId).
func (room dorm3dRoom) personalGroup() uint32 {
	if room.Type != dorm3dRoomPersonal || len(room.characters) == 0 {
		return 0
	}
	return room.characters[0]
}

// dorm3dDorm is a dorm3d_dorm_template row, by ship group: the skin she
// wears by default (Apartment skinList's first).
type dorm3dDorm struct {
	SkinModel uint32 `json:"skin_model"`
	BindRoom  uint32 `json:"bind_room"`
}

// dorm3dFavor is a dorm3d_favor row: the Intimacy points level takes from
// the one before it and what reaching it gives.
type dorm3dFavor struct {
	CharID      uint32          `json:"char_id"`
	Level       uint32          `json:"level"`
	FavorExp    uint32          `json:"favor_exp"`
	LevelupItem json.RawMessage `json:"levelup_item"`

	drops [][]uint32
}

// dorm3dFavorTrigger is a dorm3d_favor_trigger row: num Intimacy points for
// is_daily_max energy, once ever when is_repeat is 0.
type dorm3dFavorTrigger struct {
	Type       uint32 `json:"type"`
	Num        uint32 `json:"num"`
	IsDailyMax uint32 `json:"is_daily_max"`
	IsRepeat   uint32 `json:"is_repeat"`
}

// dorm3d_favor_trigger type: the one-time triggers for owning the shipgirl
// and for having married her (Apartment.TRIGGER_OWNER / TRIGGER_PROPOSE)
const (
	favorTriggerOwner   = 5
	favorTriggerPropose = 6
)

// dorm3dGift is a dorm3d_gift row: a gift any shipgirl takes (group 0) or
// one only she does, and the favor trigger giving one is.
type dorm3dGift struct {
	ShipGroupID    uint32 `json:"ship_group_id"`
	FavorTriggerID uint32 `json:"favor_trigger_id"`
}

// dorm3dDialogue is a dorm3d_dialogue_group row: whose dialogue it is and
// what plays it (ApartmentProxy.CheckUnlockConfig).
type dorm3dDialogue struct {
	CharID uint32          `json:"char_id"`
	Unlock json.RawMessage `json:"unlock"`

	unlock []uint32
}

// dorm3dCollection is a dorm3d_collection_template row: the room an item is
// found in, the favor trigger finding it is (0 none) and when it can be.
type dorm3dCollection struct {
	RoomID uint32          `json:"room_id"`
	Award  uint32          `json:"award"`
	Unlock json.RawMessage `json:"unlock"`

	unlock []uint32
}

// dorm3dFurniture is a dorm3d_furniture_template row: the room it is for
// and the slots it fits.
type dorm3dFurniture struct {
	RoomID      uint32          `json:"room_id"`
	TargetSlots json.RawMessage `json:"target_slots"`

	slots []uint32
}

// dorm3dFurnitureSlot is a dorm3d_furniture_slot_template row.
type dorm3dFurnitureSlot struct {
	RoomID uint32 `json:"room_id"`
}

// dorm3dSkin is a dorm3d_resource row: a shipgirl's outfit and what wearing
// it needs.
type dorm3dSkin struct {
	ShipGroup uint32          `json:"ship_group"`
	Unlock    json.RawMessage `json:"unlock"`

	unlock []uint32
}

var (
	dorm3dRooms          map[uint32]dorm3dRoom
	dorm3dDorms          map[uint32]dorm3dDorm
	dorm3dFavorTriggers  map[uint32]dorm3dFavorTrigger
	dorm3dGifts          map[uint32]dorm3dGift
	dorm3dDialogues      map[uint32]dorm3dDialogue
	dorm3dCollections    map[uint32]dorm3dCollection
	dorm3dFurnitures     map[uint32]dorm3dFurniture
	dorm3dFurnitureSlots map[uint32]dorm3dFurnitureSlot
	dorm3dSkins          map[uint32]dorm3dSkin

	// dorm3dFavorLevels are dorm3d_favor by ship group then level
	dorm3dFavorLevels map[uint32]map[uint32]dorm3dFavor

	// dorm3d_set: the energy a day (daily_vigor_max), the last Intimacy
	// level (favor_level) and the hours between two changes of what a
	// shipgirl calls the commander (drom3d_appellation_time; the client
	// counts 172800 s)
	dorm3dVigorMax, dorm3dFavorMaxLevel, dorm3dCallNameHours uint32
)

func loadApartmentData() error {
	var favors map[uint32]dorm3dFavor
	var settings map[string]gamedata.Setting
	err := gamedata.LoadAll(
		gamedata.Load("dorm3d_rooms", &dorm3dRooms),
		gamedata.Load("dorm3d_dorm_template", &dorm3dDorms),
		gamedata.Load("dorm3d_favor", &favors),
		gamedata.Load("dorm3d_favor_trigger", &dorm3dFavorTriggers),
		gamedata.Load("dorm3d_gift", &dorm3dGifts),
		gamedata.Load("dorm3d_dialogue_group", &dorm3dDialogues),
		gamedata.Load("dorm3d_collection_template", &dorm3dCollections),
		gamedata.Load("dorm3d_furniture_template", &dorm3dFurnitures),
		gamedata.Load("dorm3d_furniture_slot_template", &dorm3dFurnitureSlots),
		gamedata.Load("dorm3d_resource", &dorm3dSkins),
		func() (err error) {
			settings, err = gamedata.Settings("dorm3d_set")
			return err
		},
	)
	if err != nil {
		return err
	}
	for key, value := range map[string]*uint32{
		"daily_vigor_max":         &dorm3dVigorMax,
		"favor_level":             &dorm3dFavorMaxLevel,
		"drom3d_appellation_time": &dorm3dCallNameHours,
	} {
		setting, ok := settings[key]
		if !ok {
			return gamedata.Missing("dorm3d_set", "%s", key)
		}
		*value = setting.KeyValueInt
	}
	for id, room := range dorm3dRooms {
		room.characters = gamedata.IDs(room.Character)
		room.invitable = gamedata.IDs(room.CharacterPay)
		items := gamedata.IDs(room.UnlockItem)
		for i := 0; i+2 < len(items); i += 3 {
			room.unlockItems = append(room.unlockItems, [3]uint32{items[i], items[i+1], items[i+2]})
		}
		room.inviteGoods = map[uint32]uint32{}
		costs := gamedata.IDs(room.InviteCost)
		for i := 0; i+1 < len(costs); i += 2 {
			room.inviteGoods[costs[i]] = costs[i+1]
		}
		dorm3dRooms[id] = room
	}
	dorm3dFavorLevels = map[uint32]map[uint32]dorm3dFavor{}
	for _, favor := range favors {
		_ = json.Unmarshal(favor.LevelupItem, &favor.drops)
		if dorm3dFavorLevels[favor.CharID] == nil {
			dorm3dFavorLevels[favor.CharID] = map[uint32]dorm3dFavor{}
		}
		dorm3dFavorLevels[favor.CharID][favor.Level] = favor
	}
	for id, dialogue := range dorm3dDialogues {
		dialogue.unlock = gamedata.IDs(dialogue.Unlock)
		dorm3dDialogues[id] = dialogue
	}
	for id, item := range dorm3dCollections {
		item.unlock = gamedata.IDs(item.Unlock)
		dorm3dCollections[id] = item
	}
	for id, furniture := range dorm3dFurnitures {
		furniture.slots = gamedata.IDs(furniture.TargetSlots)
		dorm3dFurnitures[id] = furniture
	}
	for id, skin := range dorm3dSkins {
		skin.unlock = gamedata.IDs(skin.Unlock)
		dorm3dSkins[id] = skin
	}
	return nil
}

// favorToMax is the Intimacy points a shipgirl at level still needs to
// reach the last level (Apartment.getMaxFavor).
func favorToMax(group, level uint32) uint32 {
	total := uint32(0)
	for next := level + 1; next <= dorm3dFavorMaxLevel; next++ {
		total += dorm3dFavorLevels[group][next].FavorExp
	}
	return total
}
