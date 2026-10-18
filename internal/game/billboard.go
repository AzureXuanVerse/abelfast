package game

import (
	"context"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// PowerRank.TYPE_* (model/vo/powerrank.lua): the leaderboards. The exercise
// one has its own packet (CS_18006).
const rankCollection = 2

// GetPowerRankCommand asks for the first 100 entries in pages of 20.
const rankPageSize = 20

// nationLink is Nation.LINK: nations above it are collaborations.
const nationLink = 100

// rankPoints is every commander's points on a leaderboard, best first (by
// id among equals), and whether the server can rank it. Only the collection
// is: the power is the client's own sum of its ships' stats
// (BayProxy:getBayPower) and the others are event scores.
func (game *Game) rankPoints(ctx context.Context, kind uint32) ([]*protobuf.RANK_INFO_P18, bool, error) {
	if kind != rankCollection {
		return nil, false, nil
	}
	standings, err := listExerciseStandings(ctx, game.DB.Queries, time.Now())
	if err != nil {
		return nil, false, err
	}
	var list []*protobuf.RANK_INFO_P18
	for _, row := range standings {
		points, err := collectionCount(ctx, game.DB.Queries, row.ID)
		if err != nil {
			return nil, false, err
		}
		commander, err := game.DB.GetCommander(ctx, row.ID)
		if err != nil {
			return nil, false, err
		}
		state, err := loadExercise(ctx, game.DB.Queries, row.ID, time.Now())
		if err != nil {
			return nil, false, err
		}
		list = append(list, &protobuf.RANK_INFO_P18{
			UserId:    proto.Uint32(row.ID),
			Point:     proto.Uint32(points),
			Name:      proto.String(row.Name),
			Lv:        proto.Uint32(row.Level),
			ArenaRank: proto.Uint32(state.BestRank),
			Display:   game.displayInfo(commander),
		})
	}
	slices.SortStableFunc(list, func(a, b *protobuf.RANK_INFO_P18) int { return int(b.GetPoint()) - int(a.GetPoint()) })
	return list, true, nil
}

// collectionCount is CollectionProxy:getCollectionCount: the archive's ship
// groups collected, collaborations left out (rankedGroups). Retrofits would
// count twice, but they aren't kept.
func collectionCount(ctx context.Context, queries *db.Queries, commanderID uint32) (uint32, error) {
	groups, err := queries.ListShipGroups(ctx, commanderID)
	if err != nil {
		return 0, err
	}
	count := uint32(0)
	for _, group := range groups {
		if rankedGroups[group.GroupID] {
			count++
		}
	}
	return count, nil
}

// handleRankPage answers a page of a leaderboard (GetPowerRankCommand):
// empty for the ones the server can't rank (rankPoints).
func (game *Game) handleRankPage(client *connection.Client, req *protobuf.CS_18201) error {
	list, _, err := game.rankPoints(context.Background(), req.GetType())
	if err != nil {
		return err
	}
	start := int(max(req.GetPage(), 1)-1) * rankPageSize
	reply := &protobuf.SC_18202{}
	if start < len(list) {
		reply.List = list[start:min(len(list), start+rankPageSize)]
	}
	return client.Send(reply)
}

// handleRankSelf answers the commander's own place on a leaderboard
// (GetPowerRankCommand): rank 0 (unranked) on the ones the server can't
// rank, where the client shows its own power or event points anyway.
func (game *Game) handleRankSelf(client *connection.Client, req *protobuf.CS_18203) error {
	list, ranked, err := game.rankPoints(context.Background(), req.GetType())
	if err != nil {
		return err
	}
	reply := &protobuf.SC_18204{Point: proto.Uint32(0), Rank: proto.Uint32(0)}
	if ranked {
		if i := slices.IndexFunc(list, func(entry *protobuf.RANK_INFO_P18) bool { return entry.GetUserId() == client.CommanderID }); i >= 0 {
			reply.Point, reply.Rank = proto.Uint32(list[i].GetPoint()), proto.Uint32(uint32(i+1))
		}
	}
	return client.Send(reply)
}
