package game

import (
	"encoding/json"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// DROP_TYPE_ISLAND_* (const.lua): rewards that go to the island, which the
// client adds itself from a reply's drop_list (IslandDropHelper.AddItems).
const (
	dropTypeIslandItem       = 41 // island_item_data_template, to the warehouse
	dropTypeIslandOverflow   = 42 // a warehouse item that didn't fit
	dropTypeIslandAbility    = 43 // island_ability_template, unlocked once
	dropTypeIslandInvitation = 44 // island_chara_template: a character to invite
	dropTypeIslandFurniture  = 45 // island_furniture_template, for the plaza
	dropTypeIslandDress      = 46 // island_dress_template: the commander's or a character's
	dropTypeIslandSkin       = 47 // island_skin_template, to its character
	dropTypeIslandCollection = 49 // island_collection, completed
	dropTypeIslandTicket     = 50 // island_speedup_ticket
	dropTypeIslandAction     = 51 // island_action
	dropTypeIslandCardDiy    = 52 // island_card_diy
	// a ticket's start of validity (island_drop_time_set), next to it
	dropTypeTimestamp = 99
	// VIRTUAL_DROP_TYPE_ISLAND_SEASON_PT: the development season's points
	dropTypeIslandSeasonPt = 9000
)

// islandSkinConfig is an island_skin_template row: an outfit of a
// character (its ship group).
type islandSkinConfig struct {
	ShipGroup uint32 `json:"ship_group"`
}

// islandTicketConfig is an island_speedup_ticket row: how long a speed-up
// ticket stays valid (IslandTicket.GetEndTimeById): until a fixed time
// (expiration_type 2, expiration_time, or "always") or for duration days
// from the day it was obtained (expiration_type 1).
type islandTicketConfig struct {
	ExpirationType uint32          `json:"expiration_type"`
	ExpirationTime json.RawMessage `json:"expiration_time"`
	Duration       uint32          `json:"duration"`
}

var (
	islandSkins   map[uint32]islandSkinConfig
	islandTickets map[uint32]islandTicketConfig
)

func loadIslandDropData() error {
	return gamedata.LoadAll(
		gamedata.Load("island_skin_template", &islandSkins),
		gamedata.Load("island_speedup_ticket", &islandTickets),
	)
}

// isIslandDrop tells whether a drop type goes to the island.
func isIslandDrop(kind uint32) bool {
	return kind >= dropTypeIslandItem && kind <= dropTypeIslandCardDiy || kind == dropTypeIslandSeasonPt || kind == dropTypeTimestamp
}

// ticketEnd is when a speed-up ticket obtained now stops being valid, 0 for
// never (IslandTicket.GetEndTimeById).
func (config islandTicketConfig) end(now time.Time) uint32 {
	switch config.ExpirationType {
	case 2:
		var parts [2][3]int
		if json.Unmarshal(config.ExpirationTime, &parts) != nil {
			return 0
		}
		return uint32(time.Date(parts[0][0], time.Month(parts[0][1]), parts[0][2], parts[1][0], parts[1][1], parts[1][2], 0, gamedata.Zone()).Unix())
	case 1:
		return uint32(startOfDay(now).Unix()) + 86400*config.Duration - 1
	}
	return 0
}

// addIslandItem puts count of an island item in the warehouse.
func (state *islandState) addIslandItem(id, count uint32) {
	private := state.private()
	for _, item := range private.ItemList {
		if item.GetId() == id {
			item.Num = proto.Uint32(item.GetNum() + count)
			return
		}
	}
	private.ItemList = append(private.ItemList, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(count)})
}

// islandItemCount is how many of an island item the warehouse holds.
func (state *islandState) islandItemCount(id uint32) uint32 {
	for _, item := range state.private().GetItemList() {
		if item.GetId() == id {
			return item.GetNum()
		}
	}
	return 0
}

// takeIslandItem removes count of an island item from the warehouse;
// ErrNotEnough when there are fewer.
func (state *islandState) takeIslandItem(id, count uint32) error {
	private := state.private()
	for i, item := range private.ItemList {
		if item.GetId() != id {
			continue
		}
		if item.GetNum() < count {
			return ErrNotEnough
		}
		item.Num = proto.Uint32(item.GetNum() - count)
		if item.GetNum() == 0 {
			private.ItemList = slices.Delete(private.ItemList, i, i+1)
		}
		return nil
	}
	if count == 0 {
		return nil
	}
	return ErrNotEnough
}

