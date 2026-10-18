package game

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"math/rand/v2"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Guild operations (GuildEvent, model/vo/guild/events): an administrator
// starts an operation for guild funds; members join it (twice a month, then
// extra operations) and dispatch fleets to its missions, a tree of main
// missions (one per position, each unlocking the next position) with side
// missions. A mission progresses with time at a speed its dispatched fleets'
// efficiency raises, its nodes succeed or fail as the progress passes them,
// and once done it leaves a report the members that took part claim the
// rewards of. When the last main mission is done the operation's boss
// appears; members fight it once a day and its defeat ends the operation.
// No official capture covers any of it: everything here is built from the
// Lua alone (controller/command/guild/events, model/vo/guild/events, the
// guild event views) and the guild tables; the formulas the server keeps
// (mission speed, scores, node outcomes, boss hp) are belfast's.

// GuildReport.SCORE_TYPE_*: a finished mission's grade
const (
	guildScoreS = 1
	guildScoreA = 2
	guildScoreB = 3
)

// GuildMissionNode.STATE_*
const (
	guildNodeAhead   = 0
	guildNodeSuccess = 1
	guildNodeFailure = 2
)

// GuildConst.REPORT_STATE_* and REPORT_TYPE_*
const (
	guildReportLocked    = 0
	guildReportUnlocked  = 1
	guildReportSubmitted = 2
	guildReportMission   = 1
	guildReportBoss      = 2
)

// errGuildBossOver is the result GetGuildBossInfoCommand and
// BattleGateGuild read as "the operation / the boss is over" (20).
var errGuildBossOver error = refusal{"guild operation over", 20}

// guildOperationState is a guild's open operation, loaded for a request.
type guildOperationState struct {
	op       db.GuildOperation
	missions []db.GuildOperationMission
	nodes    []db.GuildOperationNode
	ships    []db.GuildOperationShip
}

// openGuildOperation loads the guild's operation running at now, ok false
// when there is none (never started, expired or its boss defeated).
func openGuildOperation(ctx context.Context, queries *db.Queries, guildID uint32, now time.Time) (state *guildOperationState, ok bool, err error) {
	op, err := queries.GetOpenGuildOperation(ctx, db.GetOpenGuildOperationParams{GuildID: guildID, Now: uint32(now.Unix())})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	state = &guildOperationState{op: op}
	if state.missions, err = queries.ListGuildMissions(ctx, op.ID); err != nil {
		return nil, false, err
	}
	if state.nodes, err = queries.ListGuildMissionNodes(ctx, op.ID); err != nil {
		return nil, false, err
	}
	if state.ships, err = queries.ListGuildMissionShips(ctx, op.ID); err != nil {
		return nil, false, err
	}
	return state, true, nil
}

// mission is the operation's mission of event id, nil when it has none.
func (state *guildOperationState) mission(eventID uint32) *db.GuildOperationMission {
	for i := range state.missions {
		if state.missions[i].EventID == eventID {
			return &state.missions[i]
		}
	}
	return nil
}

// lastPosition is the position of the tree's last main mission (the boss
// sits after it, GuildEvent:Active).
func (state *guildOperationState) lastPosition() uint32 {
	last := uint32(0)
	for _, mission := range state.missions {
		last = max(last, mission.Position)
	}
	return last
}

// unlocked tells whether a mission can be worked on: the first position's,
// or once the previous position's main mission is done
// (GuildViewMissionNode:IsUnLock).
func (state *guildOperationState) unlocked(mission *db.GuildOperationMission) bool {
	if mission.Position <= 1 {
		return true
	}
	for _, other := range state.missions {
		if other.Position == mission.Position-1 && guildBaseEvents[other.EventID].SubType == 1 {
			return other.FinishedAt != 0
		}
	}
	return false
}

// withdraw takes a member leaving the guild out of the operation: the work
// done until now stays (settled first), their fleets leave the missions
// still running, with the efficiency they brought, their boss fleets and
// the support ships of theirs the others' boss fleets borrowed go, and
// they stop taking part (coming back is a new join: its month's count is
// on the commander, guildOperationJoins).
func (state *guildOperationState) withdraw(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) error {
	if err := state.settle(ctx, queries, now); err != nil {
		return err
	}
	for i := range state.missions {
		mission := &state.missions[i]
		if mission.FinishedAt != 0 {
			continue
		}
		var gone uint32
		for _, ship := range state.missionShips(mission.EventID) {
			if ship.CommanderID == commanderID {
				gone += ship.Efficiency
			}
		}
		if gone == 0 {
			continue
		}
		mission.Efficiency -= min(gone, mission.Efficiency)
		if err := saveGuildMission(ctx, queries, *mission); err != nil {
			return err
		}
		if err := queries.DeleteGuildMissionShipsOf(ctx, db.DeleteGuildMissionShipsOfParams{
			OperationID: state.op.ID, EventID: mission.EventID, CommanderID: commanderID,
		}); err != nil {
			return err
		}
	}
	if err := queries.DeleteGuildBossFleetShipsOf(ctx, db.DeleteGuildBossFleetShipsOfParams{OperationID: state.op.ID, CommanderID: commanderID}); err != nil {
		return err
	}
	if err := queries.DeleteGuildBossFleetMeowfficersOf(ctx, db.DeleteGuildBossFleetMeowfficersOfParams{OperationID: state.op.ID, CommanderID: commanderID}); err != nil {
		return err
	}
	return queries.DeleteGuildOperationMember(ctx, db.DeleteGuildOperationMemberParams{OperationID: state.op.ID, CommanderID: commanderID})
}

