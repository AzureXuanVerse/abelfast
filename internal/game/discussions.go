package game

import (
	"context"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Ship comments, the ShipEvaluationLayer of a ship group (view/ship): its
// hearts (LikeShipCommand), its comments shared by every commander of the
// server (FetchEvaluationCommand, EvaluateShipCommand), their reviews
// (ZanShipEvaCommand) and reports (ImpeachShipEvaCommand).

// maxDailyDiscussions is CollectionProxy.MAX_DAILY_EVA_COUNT: the comments
// a commander leaves on a ship group a day. The layer stops at it from
// SHIP_DISCUSS_INFO daily_discuss_count (ShipEvaluation.ievaCount), which
// CollectionProxy.resetEvaCount zeroes at the day change.
const maxDailyDiscussions = 1

// maxDiscussionLength is the longest comment taken, in characters; longer
// ones get evaluate_too_loog. The client has no limit of its own.
const maxDiscussionLength = 100

// maxReportLength bounds a report's reason: report_type_<n> or "Misc:" and
// up to 150 characters (report_type_other_1).
const maxReportLength = 200

// discussion reviews, CS_17105 good_or_bad
const (
	discussionGood = 0
	discussionBad  = 1 // the down button is hidden (LOCK_DOWNVOTE, const.lua)
)

// refusals the commands tell apart (EvaluateShipCommand, ZanShipEvaCommand)
// or that error_message words right
var (
	errDiscussionTooLong  error = refusal{"comment too long", 2011} // evaluate_too_loog
	errDiscussionEmpty    error = refusal{"empty comment", 2012}
	errDiscussionLimit    error = refusal{"daily comments reached", 6} // Already Reached Limit
	errAlreadyReviewed    error = refusal{"already reviewed", 7}       // zan_ship_eva_error_7
	errDiscussionNotFound error = refusal{"no such comment", 20}       // Data Does Not Exist
	errReportLevel        error = refusal{"level too low to report", 41}
)

// reportLevelLimit is gameset report_level_limit, the commander level from
// which comments can be reported (Player.IsOpenShipEvaluationImpeach).
var reportLevelLimit uint32

func loadDiscussionData() error {
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	limit, ok := gameset["report_level_limit"]
	if !ok {
		return gamedata.Missing("gameset", "report_level_limit")
	}
	reportLevelLimit = limit
	return nil
}

// shipDiscussion is a ship group's SHIP_DISCUSS_INFO for a commander
// (ShipEvaluation.Ctor): every comment newest first (the client sorts them
// and marks the two best hot), the server's hearts for the ship and the
// commander's comments on it today. ShipEvaluation starts every comment as
// not reviewed (izan): a second review is told apart by its result.
func shipDiscussion(ctx context.Context, queries *db.Queries, commanderID, groupID uint32) (*protobuf.SHIP_DISCUSS_INFO, error) {
	rows, err := queries.ListShipDiscussions(ctx, groupID)
	if err != nil {
		return nil, err
	}
	hearts, err := queries.CountShipHearts(ctx, groupID)
	if err != nil {
		return nil, err
	}
	today, err := queries.CountGroupDiscussionsSince(ctx, db.CountGroupDiscussionsSinceParams{
		CommanderID: commanderID, GroupID: groupID, Since: startOfDay(serverNow()).Unix(),
	})
	if err != nil {
		return nil, err
	}
	info := &protobuf.SHIP_DISCUSS_INFO{
		ShipGroupId:       proto.Uint32(groupID),
		DiscussCount:      proto.Uint32(uint32(len(rows))),
		HeartCount:        proto.Uint32(uint32(hearts)),
		DailyDiscussCount: proto.Uint32(uint32(today)),
		DiscussList:       make([]*protobuf.DISCUSS_INFO, 0, len(rows)),
	}
	for _, row := range rows {
		info.DiscussList = append(info.DiscussList, &protobuf.DISCUSS_INFO{
			Id:        proto.Uint32(row.ID),
			GoodCount: proto.Uint32(uint32(row.GoodCount)),
			BadCount:  proto.Uint32(uint32(row.BadCount)),
			NickName:  proto.String(row.NickName),
			Context:   proto.String(row.Context),
		})
	}
	return info, nil
}

// dailyDiscussions is SC_17001 daily_discuss, the comments the commander
// left today (CollectionProxy keeps it as dailyEvaCount, unused).
func dailyDiscussions(ctx context.Context, queries *db.Queries, commanderID uint32) (uint32, error) {
	count, err := queries.CountDiscussionsSince(ctx, db.CountDiscussionsSinceParams{
		CommanderID: commanderID, Since: startOfDay(serverNow()).Unix(),
	})
	return uint32(count), err
}

// ownsShipGroup tells whether the ship group is in the commander's archives:
// ships never recruited can't be liked or commented (npc_evaluation_tip).
func ownsShipGroup(ctx context.Context, queries *db.Queries, commanderID, groupID uint32) (bool, error) {
	count, err := queries.CountOwnedShipGroup(ctx, db.CountOwnedShipGroupParams{CommanderID: commanderID, GroupID: groupID})
	return count > 0, err
}

// handleFetchDiscussion answers FetchEvaluationCommand (CS_17101). SC_17102
// has no result, the client only takes a SHIP_DISCUSS_INFO of the group it
// asked for.
func (game *Game) handleFetchDiscussion(client *connection.Client, req *protobuf.CS_17101) error {
	info, err := shipDiscussion(context.Background(), game.DB.Queries, client.CommanderID, req.GetShipGroupId())
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_17102{ShipDiscuss: info})
}

