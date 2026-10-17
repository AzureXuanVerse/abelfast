package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// starterCommissions are the commissions (collection_template ids) the
// captured new commander was offered. The server draws them at random; this
// is that draw.
var starterCommissions = []uint32{30103, 30108, 30107, 30105, 20110}

// commissionTeams is how many commissions can run at once:
// user_level.max_collection at level 1 (it is 2 up to level 25).
const commissionTeams = 2

func grantCommissionsStarter(ctx context.Context, queries *db.Queries, commanderID uint32) error {
	for _, id := range starterCommissions {
		if err := queries.AddCommission(ctx, db.AddCommissionParams{CommanderID: commanderID, CollectionID: id}); err != nil {
			return err
		}
	}
	return nil
}

// commissionList is SC_13002, EventProxy's commissions.
func commissionList(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_13002, error) {
	rows, err := queries.ListCommissions(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_13002{MaxTeam: proto.Uint32(commissionTeams)}
	for _, row := range rows {
		msg.CollectionList = append(msg.CollectionList, &protobuf.COLLECTIONINFO{
			Id:         proto.Uint32(row.CollectionID),
			FinishTime: proto.Uint32(row.FinishTime),
			OverTime:   proto.Uint32(row.OverTime),
		})
	}
	return msg, nil
}