// missionShips are the ships dispatched to a mission.
func (state *guildOperationState) missionShips(eventID uint32) []db.GuildOperationShip {
	var ships []db.GuildOperationShip
	for _, ship := range state.ships {
		if ship.EventID == eventID {
			ships = append(ships, ship)
		}
	}
	return ships
}

// guildMissionSpeed is how much of a mission's workload a second does:
// guildset efficiency_param_base (10) without fleets, which puts the
// missions at the wiki's durations (workload 907200 is 25.2 h), raised by
// the dispatched fleets' total efficiency E toward efficiency_param_max
// times as fast: bonus (max-1) * E / (E + param/2), param
// efficiency_param_normal (efficiency_param_elite for an elite mission). The
// wiki measured 200% * E / (E + 7000) on common missions; the server-side
// formula itself isn't known.
func guildMissionSpeed(event guildBaseEventConfig, efficiency uint32) float64 {
	param := guildEfficiencyNormal
	if event.Type == 2 {
		param = guildEfficiencyElite
	}
	e := float64(efficiency)
	bonus := float64(guildEfficiencyMax-1) * e / (e + float64(param)/2)
	return float64(guildEfficiencyBase) * (1 + bonus)
}

// guildShipAttr is a dispatched ship's attribute (attribute_info_by_type id,
// Ship.PROPERTIES order from 1) at its level, without gear or bonuses.
func guildShipAttr(ship db.GuildOperationShip, attr uint32) float64 {
	stats, ok := guildShipAttrs[ship.TemplateID]
	i := int(attr) - 1
	if !ok || i < 0 || i >= len(stats.Attrs) || i >= len(stats.AttrsGrowth) {
		return 0
	}
	value := stats.Attrs[i] + (float64(ship.Level)-1)*stats.AttrsGrowth[i]/1000
	if ship.Level > extraAttrLevelLimit && i < len(stats.AttrsGrowthExtra) {
		value += float64(ship.Level-extraAttrLevelLimit) * stats.AttrsGrowthExtra[i] / 1000
	}
	return math.Floor(value)
}

// guildMissionAttrs are a mission's attribute targets as they stand: the
// totals of event_attr_acc_effect and the ship counts of
// event_attr_count_effect, by attribute, and the score the met ones add.
func guildMissionAttrs(event guildBaseEventConfig, ships []db.GuildOperationShip) (acc, count map[uint32]uint32, score uint32) {
	acc, count = map[uint32]uint32{}, map[uint32]uint32{}
	for _, effect := range event.AttrAcc {
		if len(effect) < 4 {
			continue
		}
		total := 0.0
		for _, ship := range ships {
			total += guildShipAttr(ship, effect[0])
		}
		acc[effect[0]] = uint32(total)
		if uint32(total) >= effect[2] {
			score += effect[3]
		}
	}
	for _, effect := range event.AttrCount {
		if len(effect) < 4 {
			continue
		}
		n := uint32(0)
		for _, ship := range ships {
			if guildShipAttr(ship, effect[0]) >= float64(effect[1]) {
				n++
			}
		}
		count[effect[0]] = n
		if n >= effect[2] {
			score += effect[3]
		}
	}
	return acc, count, score
}

// guildMissionScore grades a mission: base_point for every fleet
// dispatched plus the met attribute targets' score, against result_point
// (S, A thresholds). The server-side rule isn't known.
func guildMissionScore(event guildBaseEventConfig, ships []db.GuildOperationShip) uint32 {
	fleets := map[[2]uint32]bool{}
	for _, ship := range ships {
		fleets[[2]uint32{ship.CommanderID, ship.Fleet}] = true
	}
	_, _, bonus := guildMissionAttrs(event, ships)
	points := event.BasePoint*uint32(len(fleets)) + bonus
	switch {
	case len(event.ResultPoint) > 0 && points >= event.ResultPoint[0]:
		return guildScoreS
	case len(event.ResultPoint) > 1 && points >= event.ResultPoint[1]:
		return guildScoreA
	}
	return guildScoreB
}