// addIslandDrops stores island rewards on the commander's island as the
// client adds them from the reply (IslandDropHelper.AddItems): warehouse
// items (and overflow), abilities, invitations, plaza furniture, outfits,
// character skins, collections, speed-up tickets, actions, card parts and
// season points. Called by grantDrops for the island drop types.
func (tx *Tx) addIslandDrops(drops []*protobuf.DROPINFO) error {
	state, err := tx.island()
	if err != nil {
		return err
	}
	now := serverNow()
	public, private := state.public(), state.private()
	obtained := map[uint32]uint32{}
	for _, drop := range drops {
		id, count := drop.GetId(), drop.GetNumber()
		switch drop.GetType() {
		case dropTypeIslandItem:
			obtained[id] += count
			// told to the client as overflow when it doesn't fit
			// (islandItemOverflows): the reply shares the drop
			if state.islandItemOverflows(id, count) {
				drop.Type = proto.Uint32(dropTypeIslandOverflow)
				state.addIslandOverflow(id, count)
			} else {
				state.addIslandItem(id, count)
			}
		case dropTypeIslandOverflow:
			state.addIslandOverflow(id, count)
		case dropTypeIslandAbility:
			if !slices.Contains(public.AbilityList, id) {
				public.AbilityList = append(public.AbilityList, id)
				islandFishingAbility(state, id)
				if err := tx.islandAbilityGained(state, id); err != nil {
					return err
				}
			}
		case dropTypeIslandInvitation:
			if !slices.Contains(public.ShipSys.InviteList, id) {
				public.ShipSys.InviteList = append(public.ShipSys.InviteList, id)
			}
		case dropTypeIslandFurniture:
			found := false
			for _, furniture := range private.FurnitureList {
				if furniture.GetId() == id {
					furniture.Count = proto.Uint32(furniture.GetCount() + count)
					furniture.Time = proto.Uint32(uint32(now.Unix()))
					found = true
				}
			}
			if !found {
				private.FurnitureList = append(private.FurnitureList, &protobuf.PB_FURNITURE{
					Id: proto.Uint32(id), Count: proto.Uint32(count), Time: proto.Uint32(uint32(now.Unix())),
				})
			}
		case dropTypeIslandDress:
			state.addIslandDress(id, count, now)
		case dropTypeIslandSkin:
			group := islandSkins[id].ShipGroup
			var entry *protobuf.PB_ISLAND_SHIP_SKIN
			for _, skins := range public.ShipSys.SkinList {
				if skins.GetShipId() == group {
					entry = skins
				}
			}
			if entry == nil {
				entry = &protobuf.PB_ISLAND_SHIP_SKIN{ShipId: proto.Uint32(group)}
				public.ShipSys.SkinList = append(public.ShipSys.SkinList, entry)
			}
			if !slices.ContainsFunc(entry.SkinList, func(skin *protobuf.PB_ISLAND_SKIN) bool { return skin.GetId() == id }) {
				entry.SkinList = append(entry.SkinList, &protobuf.PB_ISLAND_SKIN{Id: proto.Uint32(id), ColorId: proto.Uint32(0)})
			}
		case dropTypeIslandCollection:
			if !slices.Contains(private.CollectSys.FinishList, id) {
				private.CollectSys.FinishList = append(private.CollectSys.FinishList, id)
			}
		case dropTypeIslandTicket:
			end := islandTickets[id].end(now)
			found := false
			for _, ticket := range private.SpeedTickets {
				if ticket.GetKey().GetSpeedId() == id && ticket.GetKey().GetEndTime() == end {
					ticket.Num = proto.Uint32(ticket.GetNum() + count)
					found = true
				}
			}
			if !found {
				private.SpeedTickets = append(private.SpeedTickets, &protobuf.PB_SPEEDUP_TICKET{
					Key: &protobuf.PB_SPEEDUP_KEY{SpeedId: proto.Uint32(id), EndTime: proto.Uint32(end)}, Num: proto.Uint32(count),
				})
			}
		case dropTypeIslandAction:
			if !slices.Contains(private.ActionList, id) {
				private.ActionList = append(private.ActionList, id)
			}
		case dropTypeIslandCardDiy:
			found := false
			for _, image := range private.ImageList {
				if image.GetId() == id {
					image.Num = proto.Uint32(image.GetNum() + count)
					found = true
				}
			}
			if !found {
				private.ImageList = append(private.ImageList, &protobuf.PB_CARD_IMAGE{Id: proto.Uint32(id), Num: proto.Uint32(count)})
			}
		case dropTypeIslandSeasonPt:
			private.Season.Pt = proto.Uint32(private.Season.GetPt() + count)
		}
	}
	tx.islandBookItemsObtained(state, obtained)
	for _, id := range slices.Sorted(maps.Keys(obtained)) {
		// IslandTaskTargetType.OBTAIN
		if err := tx.islandTaskEvent(islandTargetObtain, obtained[id], id); err != nil {
			return err
		}
	}
	return nil
}

