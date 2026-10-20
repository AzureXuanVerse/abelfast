package game

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The returning player campaign (Akashi's Homecoming Campaign on the wiki,
// RefluxProxy, RefluxScene): a commander of level 10 or more coming back
// after a long time away gets a daily sign-in (return_sign_template), tasks
// unlocking with the days signed (return_task_template, task_data_template
// type 15) whose points (pt_award) claim milestone rewards
// (return_pt_template). Every reward depends on the level the commander came
// back at (the level brackets of each row).
//
// Who returns is the server's call: gameset return_player_set, which no Lua
// reads, holds {10, 30, 90, 14}. The wiki gives the level (10) and the 14
// days of sign-in; belfast reads the rest as 30 days offline and 90 days
// between two campaigns of a commander.

// return_player_set entries
type refluxRules struct {
	minLevel     uint32
	offlineDays  uint32
	cooldownDays uint32
	days         uint32 // the campaign's length
}

// level brackets of the return_* rows: {min, max} commander levels, the
// award lists are in the same order
type refluxLevels [][2]uint32

// bracket is the index of the commander's level among levels
// (RefluxSignView:getLevelIndex), -1 when none.
func (levels refluxLevels) bracket(level uint32) int {
	found := -1
	for i, span := range levels {
		if span[0] <= level && level <= span[1] {
			found = i
		}
	}
	return found
}

// return_sign_template row: the drops of one day of the sign-in
type refluxSignConfig struct {
	ID           uint32        `json:"id"`
	Level        refluxLevels  `json:"level"`
	AwardDisplay [][][3]uint32 `json:"award_display"`
}

// return_pt_template row: the drop of one milestone and the points it needs
type refluxPtConfig struct {
	ID           uint32       `json:"id"`
	Level        refluxLevels `json:"level"`
	PtRequire    uint32       `json:"pt_require"`
	AwardDisplay [][3]uint32  `json:"award_display"`
}

// return_task_template row: a campaign task's drop, the points it gives and
// the days to sign in before it can be claimed (RefluxTaskView)
type refluxTaskConfig struct {
	Level        refluxLevels `json:"level"`
	AwardDisplay [][3]uint32  `json:"award_display"`
	PtAward      uint32       `json:"pt_award"`
	RewardDate   uint32       `json:"reward_date"`
}

var (
	refluxRule  refluxRules
	refluxSigns []refluxSignConfig // by id, day 1 first
	refluxPts   []refluxPtConfig   // by id, the first milestone first
	refluxTasks map[uint32]refluxTaskConfig
)

func loadRefluxData() error {
	var signs map[uint32]refluxSignConfig
	var pts map[uint32]refluxPtConfig
	var rules []uint32
	err := gamedata.LoadAll(
		gamedata.Load("return_sign_template", &signs),
		gamedata.Load("return_pt_template", &pts),
		gamedata.Load("return_task_template", &refluxTasks),
		func() error { return gamedata.GamesetDescription("return_player_set", &rules) },
	)
	if err != nil {
		return err
	}
	if len(rules) < 4 {
		return gamedata.Missing("gameset", "return_player_set {level, offline days, cooldown days, days}")
	}
	refluxRule = refluxRules{minLevel: rules[0], offlineDays: rules[1], cooldownDays: rules[2], days: rules[3]}
	refluxSigns = slices.SortedFunc(maps.Values(signs), func(a, b refluxSignConfig) int { return cmp.Compare(a.ID, b.ID) })
	refluxPts = slices.SortedFunc(maps.Values(pts), func(a, b refluxPtConfig) int { return cmp.Compare(a.ID, b.ID) })
	return nil
}

// refluxTaskIDs are the campaign's tasks.
func refluxTaskIDs() []uint32 {
	return slices.Sorted(maps.Keys(refluxTasks))
}

// refluxActive is whether the commander's campaign runs at now.
func refluxActive(row db.CommanderReflux, now time.Time) bool {
	return row.ReturnTime > 0 && now.Before(time.Unix(row.ReturnTime, 0).Add(time.Duration(refluxRule.days)*day))
}

// commanderReflux is the commander's campaign, ok when it runs at now.
func commanderReflux(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (db.CommanderReflux, bool, error) {
	row, err := queries.GetReflux(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	return row, err == nil && refluxActive(row, now), err
}

// startRefluxIfReturning starts the campaign of a commander logging in
// (before the login burst, which then carries its tasks and SC_11752):
// level return_player_set[1] or more, offline for return_player_set[2]
// days (last_online, set when the previous session ended) and no campaign
// started in the last return_player_set[3] days. The campaign starts over:
// its tasks are handed out again from zero.
func (game *Game) startRefluxIfReturning(ctx context.Context, commander db.Commander, ships int, now time.Time) error {
	if commander.Level < refluxRule.minLevel || commander.LastOnline == 0 {
		return nil
	}
	if now.Sub(time.Unix(commander.LastOnline, 0)) < time.Duration(refluxRule.offlineDays)*day {
		return nil
	}
	return game.DB.InTx(ctx, func(queries *db.Queries) error {
		previous, err := queries.GetReflux(ctx, commander.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && now.Sub(time.Unix(previous.ReturnTime, 0)) < time.Duration(refluxRule.cooldownDays)*day {
			return nil
		}
		if err := queries.StartReflux(ctx, db.StartRefluxParams{
			CommanderID:     commander.ID,
			ReturnLv:        commander.Level,
			ReturnTime:      now.Unix(),
			ShipNumber:      uint32(ships),
			LastOfflineTime: commander.LastOnline,
		}); err != nil {
			return err
		}
		tasks := refluxTaskIDs()
		if err := queries.DeleteTasks(ctx, db.DeleteTasksParams{CommanderID: commander.ID, TaskIds: tasks}); err != nil {
			return err
		}
		_, err = acceptTasks(ctx, queries, commander.ID, tasks, uint32(now.Unix()))
		return err
	})
}

// refluxInfo is SC_11752: active 0 when the commander's campaign doesn't
// run (as the capture has it for a commander that never left).
func refluxInfo(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.SC_11752, error) {
	row, active, err := commanderReflux(ctx, queries, commanderID, now)
	if err != nil || !active {
		return &protobuf.SC_11752{Active: proto.Uint32(0)}, err
	}
	return &protobuf.SC_11752{
		Active:          proto.Uint32(1),
		ReturnLv:        proto.Uint32(row.ReturnLv),
		ReturnTime:      proto.Uint32(uint32(row.ReturnTime)),
		ShipNumber:      proto.Uint32(row.ShipNumber),
		LastOfflineTime: proto.Uint32(uint32(row.LastOfflineTime)),
		Pt:              proto.Uint32(row.Pt),
		SignCnt:         proto.Uint32(row.SignCnt),
		SignLastTime:    proto.Uint32(uint32(row.SignLastTime)),
		PtStage:         proto.Uint32(row.PtStage),
	}, nil
}

// handleRefluxData answers RefluxRequestDataCommand (CS_11751, also sent at
// every server day change) with the campaign (SC_11752).
func (game *Game) handleRefluxData(client *connection.Client, req *protobuf.CS_11751) error {
	info, err := refluxInfo(context.Background(), game.DB.Queries, client.CommanderID, time.Now())
	if err != nil {
		return err
	}
	return client.Send(info)
}

// handleRefluxSign answers RefluxSignCommand (CS_11753): the next day of
// the sign-in, once per server day (RefluxProxy:isCanSign), with that day's
// return_sign_template drops for the level the commander came back at.
func (game *Game) handleRefluxSign(client *connection.Client, req *protobuf.CS_11753) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_11754{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		row, active, err := commanderReflux(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if !active || int(row.SignCnt) >= len(refluxSigns) || (row.SignLastTime != 0 && sameServerDay(time.Unix(row.SignLastTime, 0), now)) {
			return errRefused
		}
		sign := refluxSigns[row.SignCnt]
		bracket := sign.Level.bracket(row.ReturnLv)
		if bracket < 0 || bracket >= len(sign.AwardDisplay) {
			return errRefused
		}
		signed, err := tx.SignReflux(ctx, db.SignRefluxParams{Now: now.Unix(), CommanderID: client.CommanderID, SignCnt: row.SignCnt})
		if err != nil {
			return err
		}
		if signed == 0 {
			return errRefused
		}
		for _, drop := range sign.AwardDisplay[bracket] {
			reply.AwardList = append(reply.AwardList, configDrops([][]uint32{drop[:]})...)
		}
		return tx.grantDrops(reply.AwardList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_11754{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleRefluxPtAward answers RefluxGetPTAwardCommand (CS_11755): the next
// milestone (pt_stage + 1) once the campaign's points reach its
// pt_require (RefluxPtView:isAnyPTCanGetAward), its drop for the level the
// commander came back at.
func (game *Game) handleRefluxPtAward(client *connection.Client, req *protobuf.CS_11755) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_11756{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		row, active, err := commanderReflux(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if !active || int(row.PtStage) >= len(refluxPts) {
			return errRefused
		}
		milestone := refluxPts[row.PtStage]
		bracket := milestone.Level.bracket(row.ReturnLv)
		if row.Pt < milestone.PtRequire || bracket < 0 || bracket >= len(milestone.AwardDisplay) {
			return errRefused
		}
		claimed, err := tx.ClaimRefluxPtStage(ctx, db.ClaimRefluxPtStageParams{CommanderID: client.CommanderID, PtStage: row.PtStage})
		if err != nil {
			return err
		}
		if claimed == 0 {
			return errRefused
		}
		reply.AwardList = configDrops([][]uint32{milestone.AwardDisplay[bracket][:]})
		return tx.grantDrops(reply.AwardList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_11756{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// refluxTaskClaim is what claiming a campaign task gives: its
// return_task_template drop for the level the commander came back at (its
// task_data_template award_display is empty), ok false while the campaign
// doesn't run or the task is still locked (fewer days signed than its
// reward_date). It isn't a campaign task when isTask is false.
func refluxTaskClaim(ctx context.Context, queries *db.Queries, commanderID, taskID uint32, now time.Time) (drops [][]uint32, isTask, ok bool, err error) {
	config, isTask := refluxTasks[taskID]
	if !isTask {
		return nil, false, false, nil
	}
	row, active, err := commanderReflux(ctx, queries, commanderID, now)
	if err != nil || !active || row.SignCnt < config.RewardDate {
		return nil, true, false, err
	}
	bracket := config.Level.bracket(row.ReturnLv)
	if bracket < 0 || bracket >= len(config.AwardDisplay) {
		return nil, true, false, nil
	}
	return [][]uint32{config.AwardDisplay[bracket][:]}, true, true, nil
}

// refluxTaskClaimed adds a claimed campaign task's points to the campaign
// (the client does the same, RefluxProxy:addPtAfterSubTasks).
func (tx *Tx) refluxTaskClaimed(taskID uint32) error {
	config, ok := refluxTasks[taskID]
	if !ok || config.PtAward == 0 {
		return nil
	}
	return tx.AddRefluxPt(tx.ctx, db.AddRefluxPtParams{Pt: config.PtAward, CommanderID: tx.commanderID})
}

// refluxClaimedTasks are the campaign's tasks to list as claimed in SC_20001
// while it runs (RefluxTaskView reads their state from TaskProxy:getTaskVO).
func refluxClaimedTasks(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (map[uint32]bool, error) {
	_, active, err := commanderReflux(ctx, queries, commanderID, now)
	if err != nil || !active {
		return nil, err
	}
	tasks := make(map[uint32]bool, len(refluxTasks))
	for id := range refluxTasks {
		tasks[id] = true
	}
	return tasks, nil
}
