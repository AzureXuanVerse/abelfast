package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// DROP_TYPE_* (const.lua): what a reward's id refers to.
const (
	dropTypeResource = 1 // player_resource
	dropTypeItem     = 2 // item_data_statistics
	dropTypeShip     = 4 // ship_data_template
	dropTypeVItem    = 8 // item_virtual_data_statistics
)

// grantDrops stores rewards: resources, bag items, ships (with their default
// equipment, skills and archive entry, pushed in SC_12042), and virtual items
// (the Specialized Core is stored as its link item, the cruise pass points
// as their task counter, the rest as virtual item counters), recording the
// task progress they cause. Gear, furniture, skins... have no storage yet and
// are only logged.
func (tx *Tx) grantDrops(drops []*protobuf.DROPINFO) error {
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
		case kind == dropTypeShip:
			for range count {
				ship, err := tx.AddShip(id)
				if err != nil {
					return err
				}
				tx.newShips = append(tx.newShips, ship)
			}
		case kind == dropTypeVItem && id == vitemSpecializedCore:
			if err := tx.Queries.AddItem(tx.ctx, db.AddItemParams{CommanderID: tx.commanderID, ItemID: itemSpecializedCore, Count: count}); err != nil {
				return err
			}
		case kind == dropTypeVItem && id == VirtualItemCruisePt:
			total, err := tx.cruisePt()
			if err != nil {
				return err
			}
			tx.event(TaskEvent{SubType: TaskSubCruisePt, Count: total + count})
		case kind == dropTypeVItem:
			if err := tx.AddVirtualItem(tx.ctx, db.AddVirtualItemParams{CommanderID: tx.commanderID, ItemID: id, Amount: count}); err != nil {
				return err
			}
			if id == VirtualItemWeeklyPt {
				tx.event(TaskEvent{SubType: TaskSubWeeklyPt, Count: count})
			}
		default:
			logger.WithFields("Drops", logger.FieldValue("type", kind), logger.FieldValue("id", id)).Warn("drop not stored")
		}
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
