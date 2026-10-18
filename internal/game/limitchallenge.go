package game

import (
	"context"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// systemLimitChallenge is SYSTEM_LIMIT_CHALLENGE (const.lua): a limit
// challenge battle (BattleGateLimitChallenge), fought by the challenge fleet
// (FleetProxy.CHALLENGE_FLEET_ID). battle_cost_template 14 costs no oil or
// morale and gives no exp, and its stages drop nothing (award_display empty).
const systemLimitChallenge = 14

// constellation_challenge_month row: stage lists the limit challenges of a
// month, by level
type limitChallengeMonthConfig struct {
	Stage []uint32 `json:"stage"`
}

// expedition_constellation_challenge_template row: dungeon_id is the
// expedition fought (LimitChallengeConst.GetChallengeIDByStageID), award_display
// the reward of a first clear (LimitChallengeScene.updateAward).
type limitChallengeConfig struct {
	DungeonID    uint32     `json:"dungeon_id"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

var (
	limitChallengeMonths map[uint32]limitChallengeMonthConfig
	limitChallenges      map[uint32]limitChallengeConfig
	// limitChallengeStages maps an expedition to its challenge
	limitChallengeStages map[uint32]uint32
)

func loadLimitChallengeData() error {
	err := gamedata.LoadAll(
		gamedata.Load("constellation_challenge_month", &limitChallengeMonths),
		gamedata.Load("expedition_constellation_challenge_template", &limitChallenges),
	)
	if err != nil {
		return err
	}
	limitChallengeStages = make(map[uint32]uint32, len(limitChallenges))
	for id, row := range limitChallenges {
		limitChallengeStages[row.DungeonID] = id
	}
	return nil
}

// limitChallengeMonth is the YYYYMM of now, server time: the month whose
// challenges are open (LimitChallengeConst.GetCurMonth).
func limitChallengeMonth() uint32 {
	now := serverNow()
	return uint32(now.Year()*100 + int(now.Month()))
}

// handleLimitChallenge answers LimitChallengeReqCommand: times is the best
// clear time of every challenge cleared (seconds), awards the challenges
// whose reward was taken (value 1), pass_ids the challenges cleared this
// month. A level opens once the previous one's reward is taken
// (LimitChallengeProxy.isLevelUnlock).
func (game *Game) handleLimitChallenge(client *connection.Client, req *protobuf.CS_24020) error {
	rows, err := game.DB.ListLimitChallenges(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_24021{Result: result(nil)}
	month := limitChallengeMonth()
	for _, row := range rows {
		reply.Times = append(reply.Times, &protobuf.KVDATA{Key: proto.Uint32(row.ChallengeID), Value: proto.Uint32(row.BestTime)})
		if row.Awarded {
			reply.Awards = append(reply.Awards, &protobuf.KVDATA{Key: proto.Uint32(row.ChallengeID), Value: proto.Uint32(1)})
		}
		if row.PassMonth == month {
			reply.PassIds = append(reply.PassIds, row.ChallengeID)
		}
	}
	return client.Send(reply)
}

// handleLimitChallengeAward answers LimitChallengeGetAwardCommand (CS_24022),
// sent by LimitChallengeMediator on opening the challenges with those cleared
// this month and not awarded yet: each gives its award_display (an icon or
// chat frame), drop_list is added by the client (PlayerConst.addTranDrop).
// Refused (errorTip) when one isn't cleared this month or was already
// awarded. Built from the Lua alone (never captured).
func (game *Game) handleLimitChallengeAward(client *connection.Client, req *protobuf.CS_24022) error {
	ctx := context.Background()
	reply := &protobuf.SC_24023{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		rows, err := tx.ListLimitChallenges(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		month := limitChallengeMonth()
		for i, id := range req.GetChallengeids() {
			index := slices.IndexFunc(rows, func(row db.LimitChallenge) bool { return row.ChallengeID == id })
			if index < 0 || rows[index].PassMonth != month || rows[index].Awarded || slices.Contains(req.GetChallengeids()[:i], id) {
				return errRefused
			}
			if err := tx.AwardLimitChallenge(ctx, db.AwardLimitChallengeParams{CommanderID: client.CommanderID, ChallengeID: id}); err != nil {
				return err
			}
			reply.DropList = append(reply.DropList, configDrops(limitChallenges[id].AwardDisplay)...)
		}
		return tx.grantDrops(reply.DropList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_24023{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// openLimitChallenge is the challenge fought at expedition stage, if it is
// one of this month's and its level is open (the previous level awarded).
func openLimitChallenge(ctx context.Context, queries *db.Queries, commanderID, stage uint32) (uint32, bool, error) {
	id, known := limitChallengeStages[stage]
	levels := limitChallengeMonths[uint32(serverNow().Month())].Stage
	level := slices.Index(levels, id)
	if !known || level < 0 {
		return 0, false, nil
	}
	if level == 0 {
		return id, true, nil
	}
	rows, err := queries.ListLimitChallenges(ctx, commanderID)
	if err != nil {
		return 0, false, err
	}
	previous := slices.IndexFunc(rows, func(row db.LimitChallenge) bool { return row.ChallengeID == levels[level-1] })
	return id, previous >= 0 && rows[previous].Awarded, nil
}

// beginLimitChallenge starts a limit challenge battle
// (BattleGateLimitChallenge.Entrance): the stage must be an open challenge
// of this month and the fleet the commander's ships. Nothing is spent.
func (game *Game) beginLimitChallenge(client *connection.Client, req *protobuf.CS_40001) error {
	ctx := context.Background()
	_, open, err := openLimitChallenge(ctx, game.DB.Queries, client.CommanderID, req.GetData())
	if err != nil {
		return err
	}
	owned, err := ownsShips(ctx, game.DB.Queries, client.CommanderID, req.GetShipIdList())
	if err != nil {
		return err
	}
	if !open || !owned {
		return client.Send(&protobuf.SC_40002{Result: result(errRefused)})
	}
	return client.Send(&protobuf.SC_40002{Result: result(nil), Key: proto.Uint32(rand.Uint32N(10000))})
}

// finishLimitChallenge settles a limit challenge battle
// (BattleGateLimitChallenge.Exit): a win (score above C) clears the
// challenge this month and keeps total_time if it is its best
// (LimitChallengeProxy.setPassTime, which the client does itself); its reward
// is taken with CS_24022. No exp, drops or morale (battle_cost_template 14).
// Built from the Lua alone (never captured).
func (game *Game) finishLimitChallenge(client *connection.Client, req *protobuf.CS_40003) error {
	ctx := context.Background()
	id, open, err := openLimitChallenge(ctx, game.DB.Queries, client.CommanderID, req.GetData())
	if err != nil {
		return err
	}
	if !open {
		return client.Send(&protobuf.SC_40004{Result: result(errRefused)})
	}
	statShips := make([]uint32, len(req.GetStatistics()))
	for i, stat := range req.GetStatistics() {
		statShips[i] = stat.GetShipId()
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, statShips)
	if err != nil {
		return err
	}
	if req.GetScore() > scoreC {
		err := game.DB.PassLimitChallenge(ctx, db.PassLimitChallengeParams{
			CommanderID: client.CommanderID,
			ChallengeID: id,
			BestTime:    req.GetTotalTime(),
			PassMonth:   limitChallengeMonth(),
		})
		if err != nil {
			return err
		}
	}
	return client.Send(&protobuf.SC_40004{Result: result(nil), Mvp: proto.Uint32(battleMVP(req.GetStatistics(), byID))})
}