// settle brings the operation up to now: missions progress, the nodes
// their progress passed succeed (the mission grading A or better then) or
// fail, finished missions leave their report, and the boss appears once
// the last main mission is done. Progress stops at the operation's end.
func (state *guildOperationState) settle(ctx context.Context, queries *db.Queries, now time.Time) error {
	end := min(uint32(now.Unix()), state.op.EndTime)
	for i := range state.missions {
		mission := &state.missions[i]
		if mission.StartTime == 0 || mission.FinishedAt != 0 || mission.UpdatedAt >= end {
			continue
		}
		event := guildBaseEvents[mission.EventID]
		speed := guildMissionSpeed(event, mission.Efficiency)
		before, since := mission.Work, mission.UpdatedAt
		mission.Work = min(float64(event.Workload), before+speed*float64(end-since))
		mission.UpdatedAt = end
		ships := state.missionShips(mission.EventID)
		score := guildMissionScore(event, ships)
		for j := range state.nodes {
			node := &state.nodes[j]
			if node.EventID != mission.EventID || node.Status != guildNodeAhead ||
				float64(node.Position)*float64(event.Workload)/100 > mission.Work {
				continue
			}
			node.Status = guildNodeFailure
			if score <= guildScoreA {
				node.Status = guildNodeSuccess
			}
			if err := queries.SetGuildMissionNodeStatus(ctx, db.SetGuildMissionNodeStatusParams{
				Status: node.Status, OperationID: node.OperationID, EventID: node.EventID, Position: node.Position,
			}); err != nil {
				return err
			}
		}
		if mission.Work >= float64(event.Workload) {
			mission.FinishedAt = since + uint32(math.Ceil((float64(event.Workload)-before)/speed))
			mission.Score = score
			if err := state.addReport(ctx, queries, guildReportMission, mission.EventID, score, mission.FinishedAt, ships); err != nil {
				return err
			}
		}
		if err := saveGuildMission(ctx, queries, *mission); err != nil {
			return err
		}
	}
	if !state.op.BossActive {
		last := state.lastPosition()
		for _, mission := range state.missions {
			if mission.Position == last && guildBaseEvents[mission.EventID].SubType == 1 && mission.FinishedAt != 0 {
				state.op.BossActive = true
				return queries.ActivateGuildBoss(ctx, state.op.ID)
			}
		}
	}
	return nil
}

func saveGuildMission(ctx context.Context, queries *db.Queries, mission db.GuildOperationMission) error {
	return queries.SaveGuildMission(ctx, db.SaveGuildMissionParams{
		StartTime: mission.StartTime, Work: mission.Work, UpdatedAt: mission.UpdatedAt, Efficiency: mission.Efficiency,
		FinishedAt: mission.FinishedAt, Score: mission.Score, OperationID: mission.OperationID, EventID: mission.EventID,
	})
}

// addReport writes a report of the operation, claimable by the commanders
// whose ships took part (unlocked), locked for the others.
func (state *guildOperationState) addReport(ctx context.Context, queries *db.Queries, kind, eventID, score, at uint32, ships []db.GuildOperationShip) error {
	id, err := queries.AddGuildReport(ctx, db.AddGuildReportParams{
		GuildID: state.op.GuildID, OperationID: state.op.ID, EventID: eventID, EventType: kind, Score: score, Time: at,
	})
	if err != nil {
		return err
	}
	done := map[uint32]bool{}
	for _, ship := range ships {
		if done[ship.CommanderID] {
			continue
		}
		done[ship.CommanderID] = true
		if err := queries.SetGuildReportClaim(ctx, db.SetGuildReportClaimParams{
			ReportID: id, CommanderID: ship.CommanderID, Status: guildReportUnlocked,
		}); err != nil {
			return err
		}
	}
	return nil
}

// guildBossDaily is the member's boss fights today.
func guildBossDaily(member db.GuildOperationMember, now time.Time) uint32 {
	if member.BossDay != uint32(startOfDay(now).Unix()) {
		return 0
	}
	return member.BossCount
}