// handlePostDiscussion answers EvaluateShipCommand (CS_17103): the comment
// is added under the commander's name and SC_17104 carries the group's
// SHIP_DISCUSS_INFO, which replaces the client's. need_level is only read
// for result 41.
func (game *Game) handlePostDiscussion(client *connection.Client, req *protobuf.CS_17103) error {
	ctx := context.Background()
	groupID := req.GetShipGroupId()
	text := req.GetContext()
	var info *protobuf.SHIP_DISCUSS_INFO
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		switch length := utf8.RuneCountInString(text); {
		case strings.TrimSpace(text) == "":
			return errDiscussionEmpty
		case length > maxDiscussionLength || !utf8.ValidString(text):
			return errDiscussionTooLong
		}
		owned, err := ownsShipGroup(ctx, queries, client.CommanderID, groupID)
		if err != nil {
			return err
		}
		if !owned {
			return errRefused
		}
		now := serverNow()
		today, err := queries.CountGroupDiscussionsSince(ctx, db.CountGroupDiscussionsSinceParams{
			CommanderID: client.CommanderID, GroupID: groupID, Since: startOfDay(now).Unix(),
		})
		if err != nil {
			return err
		}
		if today >= maxDailyDiscussions {
			return errDiscussionLimit
		}
		if err := queries.CreateShipDiscussion(ctx, db.CreateShipDiscussionParams{
			GroupID: groupID, CommanderID: client.CommanderID, Context: text, PostedAt: now.Unix(),
		}); err != nil {
			return err
		}
		info, err = shipDiscussion(ctx, queries, client.CommanderID, groupID)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17104{Result: result(err), ShipDiscuss: info, NeedLevel: proto.Uint32(0)})
}

// handleReviewDiscussion answers ZanShipEvaCommand (CS_17105): one review
// per commander and comment, the client counts it itself on result 0 and
// marks the comment reviewed on result 7.
func (game *Game) handleReviewDiscussion(client *connection.Client, req *protobuf.CS_17105) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		if req.GetGoodOrBad() != discussionGood {
			return errRefused
		}
		found, err := queries.CountDiscussionInGroup(ctx, db.CountDiscussionInGroupParams{ID: req.GetDiscussId(), GroupID: req.GetShipGroupId()})
		if err != nil {
			return err
		}
		if found == 0 {
			return errDiscussionNotFound
		}
		added, err := queries.AddShipDiscussionVote(ctx, db.AddShipDiscussionVoteParams{
			DiscussionID: req.GetDiscussId(), CommanderID: client.CommanderID, GoodOrBad: req.GetGoodOrBad(),
		})
		if err != nil {
			return err
		}
		if added == 0 {
			return errAlreadyReviewed
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17106{Result: result(err)})
}

// handleLikeShip answers LikeShipCommand (CS_17107): a ship group of the
// archives is liked once (SHIP_STATISTICS_INFO heart_flag), the client adds
// the heart itself on result 0. heart_count counts the server's likes.
func (game *Game) handleLikeShip(client *connection.Client, req *protobuf.CS_17107) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		owned, err := ownsShipGroup(ctx, queries, client.CommanderID, req.GetShipGroupId())
		if err != nil {
			return err
		}
		if !owned {
			return errRefused
		}
		liked, err := queries.SetShipGroupHeart(ctx, db.SetShipGroupHeartParams{CommanderID: client.CommanderID, GroupID: req.GetShipGroupId()})
		if err != nil {
			return err
		}
		if liked == 0 {
			return errAlreadyReviewed
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17108{Result: result(err)})
}

// handleReportDiscussion answers ImpeachShipEvaCommand (CS_17109): the
// report is kept, report_sent_thank shows on result 0. Reporting opens at
// gameset report_level_limit.
func (game *Game) handleReportDiscussion(client *connection.Client, req *protobuf.CS_17109) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		commander, err := queries.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if commander.Level < reportLevelLimit {
			return errReportLevel
		}
		reason := req.GetReason()
		if strings.TrimSpace(reason) == "" || utf8.RuneCountInString(reason) > maxReportLength {
			return errRefused
		}
		found, err := queries.CountDiscussionInGroup(ctx, db.CountDiscussionInGroupParams{ID: req.GetDiscussId(), GroupID: req.GetShipGroupId()})
		if err != nil {
			return err
		}
		if found == 0 {
			return errDiscussionNotFound
		}
		return queries.SetShipDiscussionReport(ctx, db.SetShipDiscussionReportParams{
			DiscussionID: req.GetDiscussId(), CommanderID: client.CommanderID, Reason: reason, ReportedAt: serverNow().Unix(),
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17110{Result: result(err)})
}
