package game

import (
	"context"
	"math"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Shared loadouts, the EquipCodeShareLayer a ship's EquipCodeScene opens
// (view/ship): gear codes shared for a ship group by every commander of the
// server (EquipCodeRequestCommand, EquipCodeShareCommand), their likes
// (EquipCodeLikeCommand) and reports (EquipCodeImpeachCommand). None of
// these packets were captured: the replies are built from the Lua alone.

// A shared code is EquipCodeScene's <gear code>&<ship group in base 32
// (ConversionBase)>&<tag>&<tag>, the two tags being the equip_share_label_<n>
// picked in the share dialog. EquipCode.IsValid drops codes that don't split
// in four or whose group isn't the one asked for.
const (
	equipCodeTags      = 9   // IndexConst.LABEL_COUNT
	maxEquipCodeLength = 256 // the gear part, buildEquipCode isn't in the Lua
)

// share limits: equipcode_share_errorcode7 (a ship group once a day) and
// equipcode_share_errorcode44 (shares a day, the client doesn't say how many)
const maxDailyEquipCodeShares = 5

// maxDailyEquipCodeReports is the reports a commander makes a day before
// they get equipcode_report_warning.
const maxDailyEquipCodeReports = 10

// equipCodeHiddenReports is the commanders reporting a code from which its
// state turns 1: EquipCodeShareLayer leaves it out of the most liked pool
// it shows first (state 0 only).
const equipCodeHiddenReports = 3

// SC_17602 lists: recent_infos are the codes shared over the last
// equipCodeRecentDays (EquipCode new 1, the layer's newPool), infos the
// older ones most liked first (new 0: the 12 most liked of them with state 0
// are shown first, by eval_point). eval_point is the likes of the last
// equipCodeEvalDays.
const (
	equipCodeRecentDays = 3
	equipCodeEvalDays   = 7
	maxRecentEquipCodes = 20
	maxOlderEquipCodes  = 100
)

// CS_17607 report_type, the impeach panel's options
const (
	equipCodeReportNonsense = 1 // equipcode_report_type_1
	equipCodeReportOutdated = 2 // equipcode_report_type_2
)

var (
	errEquipCodeShared     error = refusal{"ship group shared today", 7} // equipcode_share_errorcode7
	errEquipCodeShareLimit error = refusal{"daily shares reached", 44}   // equipcode_share_errorcode44
	errEquipCodeLiked      error = refusal{"code liked today", 7}        // equipcode_like_limited
	errEquipCodeNotFound   error = refusal{"no such equip code", 20}     // Data Does Not Exist
	// -1, equipcode_report_warning (result is a uint32 on the wire)
	errEquipCodeReportLimit error = refusal{"daily reports reached", math.MaxUint32}
)

// validEquipCode tells whether code is one EquipCodeScene builds for the
// ship group and EquipCode.IsValid takes.
func validEquipCode(code string, groupID uint32) bool {
	parts := strings.Split(code, "&")
	if len(parts) != 4 || parts[0] == "" || len(parts[0]) > maxEquipCodeLength {
		return false
	}
	for _, c := range parts[0] {
		if c <= ' ' || c > '~' {
			return false
		}
	}
	group, err := strconv.ParseUint(parts[1], 32, 32)
	if err != nil || uint32(group) != groupID {
		return false
	}
	var tags [2]int
	for i, part := range parts[2:] {
		tag, err := strconv.Atoi(part)
		if err != nil || tag < 1 || tag > equipCodeTags {
			return false
		}
		tags[i] = tag
	}
	return tags[0] != tags[1]
}

// equipCodeInfo is a listed code's EQCODE_SHARE_INFO.
func equipCodeInfo(id uint32, code string, likes uint32, recentLikes, reports int64) *protobuf.EQCODE_SHARE_INFO {
	state := uint32(0)
	if reports >= equipCodeHiddenReports {
		state = 1
	}
	return &protobuf.EQCODE_SHARE_INFO{
		Id:        proto.Uint32(id),
		Eqcode:    proto.String(code),
		Like:      proto.Uint32(likes),
		EvalPoint: proto.Uint32(uint32(recentLikes)),
		State:     proto.Uint32(state),
	}
}

// handleRequestEquipCodes answers EquipCodeRequestCommand (CS_17601), sent
// when EquipCodeShareLayer opens: the client keeps the valid codes of infos
// (new 0) and recent_infos (new 1) on its ShipGroup, which must be in its
// archives. Any other result only shows "Request equip code data failed".
func (game *Game) handleRequestEquipCodes(client *connection.Client, req *protobuf.CS_17601) error {
	ctx := context.Background()
	groupID := req.GetShipgroup()
	reply := &protobuf.SC_17602{}
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		owned, err := ownsShipGroup(ctx, queries, client.CommanderID, groupID)
		if err != nil {
			return err
		}
		if !owned {
			return errRefused
		}
		today := startOfDay(serverNow())
		recentSince := today.AddDate(0, 0, 1-equipCodeRecentDays).Unix()
		likedSince := today.AddDate(0, 0, 1-equipCodeEvalDays).Unix()
		recent, err := queries.ListRecentEquipCodes(ctx, db.ListRecentEquipCodesParams{
			LikedSince: likedSince, GroupID: groupID, SharedSince: recentSince, MaxRows: maxRecentEquipCodes,
		})
		if err != nil {
			return err
		}
		older, err := queries.ListOlderEquipCodes(ctx, db.ListOlderEquipCodesParams{
			LikedSince: likedSince, GroupID: groupID, SharedBefore: recentSince, MaxRows: maxOlderEquipCodes,
		})
		if err != nil {
			return err
		}
		reply.RecentInfos = make([]*protobuf.EQCODE_SHARE_INFO, 0, len(recent))
		for _, row := range recent {
			reply.RecentInfos = append(reply.RecentInfos, equipCodeInfo(row.ID, row.Eqcode, row.Likes, row.RecentLikes, row.Reports))
		}
		reply.Infos = make([]*protobuf.EQCODE_SHARE_INFO, 0, len(older))
		for _, row := range older {
			reply.Infos = append(reply.Infos, equipCodeInfo(row.ID, row.Eqcode, row.Likes, row.RecentLikes, row.Reports))
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		reply.Infos, reply.RecentInfos = nil, nil
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleShareEquipCode answers EquipCodeShareCommand (CS_17603): the
// commander shares the loadout of one of their ships with two tags, a ship
// group once a day (result 7) and maxDailyEquipCodeShares a day (result 44).
func (game *Game) handleShareEquipCode(client *connection.Client, req *protobuf.CS_17603) error {
	ctx := context.Background()
	groupID := req.GetShipgroup()
	code := req.GetEqcode()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		if !validEquipCode(code, groupID) {
			return errRefused
		}
		owned, err := ownsShipGroup(ctx, queries, client.CommanderID, groupID)
		if err != nil {
			return err
		}
		if !owned {
			return errRefused
		}
		now := serverNow()
		since := startOfDay(now).Unix()
		group, err := queries.CountGroupEquipCodesSince(ctx, db.CountGroupEquipCodesSinceParams{
			CommanderID: client.CommanderID, GroupID: groupID, Since: since,
		})
		if err != nil {
			return err
		}
		if group > 0 {
			return errEquipCodeShared
		}
		total, err := queries.CountEquipCodesSince(ctx, db.CountEquipCodesSinceParams{CommanderID: client.CommanderID, Since: since})
		if err != nil {
			return err
		}
		if total >= maxDailyEquipCodeShares {
			return errEquipCodeShareLimit
		}
		return queries.CreateEquipCode(ctx, db.CreateEquipCodeParams{
			GroupID: groupID, CommanderID: client.CommanderID, Eqcode: code, SharedAt: now.Unix(),
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17604{Result: result(err)})
}

// handleLikeEquipCode answers EquipCodeLikeCommand (CS_17605): a commander
// likes a code once a day. The client adds the like itself on result 0 and
// shows equipcode_like_limited on result 7, marking the code liked either way.
func (game *Game) handleLikeEquipCode(client *connection.Client, req *protobuf.CS_17605) error {
	ctx := context.Background()
	shareID := req.GetShareid()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		found, err := queries.CountEquipCodeInGroup(ctx, db.CountEquipCodeInGroupParams{ID: shareID, GroupID: req.GetShipgroup()})
		if err != nil {
			return err
		}
		if found == 0 {
			return errEquipCodeNotFound
		}
		now := serverNow()
		liked, err := queries.CountEquipCodeLikesSince(ctx, db.CountEquipCodeLikesSinceParams{
			ShareID: shareID, CommanderID: client.CommanderID, Since: startOfDay(now).Unix(),
		})
		if err != nil {
			return err
		}
		if liked > 0 {
			return errEquipCodeLiked
		}
		if err := queries.SetEquipCodeLike(ctx, db.SetEquipCodeLikeParams{
			ShareID: shareID, CommanderID: client.CommanderID, LikedAt: now.Unix(),
		}); err != nil {
			return err
		}
		return queries.AddEquipCodeLike(ctx, shareID)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17606{Result: result(err)})
}

// handleReportEquipCode answers EquipCodeImpeachCommand (CS_17607): the
// report is kept (a commander's latest per code) and equipcode_dislike_success
// shows on result 0. Past maxDailyEquipCodeReports a day the report is
// turned down with -1, equipcode_report_warning.
func (game *Game) handleReportEquipCode(client *connection.Client, req *protobuf.CS_17607) error {
	ctx := context.Background()
	shareID := req.GetShareid()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		switch req.GetReportType() {
		case equipCodeReportNonsense, equipCodeReportOutdated:
		default:
			return errRefused
		}
		found, err := queries.CountEquipCodeInGroup(ctx, db.CountEquipCodeInGroupParams{ID: shareID, GroupID: req.GetShipgroup()})
		if err != nil {
			return err
		}
		if found == 0 {
			return errEquipCodeNotFound
		}
		now := serverNow()
		reports, err := queries.CountEquipCodeReportsSince(ctx, db.CountEquipCodeReportsSinceParams{
			CommanderID: client.CommanderID, Since: startOfDay(now).Unix(),
		})
		if err != nil {
			return err
		}
		if reports >= maxDailyEquipCodeReports {
			return errEquipCodeReportLimit
		}
		return queries.SetEquipCodeReport(ctx, db.SetEquipCodeReportParams{
			ShareID: shareID, CommanderID: client.CommanderID, ReportType: req.GetReportType(), ReportedAt: now.Unix(),
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17608{Result: result(err)})
}