// eventBase is a mission as GuildMission reads it for commanderID: the
// times its progress is drawn from (start_time to complete_time, the time
// left at the current speed, both 0 until a fleet is dispatched), every
// member's ships, the commander's fleets (personship), the attribute
// targets, nodes and total efficiency.
func (state *guildOperationState) eventBase(mission db.GuildOperationMission, commanderID uint32, now time.Time) *protobuf.EVENT_BASE {
	event := guildBaseEvents[mission.EventID]
	ships := state.missionShips(mission.EventID)
	base := &protobuf.EVENT_BASE{
		EventId:    proto.Uint32(mission.EventID),
		Position:   proto.Uint32(mission.Position),
		StartTime:  proto.Uint32(0),
		Efficiency: proto.Uint32(mission.Efficiency),
	}
	if mission.StartTime != 0 {
		speed := guildMissionSpeed(event, mission.Efficiency)
		complete := uint32(now.Unix()) + uint32(math.Ceil((float64(event.Workload)-mission.Work)/speed))
		base.CompleteTime = proto.Uint32(complete)
		base.StartTime = proto.Uint32(complete - uint32(float64(event.Workload)/speed))
	}
	pages := map[uint32]*protobuf.PERSON_SHIP_IN_PAGE{}
	for _, ship := range ships {
		base.Shipinevent = append(base.Shipinevent, &protobuf.SHIP_IN_EVENT{
			UserId: proto.Uint32(ship.CommanderID), ShipId: proto.Uint32(ship.ShipID),
			TemplateId: proto.Uint32(ship.TemplateID), Skin: proto.Uint32(ship.SkinID),
		})
		if ship.CommanderID != commanderID {
			continue
		}
		page := pages[ship.Fleet]
		if page == nil {
			page = &protobuf.PERSON_SHIP_IN_PAGE{PageId: proto.Uint32(ship.Fleet)}
			pages[ship.Fleet] = page
			base.Personship = append(base.Personship, page)
		}
		page.ShipIds = append(page.ShipIds, ship.ShipID)
	}
	acc, count, _ := guildMissionAttrs(event, ships)
	for _, effect := range event.AttrAcc {
		if len(effect) > 0 {
			base.AttrAccList = append(base.AttrAccList, keyValueP61(effect[0], acc[effect[0]]))
		}
	}
	for _, effect := range event.AttrCount {
		if len(effect) > 0 {
			base.AttrCountList = append(base.AttrCountList, keyValueP61(effect[0], count[effect[0]]))
		}
	}
	for _, node := range state.nodes {
		if node.EventID == mission.EventID {
			base.Eventnodes = append(base.Eventnodes, &protobuf.EVENT_NODE{
				Position: proto.Uint32(node.Position), NodeId: proto.Uint32(node.NodeID), Status: proto.Uint32(node.Status),
			})
		}
	}
	return base
}

func keyValueP61(key, value uint32) *protobuf.KEYVALUE_P61 {
	return &protobuf.KEYVALUE_P61{Key: proto.Uint32(key), Value: proto.Uint32(value)}
}

// eventBoss is the boss as GuildBossMission:Flush reads it.
func (state *guildOperationState) eventBoss() *protobuf.EVENT_BOSS {
	return &protobuf.EVENT_BOSS{
		BossId: proto.Uint32(state.op.BossID),
		Damage: proto.Uint32(state.op.BossDamage),
		Hp:     proto.Uint32(state.op.BossHp),
	}
}

// currentOperation is the operation as GuildEvent:Active reads it for
// commanderID: unfinished missions in base_events, finished ones in
// completed_events (GuildMission.CompleteData2FullData marks them
// server_finish, which makes the event page ask for the boss), the boss once
// it appeared, the commander's node animations, dispatch times, boss fleets
// and fights today, monthly joins and participation.
func (game *Game) currentOperation(ctx context.Context, queries *db.Queries, state *guildOperationState, commanderID uint32, now time.Time) (*protobuf.CURRENT_OPERATION, error) {
	operation := &protobuf.CURRENT_OPERATION{
		OperationId: proto.Uint32(state.op.OperationID),
		StartTime:   proto.Uint32(state.op.StartTime),
	}
	formation := map[uint32]uint32{}
	for _, ship := range state.ships {
		if ship.CommanderID == commanderID {
			formation[ship.EventID] = max(formation[ship.EventID], ship.DispatchedAt)
		}
	}
	for _, mission := range state.missions {
		if mission.FinishedAt != 0 {
			operation.CompletedEvents = append(operation.CompletedEvents, &protobuf.EVENT_BASE_COMPLETED{
				EventId: proto.Uint32(mission.EventID), Position: proto.Uint32(mission.Position),
			})
			continue
		}
		operation.BaseEvents = append(operation.BaseEvents, state.eventBase(mission, commanderID, now))
		if at := formation[mission.EventID]; at != 0 {
			operation.FormationTime = append(operation.FormationTime, keyValueP61(mission.EventID, at))
		}
	}
	if state.op.BossActive {
		operation.BossEvent = state.eventBoss()
	}
	perfs, err := queries.ListGuildMissionPerfs(ctx, db.ListGuildMissionPerfsParams{OperationID: state.op.ID, CommanderID: commanderID})
	if err != nil {
		return nil, err
	}
	for _, perf := range perfs {
		operation.Perfs = append(operation.Perfs, &protobuf.EVENT_PERFORMANCE{EventId: proto.Uint32(perf.EventID), Index: proto.Uint32(perf.Idx)})
	}
	if operation.Fleets, err = guildBossFleets(ctx, queries, state.op.ID, commanderID); err != nil {
		return nil, err
	}
	joins, err := guildOperationJoins(ctx, queries, commanderID, now)
	if err != nil {
		return nil, err
	}
	operation.JoinTimes = proto.Uint32(joins)
	member, err := queries.GetGuildOperationMember(ctx, db.GetGuildOperationMemberParams{OperationID: state.op.ID, CommanderID: commanderID})
	switch {
	case err == nil:
		operation.IsParticipant = proto.Uint32(1)
		operation.DailyCount = proto.Uint32(guildBossDaily(member, now))
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}
	return operation, nil
}

