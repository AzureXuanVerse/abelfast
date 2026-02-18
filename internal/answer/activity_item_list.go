package answer

import (
	"encoding/json"
	"sort"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	activityItemResultSuccess = uint32(0)
	activityItemResultFailure = uint32(1)
)

func ActivityItemList(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26106{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26107, err
	}

	response := &protobuf.SC_26107{Ret: proto.Uint32(activityItemResultFailure), ItemList: []*protobuf.PB_ACTIVITY_ITEM{}}
	template, err := loadActivityTemplate(req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26107, client, response)
	}

	itemIDs, err := collectActivityScopedItemIDs(template)
	if err != nil {
		return connection.SendProtoMessage(26107, client, response)
	}
	balances, err := orm.ListCommanderItemBalances(client.Commander.CommanderID, itemIDs)
	if err != nil {
		return connection.SendProtoMessage(26107, client, response)
	}

	response.Ret = proto.Uint32(activityItemResultSuccess)
	response.ItemList = buildActivityItemList(balances)
	return connection.SendProtoMessage(26107, client, response)
}

func collectActivityScopedItemIDs(template activityTemplate) ([]uint32, error) {
	raw := []any{}
	if len(template.ConfigData) > 0 {
		var value any
		if err := json.Unmarshal(template.ConfigData, &value); err != nil {
			return nil, err
		}
		raw = append(raw, value)
	}
	if len(template.ConfigClient) > 0 {
		var value any
		if err := json.Unmarshal(template.ConfigClient, &value); err == nil {
			raw = append(raw, value)
		}
	}
	ids := make([]uint32, 0)
	for _, value := range raw {
		ids = collectUint32Values(value, ids)
	}
	set := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	result := make([]uint32, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i] < result[j]
	})
	return result, nil
}

func buildActivityItemList(balances map[uint32]uint32) []*protobuf.PB_ACTIVITY_ITEM {
	itemIDs := make([]uint32, 0, len(balances))
	for itemID, count := range balances {
		if count == 0 {
			continue
		}
		itemIDs = append(itemIDs, itemID)
	}
	sort.Slice(itemIDs, func(i int, j int) bool {
		return itemIDs[i] < itemIDs[j]
	})
	items := make([]*protobuf.PB_ACTIVITY_ITEM, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		items = append(items, &protobuf.PB_ACTIVITY_ITEM{Id: proto.Uint32(itemID), Num: proto.Uint32(balances[itemID])})
	}
	return items
}
