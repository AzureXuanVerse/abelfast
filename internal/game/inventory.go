package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// player_resource ids
const (
	ResourceGold = 1
	ResourceOil  = 2
)

// item_data_statistics ids
const ItemWisdomCube = 20001

// GrantStarter gives a new commander exactly what the new player guide
// (SG002) spends before any reward: one light build (ship_data_create_material
// 2: 600 gold and 1 wisdom cube).
func GrantStarter(ctx context.Context, queries *db.Queries, commanderID uint32) error {
	if err := queries.AddResource(ctx, db.AddResourceParams{CommanderID: commanderID, ResourceID: ResourceGold, Amount: 600}); err != nil {
		return err
	}
	return queries.AddItem(ctx, db.AddItemParams{CommanderID: commanderID, ItemID: ItemWisdomCube, Count: 1})
}

func resourceList(rows []db.ListResourcesRow) []*protobuf.RESOURCE {
	list := make([]*protobuf.RESOURCE, len(rows))
	for i, row := range rows {
		list[i] = &protobuf.RESOURCE{Type: proto.Uint32(row.ResourceID), Num: proto.Uint32(row.Amount)}
	}
	return list
}

// bag is SC_15001, BagProxy's items.
func bag(rows []db.ListItemsRow) *protobuf.SC_15001 {
	msg := &protobuf.SC_15001{ItemList: make([]*protobuf.ITEMINFO, len(rows))}
	for i, row := range rows {
		msg.ItemList[i] = &protobuf.ITEMINFO{Id: proto.Uint32(row.ItemID), Count: proto.Uint32(row.Count)}
	}
	return msg
}

// handleTracking takes the client's analytics events (CS_10991), no reply.
func (game *Game) handleTracking(client *connection.Client, req *protobuf.CS_10991) error {
	return nil
}