// addIslandDress gives an outfit part: the commander's (belongto 1) once,
// unseen; a character's counted (PB_ISLAND_DRESS_NUM).
func (state *islandState) addIslandDress(id, count uint32, now time.Time) {
	if islandDresses[id].BelongTo == 1 {
		dress := state.private().UserDress
		if !slices.ContainsFunc(dress.HadDress, func(owned *protobuf.PB_ISLAND_DRESS_USER) bool { return owned.GetId() == id }) {
			dress.HadDress = append(dress.HadDress, &protobuf.PB_ISLAND_DRESS_USER{Id: proto.Uint32(id), State: proto.Uint32(0), Color: proto.Uint32(0)})
		}
		return
	}
	ships := state.public().ShipSys
	for _, owned := range ships.HadDress {
		if owned.GetId() == id {
			owned.Num = proto.Uint32(owned.GetNum() + count)
			return
		}
	}
	ships.HadDress = append(ships.HadDress, &protobuf.PB_ISLAND_DRESS_NUM{
		Id: proto.Uint32(id), Num: proto.Uint32(count), Read: proto.Uint32(0), Time: proto.Uint32(uint32(now.Unix())),
	})
}

// islandDropTimeConfig is an island_drop_time_set row: the id of the
// DROP_TYPE_TIMESTAMP drop that goes with a timed island reward (drop_type,
// link_id).
type islandDropTimeConfig struct {
	DropType uint32 `json:"drop_type"`
	LinkID   uint32 `json:"link_id"`
}

// islandDropTimes are the island_drop_time_set ids by drop type and id.
var islandDropTimes map[[2]uint32]uint32

func loadIslandDropTimes() error {
	var rows map[uint32]islandDropTimeConfig
	if err := gamedata.Load("island_drop_time_set", &rows)(); err != nil {
		return err
	}
	islandDropTimes = make(map[[2]uint32]uint32, len(rows))
	for id, row := range rows {
		islandDropTimes[[2]uint32{row.DropType, row.LinkID}] = id
	}
	return nil
}

// islandStamped adds to a reply's drop_list the DROP_TYPE_TIMESTAMP drop of
// every speed-up ticket in it: IslandDropHelper.AddItems reads a ticket's
// start of validity from it (GetIslandTimestamps) and fails without one.
// The time is the start of the server day, from which IslandTicket counts a
// ticket's days.
func islandStamped[M proto.Message](reply M) M {
	message := reply.ProtoReflect()
	field := message.Descriptor().Fields().ByName("drop_list")
	if field == nil || !field.IsList() || field.Message() == nil || field.Message().Name() != "DROPINFO" {
		return reply
	}
	list := message.Get(field).List()
	seen := map[uint32]bool{}
	var stamps []*protobuf.DROPINFO
	for i := range list.Len() {
		drop := list.Get(i).Message().Interface().(*protobuf.DROPINFO)
		if drop.GetType() == dropTypeTimestamp {
			seen[drop.GetId()] = true
		}
		if drop.GetType() != dropTypeIslandTicket {
			continue
		}
		if id, ok := islandDropTimes[[2]uint32{dropTypeIslandTicket, drop.GetId()}]; ok && !seen[id] {
			seen[id] = true
			stamps = append(stamps, dropInfo(dropTypeTimestamp, id, uint32(startOfDay(serverNow()).Unix())))
		}
	}
	for _, stamp := range stamps {
		list.Append(protoreflect.ValueOfMessage(stamp.ProtoReflect()))
	}
	return reply
}