// fillGuildOperations adds the guild's operation counters to its
// GUILD_EXPANSION_INFO: active_event_cnt the operations started this month
// (Guild:ShouldTipActiveEvent against guildset operation_monthly_time);
// retreat_cnt, which no Lua reads, stays 0.
func (game *Game) fillGuildOperations(ctx context.Context, queries *db.Queries, guild db.Guild, expansion *protobuf.GUILD_EXPANSION_INFO) error {
	started, err := queries.CountGuildOperationsSince(ctx, db.CountGuildOperationsSinceParams{
		GuildID: guild.ID, Since: uint32(startOfMonth(serverNow()).Unix()),
	})
	if err != nil {
		return err
	}
	expansion.ActiveEventCnt = proto.Uint32(uint32(started))
	return nil
}

// guildOperationRequest runs fn on the commander's guild's open operation,
// settled to now, in a transaction: errNotInGuild outside a guild,
// errNoGuildOperation without an open operation.
func (game *Game) guildOperationRequest(client *connection.Client, fn func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error) error {
	ctx := context.Background()
	return game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok {
			return errNotInGuild
		}
		now := serverNow()
		state, ok, err := openGuildOperation(ctx, tx.Queries, member.GuildID, now)
		if err != nil {
			return err
		}
		if !ok {
			return errNoGuildOperation
		}
		if err := state.settle(ctx, tx.Queries, now); err != nil {
			return err
		}
		return fn(tx, member, state, now)
	})
}

// errNoGuildOperation is a request about an operation when the guild has
// none open.
var errNoGuildOperation error = refusal{"no guild operation", resultFailed}

// handleActivateGuildOperation starts an operation (GuildActiveEventCommand,
// CS_61001): an administrator's, the guild at the operation's
// unlock_guild_level, none open, the guild funds paying its consume. The
// operation's tree is drawn from its missions (guildOperationPlans). On
// result 0 the client counts the operation and takes the funds itself, then
// fetches it (CS_61005); the other members online get SC_61021, which makes
// their GuildProxy fetch it again.
func (game *Game) handleActivateGuildOperation(client *connection.Client, req *protobuf.CS_61001) error {
	ctx := context.Background()
	var guildID uint32
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, err := guildAdministratorTx(tx)
		if err != nil {
			return err
		}
		guildID = member.GuildID
		config, known := guildOperations[req.GetChapterId()]
		if !known {
			return errRefused
		}
		guild, err := tx.GetGuild(ctx, member.GuildID)
		if err != nil {
			return err
		}
		if guild.Level < config.UnlockGuildLevel || guild.Capital < config.Consume {
			return errRefused
		}
		now := serverNow()
		if _, open, err := openGuildOperation(ctx, tx.Queries, guild.ID, now); err != nil {
			return err
		} else if open {
			return errRefused
		}
		spent, err := tx.SpendGuildCapital(ctx, db.SpendGuildCapitalParams{Amount: config.Consume, ID: guild.ID})
		if err != nil {
			return err
		}
		if spent == 0 {
			return ErrNotEnough
		}
		commander, err := tx.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if err := tx.addGuildCapitalLog(guild.ID, commander, guildCapitalBattle, config.Consume); err != nil {
			return err
		}
		return createGuildOperation(ctx, tx.Queries, guild.ID, config.ID, now)
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		members, listErr := game.DB.ListGuildMemberIDs(ctx, guildID)
		if listErr != nil {
			return listErr
		}
		defer game.tellGuild(client, members, client.CommanderID, &protobuf.SC_61021{UserId: proto.Uint32(client.CommanderID)})
	}
	return client.Send(&protobuf.SC_61002{Result: result(err)})
}

