package game

import (
	"context"

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

// granted is what grantDrops did besides storing the drops.
type granted struct {
	Ships  []*protobuf.SHIPINFO // new ships, for an SC_12042
	Pushes []proto.Message      // SC_17002 for new ship groups, sent before the reply
	Events []TaskEvent          // task progress the drops cause
}

// grantDrops stores rewards: resources, bag items, ships (with their default
// equipment, skills and archive entry), and virtual items (the Specialized
// Core is stored as its link item, the rest as virtual item counters).
// Gear, furniture, skins... have no storage yet and are only logged.
func grantDrops(ctx context.Context, queries *db.Queries, commanderID uint32, drops []*protobuf.DROPINFO) (granted, error) {
	var result granted
	for _, entry := range drops {
		kind, id, count := entry.GetType(), entry.GetId(), entry.GetNumber()
		switch {
		case kind == dropTypeResource:
			if err := queries.AddResource(ctx, db.AddResourceParams{CommanderID: commanderID, ResourceID: id, Amount: count}); err != nil {
				return result, err
			}
		case kind == dropTypeItem:
			if err := queries.AddItem(ctx, db.AddItemParams{CommanderID: commanderID, ItemID: id, Count: count}); err != nil {
				return result, err
			}
			result.Events = append(result.Events, TaskEvent{SubType: TaskSubObtainItem, Target: id, Count: count})
		case kind == dropTypeShip:
			for range count {
				ship, pushes, err := AddShip(ctx, queries, commanderID, id)
				if err != nil {
					return result, err
				}
				result.Ships = append(result.Ships, ship)
				result.Pushes = append(result.Pushes, pushes...)
			}
		case kind == dropTypeVItem && id == vitemSpecializedCore:
			if err := queries.AddItem(ctx, db.AddItemParams{CommanderID: commanderID, ItemID: itemSpecializedCore, Count: count}); err != nil {
				return result, err
			}
		case kind == dropTypeVItem:
			total, err := queries.AddVirtualItem(ctx, db.AddVirtualItemParams{CommanderID: commanderID, ItemID: id, Amount: count})
			if err != nil {
				return result, err
			}
			switch id {
			case VirtualItemCruisePt:
				result.Events = append(result.Events, TaskEvent{SubType: TaskSubCruisePt, Count: total})
			case VirtualItemWeeklyPt:
				result.Events = append(result.Events, TaskEvent{SubType: TaskSubWeeklyPt, Count: count})
			}
		default:
			logger.WithFields("Drops", logger.FieldValue("type", kind), logger.FieldValue("id", id)).Warn("drop not stored")
		}
	}
	return result, nil
}

// before is what the client needs ahead of the reply that lists the drops:
// archive updates, then the new ships.
func (result granted) before() []proto.Message {
	pushes := append([]proto.Message{}, result.Pushes...)
	if len(result.Ships) > 0 {
		pushes = append(pushes, NewShipsPush(result.Ships...))
	}
	return pushes
}

// configDrops turns a config table's [[type, id, count], ...] into DROPINFOs.
func configDrops(drops [][]uint32) []*protobuf.DROPINFO {
	var list []*protobuf.DROPINFO
	for _, entry := range drops {
		if len(entry) >= 3 {
			list = append(list, &protobuf.DROPINFO{Type: proto.Uint32(entry[0]), Id: proto.Uint32(entry[1]), Number: proto.Uint32(entry[2])})
		}
	}
	return list
}
