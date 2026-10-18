package game

import (
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// DROP_TYPE_* (const.lua): what a reward's id refers to.
const (
	dropTypeResource  = 1 // player_resource
	dropTypeItem      = 2 // item_data_statistics
	dropTypeEquip     = 3 // equip_data_statistics
	dropTypeShip      = 4 // ship_data_template
	dropTypeFurniture = 5 // furniture_data_template
	dropTypeSkin      = 7 // ship_skin_template, for good
	dropTypeVItem     = 8 // item_virtual_data_statistics
	// ship_skin_template, number the seconds it lasts
	dropTypeSkinTimeLimit = 23
	dropTypeMeowfficer    = 25 // DROP_TYPE_COMMANDER_CAT, commander_data_template
)

// grantDrops stores rewards: resources, bag items, equipment, ships (with
// their default equipment, skills and archive entry, pushed in SC_12042), and
// backyard furniture, virtual items (the Specialized Core is stored as its
// link item, the cruise pass points as their task counter, the rest as
// virtual item counters) and ship skins (for good or for a time, as the
// client adds them: Drop's DROP_TYPE_SKIN and DROP_TYPE_SKIN_TIMELIMIT
// cases, model/vo/drop.lua) and meowfficers (grantMeowfficers, pushed in
// SC_25039), recording the task progress they cause. Gear skins, frames...
// have no storage yet and are only logged.
func (tx *Tx) grantDrops(drops []*protobuf.DROPINFO) error {
	var cats []*protobuf.COMMANDERINFO
	for _, entry := range drops {
		kind, id, count := entry.GetType(), entry.GetId(), entry.GetNumber()
		switch {
		case kind == dropTypeResource:
			if err := tx.AddResource(id, count); err != nil {
				return err
			}
		case kind == dropTypeItem:
			if err := tx.AddItem(id, count); err != nil {
				return err
			}
		case kind == dropTypeEquip:
			if err := tx.AddEquipment(id, count); err != nil {
				return err
			}
		case kind == dropTypeShip:
			for range count {
				ship, err := tx.AddShip(id)
				if err != nil {
					return err
				}
				tx.newShips = append(tx.newShips, ship)
			}
		case kind == dropTypeFurniture:
			if err := tx.AddFurniture(tx.ctx, db.AddFurnitureParams{CommanderID: tx.commanderID, FurnitureID: id, Count: count}); err != nil {
				return err
			}
		case kind == dropTypeSkin:
			if err := tx.AddSkin(id); err != nil {
				return err
			}
		case kind == dropTypeSkinTimeLimit:
			if err := tx.AddTimedSkin(id, int64(count), time.Now()); err != nil {
				return err
			}
		case kind == dropTypeVItem && id == vitemSpecializedCore:
			// stored as its link item, which tasks count as obtained
			if err := tx.AddItem(itemSpecializedCore, count); err != nil {
				return err
			}
		case kind == dropTypeVItem && id == VirtualItemCruisePt:
			total, err := tx.cruisePt()
			if err != nil {
				return err
			}
			tx.event(TaskEvent{SubType: TaskSubCruisePt, Count: total + count})
		case kind == dropTypeVItem:
			if err := tx.AddVirtualItem(id, count); err != nil {
				return err
			}
		case kind == dropTypeMeowfficer:
			added, err := tx.grantMeowfficers(entry)
			if err != nil {
				return err
			}
			cats = append(cats, added...)
		default:
			logger.WithFields("Drops", logger.FieldValue("type", kind), logger.FieldValue("id", id)).Warn("drop not stored")
		}
	}
	if len(cats) > 0 {
		// CommanderProxy adds them ahead of the reply, whose award window
		// shows the newest as many as the drops it lists
		// (ContextMediator ON_ACHIEVE, GetNewestCommander)
		tx.push(&protobuf.SC_25039{CommanderList: cats})
	}
	return nil
}

// weightedPick draws the index of one of items by weight, roll(n) being a
// random number below n; -1 when no item weighs anything.
func weightedPick[T any](roll func(n uint32) uint32, items []T, weight func(T) uint32) int {
	total := uint32(0)
	for _, item := range items {
		total += weight(item)
	}
	if total == 0 {
		return -1
	}
	n := roll(total)
	for i, item := range items {
		if n < weight(item) {
			return i
		}
		n -= weight(item)
	}
	return -1
}

// dropInfo is one DROPINFO of a reward: a DROP_TYPE_*, an id and a count.
func dropInfo(kind, id, number uint32) *protobuf.DROPINFO {
	return &protobuf.DROPINFO{Type: proto.Uint32(kind), Id: proto.Uint32(id), Number: proto.Uint32(number)}
}

// configDrops turns a config table's [[type, id, count], ...] into DROPINFOs.
func configDrops(drops [][]uint32) []*protobuf.DROPINFO {
	var list []*protobuf.DROPINFO
	for _, entry := range drops {
		if len(entry) >= 3 {
			list = append(list, dropInfo(entry[0], entry[1], entry[2]))
		}
	}
	return list
}
