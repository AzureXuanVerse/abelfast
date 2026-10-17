package game

import (
	"context"
	"math/rand/v2"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// collection_template row
type commissionConfig struct {
	Type uint32 `json:"type"`
	Lv   uint32 `json:"lv"`   // commander level it is offered from
	Time uint32 `json:"time"` // how long it stays offered, 0 for no limit
}

// collection_template types that aren't drawn
const (
	// given by the ACTIVITY_TYPE_COLLECTION_EVENT activity's config_data
	// (model/vo/collectioneventactivity.lua)
	commissionTypeActivity = 0
	// EventConst.EVENT_TYPE_NIGHT (view/event/eventconst.lua), offered at
	// night only (EventProxy:checkNightEvent)
	commissionTypeNight = 5
)

var commissionConfigs map[uint32]commissionConfig

func loadCommissionData() error {
	return gamedata.Load("collection_template", &commissionConfigs)()
}

// grantCommissionsStarter offers a new commander new_commander.json's count
// of commissions, drawn at random from the collection_template rows open at
// their level that are neither activity nor night ones nor the urgent ones
// with a time limit (pushed after battles, SC_13011). The capture drew 30103,
// 30108, 30107, 30105, 20110 out of these 26 at level 1.
func grantCommissionsStarter(ctx context.Context, queries *db.Queries, commander db.Commander) error {
	var candidates []uint32
	for id, config := range commissionConfigs {
		if config.Lv <= commander.Level && config.Time == 0 && config.Type != commissionTypeActivity && config.Type != commissionTypeNight {
			candidates = append(candidates, id)
		}
	}
	rand.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	for _, id := range candidates[:min(newCommander.Commissions, len(candidates))] {
		if err := queries.AddCommission(ctx, db.AddCommissionParams{CommanderID: commander.ID, CollectionID: id}); err != nil {
			return err
		}
	}
	return nil
}

// commissionList is SC_13002, EventProxy's commissions; max_team, how many
// can run at once, is user_level max_collection at the commander's level.
func commissionList(ctx context.Context, queries *db.Queries, commanderID, level uint32) (*protobuf.SC_13002, error) {
	rows, err := queries.ListCommissions(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_13002{MaxTeam: proto.Uint32(commanderLevels[level].MaxCollection)}
	for _, row := range rows {
		msg.CollectionList = append(msg.CollectionList, &protobuf.COLLECTIONINFO{
			Id:         proto.Uint32(row.CollectionID),
			FinishTime: proto.Uint32(row.FinishTime),
			OverTime:   proto.Uint32(row.OverTime),
		})
	}
	return msg, nil
}
