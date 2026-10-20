package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ActivityConst.ACTIVITY_TYPE_VIRTUAL_BAG: VirtualBagActivity keeps the
// event's own items in data1KeyValueList[virtualBagList] (getVitemNumber
// indexes it unchecked, so the list is always sent).
const (
	activityTypeVirtualBag = 41
	virtualBagList         = 1
)

// DROP_TYPE_USE_ACTIVITY_DROP (const.lua): drop types above it are an
// activity's items, the activity being activity_drop_type[type].activity_id
// (Drop.AddItemDefault, model/vo/drop.lua). DROP_TYPE_RYZA_DROP keeps them
// (dropTypeRyza) as AtelierMaterials instead.
const dropTypeUseActivityDrop = 1000

// activity_drop_type row: the activity whose items a drop type is
type activityDropType struct {
	ActivityID uint32 `json:"activity_id"`
}

var activityDropTypes map[uint32]activityDropType

// activityItemDrop is the activity whose virtual bag holds a drop type's
// items, false when the type isn't kept in one.
func activityItemDrop(kind uint32) (uint32, bool) {
	if kind <= dropTypeUseActivityDrop || kind == dropTypeRyza {
		return 0, false
	}
	row, ok := activityDropTypes[kind]
	return row.ActivityID, ok && row.ActivityID != 0
}

// addActivityItem adds count of an activity's virtual bag item, as the
// client does when the drop comes in (addVitemNumber).
func (tx *Tx) addActivityItem(activityID, itemID, count uint32) error {
	return tx.AddActivityKeyValue(tx.ctx, db.AddActivityKeyValueParams{
		CommanderID: tx.commanderID, ActivityID: activityID, ListKey: virtualBagList, ItemKey: itemID, Value: count,
	})
}

// spendActivityItem takes count of an activity's virtual bag item,
// ErrNotEnough if the commander has less.
func (tx *Tx) spendActivityItem(activityID, itemID, count uint32) error {
	n, err := tx.SpendActivityKeyValue(tx.ctx, db.SpendActivityKeyValueParams{
		Amount: count, CommanderID: tx.commanderID, ActivityID: activityID, ListKey: virtualBagList, ItemKey: itemID,
	})
	if err == nil && n == 0 && count > 0 {
		err = ErrNotEnough
	}
	return err
}

// activityKeyValues are the commander's stored date1_key_value_list
// entries: by activity, then by list key.
func activityKeyValues(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32]map[uint32][]*protobuf.KEYVALUE_P11, error) {
	rows, err := queries.ListActivityKeyValues(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	lists := map[uint32]map[uint32][]*protobuf.KEYVALUE_P11{}
	for _, row := range rows {
		if lists[row.ActivityID] == nil {
			lists[row.ActivityID] = map[uint32][]*protobuf.KEYVALUE_P11{}
		}
		lists[row.ActivityID][row.ListKey] = append(lists[row.ActivityID][row.ListKey],
			&protobuf.KEYVALUE_P11{Key: proto.Uint32(row.ItemKey), Value: proto.Uint32(row.Value)})
	}
	return lists, nil
}