// createGuildOperation draws an operation's tree: guildOperationPositions
// main missions, common ones at odd positions and elite ones at even ones,
// a side mission at half of the positions after the first, each mission's
// nodes (at its node_position) from the operation's tier of
// guild_event_node, and one of its bosses. The official server's draw isn't
// known.
func createGuildOperation(ctx context.Context, queries *db.Queries, guildID, operationID uint32, now time.Time) error {
	content := guildOperationPlans[operationID]
	bossID := content.bosses[rand.IntN(len(content.bosses))]
	id, err := queries.CreateGuildOperation(ctx, db.CreateGuildOperationParams{
		GuildID: guildID, OperationID: operationID, StartTime: uint32(now.Unix()),
		EndTime: uint32(now.Unix()) + guildOperationLength, BossID: bossID, BossHp: guildBosses[bossID].hp,
	})
	if err != nil {
		return err
	}
	used := map[uint32]bool{}
	draw := func(pools ...[]uint32) uint32 {
		for _, pool := range pools {
			var left []uint32
			for _, event := range pool {
				if !used[event] {
					left = append(left, event)
				}
			}
			if len(left) > 0 {
				event := left[rand.IntN(len(left))]
				used[event] = true
				return event
			}
		}
		return 0
	}
	add := func(eventID, position uint32) error {
		if err := queries.AddGuildMission(ctx, db.AddGuildMissionParams{OperationID: id, EventID: eventID, Position: position}); err != nil {
			return err
		}
		for _, at := range guildBaseEvents[eventID].NodePosition {
			pool := content.battleNodes
			if rand.IntN(2) == 0 && len(content.boxNodes) > 0 || len(pool) == 0 {
				pool = content.boxNodes
			}
			if len(pool) == 0 {
				continue
			}
			if err := queries.AddGuildMissionNode(ctx, db.AddGuildMissionNodeParams{
				OperationID: id, EventID: eventID, Position: at, NodeID: pool[rand.IntN(len(pool))],
			}); err != nil {
				return err
			}
		}
		return nil
	}
	for position := uint32(1); position <= guildOperationPositions; position++ {
		pools := [][]uint32{content.common, content.elite}
		if position%2 == 0 {
			pools = [][]uint32{content.elite, content.common}
		}
		main := draw(pools...)
		if main == 0 {
			break
		}
		if err := add(main, position); err != nil {
			return err
		}
		if position > 1 && rand.IntN(2) == 0 {
			if side := draw(content.side); side != 0 {
				if err := add(side, position); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// handleGuildOperation answers GuildGetActivationEventCommand (CS_61005),
// which GuildProxy sends on entering a guild: the open operation
// (currentOperation), or a non-zero result without one, which the client
// reads as "no operation" (it deactivates its own) without a tip.
func (game *Game) handleGuildOperation(client *connection.Client, req *protobuf.CS_61005) error {
	reply := &protobuf.SC_61006{}
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		var err error
		reply.Operation, err = game.currentOperation(tx.ctx, tx.Queries, state, client.CommanderID, now)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// guildOperationJoins is how many operations the commander joined this
// month without an extra operation, in whatever guild.
func guildOperationJoins(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (uint32, error) {
	user, err := queries.GetGuildUser(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && user.OperationMonth != uint32(startOfMonth(now).Unix()) {
		return 0, nil
	}
	return user.OperationJoins, err
}

// handleJoinGuildOperation takes the commander into the open operation
// (GuildJoinEventCommand, CS_61031): guildset efficiency_param_times joins a
// month, then an extra operation each (GuildEvent:IncreaseJoinCnt). The
// member gains operation_event_guild_active liveness, as the client adds,
// and an operation in the ranks.
func (game *Game) handleJoinGuildOperation(client *connection.Client, req *protobuf.CS_61031) error {
	var notices *guildNotices
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		ctx := tx.ctx
		_, err := tx.GetGuildOperationMember(ctx, db.GetGuildOperationMemberParams{OperationID: state.op.ID, CommanderID: client.CommanderID})
		if err == nil {
			return errRefused
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		joins, err := guildOperationJoins(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		extra := joins >= guildJoinsPerMonth
		if !extra {
			if err := tx.CreateGuildUser(ctx, client.CommanderID); err != nil {
				return err
			}
			if err := tx.SetGuildUserOperationJoins(ctx, db.SetGuildUserOperationJoinsParams{
				OperationMonth: uint32(startOfMonth(now).Unix()), OperationJoins: joins + 1, CommanderID: client.CommanderID,
			}); err != nil {
				return err
			}
		}
		if extra {
			used, err := tx.UseGuildUserExtraOperation(ctx, client.CommanderID)
			if err != nil {
				return err
			}
			if used == 0 {
				return errRefused
			}
		}
		if err := tx.JoinGuildOperation(ctx, db.JoinGuildOperationParams{
			OperationID: state.op.ID, CommanderID: client.CommanderID, JoinedAt: uint32(now.Unix()), Extra: extra,
		}); err != nil {
			return err
		}
		if err := tx.addGuildRankScore(member.GuildID, client.CommanderID, guildRankOperation, 1, now); err != nil {
			return err
		}
		if notices, err = tx.guildNoticesFor(member.GuildID); err != nil {
			return err
		}
		return tx.addGuildLiveness(game, member, guildJoinLiveness, notices)
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.tellGuildNotices(client, notices)
	}
	return client.Send(&protobuf.SC_61032{Result: result(err)})
}

// handleRefreshGuildMission answers GuildRefreshMissionCommand (CS_61023):
// a mission of the open operation as it stands, event_info while it runs,
// completed_info once done (GuildMission.CompleteData2FullData).
func (game *Game) handleRefreshGuildMission(client *connection.Client, req *protobuf.CS_61023) error {
	reply := &protobuf.SC_61024{}
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		mission := state.mission(req.GetEventTid())
		if mission == nil {
			return errRefused
		}
		if mission.FinishedAt != 0 {
			reply.CompletedInfo = &protobuf.EVENT_BASE_COMPLETED{EventId: proto.Uint32(mission.EventID), Position: proto.Uint32(mission.Position)}
			return nil
		}
		reply.EventInfo = state.eventBase(*mission, client.CommanderID, now)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleGuildNodeAnimation records how far the commander watched missions'
// node animations (GuildUpdateNodeAnimFlagCommand, CS_61025), sent back in
// CURRENT_OPERATION.perfs.
func (game *Game) handleGuildNodeAnimation(client *connection.Client, req *protobuf.CS_61025) error {
	err := game.guildOperationRequest(client, func(tx *Tx, member db.GuildMember, state *guildOperationState, now time.Time) error {
		for _, perf := range req.GetPerf() {
			if state.mission(perf.GetEventId()) == nil {
				return errRefused
			}
			if err := tx.SetGuildMissionPerf(tx.ctx, db.SetGuildMissionPerfParams{
				OperationID: state.op.ID, CommanderID: client.CommanderID, EventID: perf.GetEventId(), Idx: perf.GetIndex(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_61026{Result: result(err)})
}

// handleGuildOperation61015 answers CS_61015, which has a proto (type in,
// result out) but that no Lua sends: nothing to do.
//
//belfast:static
func (game *Game) handleGuildOperation61015(client *connection.Client, req *protobuf.CS_61015) error {
	return client.Send(&protobuf.SC_61016{Result: result(nil)})
}

// handleGuildReports answers GuildGetReportsCommand (CS_61017), sent right
// after a guild is entered or created and when missions finish: the guild's
// reports after index (the newest id the client has), the latest
// guildset operation_report_max of them, each with the commander's state
// (locked unless they took part, unlocked, submitted) and, for a mission's,
// its nodes (GuildReport adds their success or failure awards). Outside a
// guild or without reports the list is empty.
func (game *Game) handleGuildReports(client *connection.Client, req *protobuf.CS_61017) error {
	ctx := context.Background()
	reply := &protobuf.SC_61018{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil || !ok {
			return err
		}
		now := serverNow()
		if state, open, err := openGuildOperation(ctx, tx.Queries, member.GuildID, now); err != nil {
			return err
		} else if open {
			if err := state.settle(ctx, tx.Queries, now); err != nil {
				return err
			}
		}
		reports, err := tx.ListGuildReportsAfter(ctx, db.ListGuildReportsAfterParams{
			GuildID: member.GuildID, After: req.GetIndex(), Count: int64(guildReportMax),
		})
		if err != nil {
			return err
		}
		nodes := map[uint32][]db.GuildOperationNode{}
		for _, report := range reports {
			info, err := guildReportInfo(ctx, tx.Queries, report, client.CommanderID, nodes)
			if err != nil {
				return err
			}
			reply.Reports = append(reply.Reports, info)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// guildReportInfo is a report as GuildReport / GuildBossReport read it for
// commanderID; nodes caches the operations' nodes.
func guildReportInfo(ctx context.Context, queries *db.Queries, report db.GuildReport, commanderID uint32, nodes map[uint32][]db.GuildOperationNode) (*protobuf.REPORT, error) {
	status, err := guildReportStatus(ctx, queries, report.ID, commanderID)
	if err != nil {
		return nil, err
	}
	info := &protobuf.REPORT{
		Id:        proto.Uint32(report.ID),
		EventId:   proto.Uint32(report.EventID),
		EventType: proto.Uint32(report.EventType),
		Score:     proto.Uint32(report.Score),
		Status:    proto.Uint32(status),
	}
	if report.EventType != guildReportMission {
		return info, nil
	}
	if _, cached := nodes[report.OperationID]; !cached {
		if nodes[report.OperationID], err = queries.ListGuildMissionNodes(ctx, report.OperationID); err != nil {
			return nil, err
		}
	}
	for _, node := range nodes[report.OperationID] {
		if node.EventID == report.EventID && node.Status != guildNodeAhead {
			info.Nodes = append(info.Nodes, &protobuf.REPORT_NODE{Id: proto.Uint32(node.NodeID), Status: proto.Uint32(node.Status)})
		}
	}
	return info, nil
}

func guildReportStatus(ctx context.Context, queries *db.Queries, reportID, commanderID uint32) (uint32, error) {
	status, err := queries.GetGuildReportClaim(ctx, db.GetGuildReportClaimParams{ReportID: reportID, CommanderID: commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return guildReportLocked, nil
	}
	return status, err
}

// guildReportDrops are a report's rewards (GuildReport:GetDrop): a mission's
// award_list_report on an S grade plus its nodes' success or failure
// awards, a boss's award_report.
func guildReportDrops(ctx context.Context, queries *db.Queries, report db.GuildReport) ([][]uint32, error) {
	if report.EventType == guildReportBoss {
		return guildBosses[report.EventID].awards, nil
	}
	var drops [][]uint32
	event := guildBaseEvents[report.EventID]
	if report.Score == guildScoreS {
		drops = append(drops, event.AwardListReport...)
	}
	nodes, err := queries.ListGuildMissionNodes(ctx, report.OperationID)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if node.EventID != report.EventID {
			continue
		}
		switch node.Status {
		case guildNodeSuccess:
			drops = append(drops, guildEventNodes[node.NodeID].SuccessAward...)
		case guildNodeFailure:
			drops = append(drops, guildEventNodes[node.NodeID].FailAward...)
		}
	}
	return drops, nil
}

// handleSubmitGuildReports claims reports' rewards (SubmitGuildReportCommand,
// CS_61019): every report must be the guild's and unlocked for the
// commander, who must not be a recruit (guild_duty_is_too_low). The client
// adds drop_list (PlayerConst.addTranDrop) and marks the reports submitted.
func (game *Game) handleSubmitGuildReports(client *connection.Client, req *protobuf.CS_61019) error {
	ctx := context.Background()
	reply := &protobuf.SC_61020{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		member, ok, err := guildMembership(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		if !ok || member.Duty == guildDutyRecruit || len(req.GetIds()) == 0 {
			return errRefused
		}
		var drops [][]uint32
		for _, id := range req.GetIds() {
			report, err := tx.GetGuildReport(ctx, db.GetGuildReportParams{ID: id, GuildID: member.GuildID})
			if errors.Is(err, sql.ErrNoRows) {
				return errRefused
			}
			if err != nil {
				return err
			}
			status, err := guildReportStatus(ctx, tx.Queries, id, client.CommanderID)
			if err != nil {
				return err
			}
			if status != guildReportUnlocked {
				return errRefused
			}
			if err := tx.SetGuildReportClaim(ctx, db.SetGuildReportClaimParams{
				ReportID: id, CommanderID: client.CommanderID, Status: guildReportSubmitted,
			}); err != nil {
				return err
			}
			reportDrops, err := guildReportDrops(ctx, tx.Queries, report)
			if err != nil {
				return err
			}
			drops = append(drops, reportDrops...)
		}
		reply.DropList = mergeDrops(configDrops(drops))
		return tx.grantDrops(reply.DropList)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	if err != nil {
		reply.DropList = nil
	}
	return client.Send(reply)
}

// handleGuildReportRank answers GuildGetReportRankCommand (CS_61037): the
// damage the members dealt to a boss report's boss (GuildBossRankPage); a
// mission's report has no ranking.
func (game *Game) handleGuildReportRank(client *connection.Client, req *protobuf.CS_61037) error {
	ctx := context.Background()
	reply := &protobuf.SC_61038{}
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(reply)
	}
	report, err := game.DB.GetGuildReport(ctx, db.GetGuildReportParams{ID: req.GetId(), GuildID: member.GuildID})
	if errors.Is(err, sql.ErrNoRows) || err == nil && report.EventType != guildReportBoss {
		return client.Send(reply)
	}
	if err != nil {
		return err
	}
	if reply.List, err = guildBossRank(ctx, game.DB.Queries, report.OperationID); err != nil {
		return err
	}
	return client.Send(reply)
}

// guildDispatchWindow is when the dispatch window running at now opened:
// the latest of guildset operation_member_dispatch_reset's hours (server
// time) passed (GuildMission:GetNextFormationTime).
func guildDispatchWindow(now time.Time) time.Time {
	day := startOfDay(now)
	for _, start := range []time.Time{day, day.AddDate(0, 0, -1)} {
		for i := len(guildDispatchResets) - 1; i >= 0; i-- {
			if at := start.Add(time.Duration(guildDispatchResets[i]) * time.Hour); !at.After(now) {
				return at
			}
		}
	}
	return day
}
