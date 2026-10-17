package game

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleSubmitWeeklyTask claims a finished weekly mission
// (SubmitWeekTaskCommand): its points, and the next step of its chain, which
// the client adds when next.id isn't 0.
func (game *Game) handleSubmitWeeklyTask(client *connection.Client, req *protobuf.CS_20106) error {
	var next []*protobuf.WEEKLY_TASK_P20
	err := game.tx(context.Background(), client, func(tx *Tx) (err error) {
		_, next, err = tx.claimWeeklyTasks([]uint32{req.GetId()})
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	reply := &protobuf.SC_20107{Result: result(err)}
	if err == nil && len(next) > 0 {
		reply.Next = next[0]
	}
	return client.Send(reply)
}

// handleSubmitWeeklyTasks claims several finished weekly missions at once
// (BatchSubmitWeekTaskCommand): pt is the points they give in total, next the
// next steps of their chains. The client drops all the ids it sent on
// success, so one that can't be claimed refuses them all.
func (game *Game) handleSubmitWeeklyTasks(client *connection.Client, req *protobuf.CS_20108) error {
	var pt uint32
	var next []*protobuf.WEEKLY_TASK_P20
	err := game.tx(context.Background(), client, func(tx *Tx) (err error) {
		pt, next, err = tx.claimWeeklyTasks(req.GetId())
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		pt, next = 0, nil
	}
	return client.Send(&protobuf.SC_20109{Result: result(err), Pt: proto.Uint32(pt), Next: next})
}

// claimWeeklyTasks claims finished weekly missions: they stay stored as
// claimed, their points (weekly_task_template.award_display, the virtual
// item 59012) are given, and the next step of each chain (ids by tens, the
// next id) starts from the claimed one's progress, weekly missions counting
// on past their target. It returns the points given and the next steps.
func (tx *Tx) claimWeeklyTasks(ids []uint32) (uint32, []*protobuf.WEEKLY_TASK_P20, error) {
	if len(ids) == 0 {
		return 0, nil, errRefused
	}
	var pt uint32
	var next []*protobuf.WEEKLY_TASK_P20
	for i, id := range ids {
		config, ok := weeklyTaskConfigs[id]
		if !ok || slices.Contains(ids[:i], id) {
			return 0, nil, errRefused
		}
		task, err := tx.GetWeeklyTask(tx.ctx, db.GetWeeklyTaskParams{CommanderID: tx.commanderID, TaskID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil, errRefused
		}
		if err != nil {
			return 0, nil, err
		}
		if task.Claimed || task.Progress < config.TargetNum {
			return 0, nil, errRefused
		}
		if _, err := tx.ClaimWeeklyTask(tx.ctx, db.ClaimWeeklyTaskParams{CommanderID: tx.commanderID, TaskID: id}); err != nil {
			return 0, nil, err
		}
		if err := tx.grantDrops(configDrops([][]uint32{config.AwardDisplay})); err != nil {
			return 0, nil, err
		}
		pt += config.AwardDisplay[2]
		nextID := id + 1
		if _, ok := weeklyTaskConfigs[nextID]; !ok || nextID/10 != id/10 {
			continue
		}
		if err := tx.CreateWeeklyTask(tx.ctx, db.CreateWeeklyTaskParams{CommanderID: tx.commanderID, TaskID: nextID, Progress: task.Progress}); err != nil {
			return 0, nil, err
		}
		next = append(next, &protobuf.WEEKLY_TASK_P20{Id: proto.Uint32(nextID), Progress: proto.Uint32(task.Progress)})
	}
	return pt, next, nil
}

// handleSubmitWeeklyProgress claims the next phase of the weekly reward bar
// (SubmitWeekTaskProgressCommand, id 0) once the commander has its points.
// SC_20101.reward_lv is the weekly_target value of the last phase claimed,
// not a phase number: WeekTaskProgress.Init takes the phase from
// table.indexof(targets, reward_lv), 0 when it isn't one.
func (game *Game) handleSubmitWeeklyProgress(client *connection.Client, req *protobuf.CS_20110) error {
	var awards []*protobuf.DROPINFO
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		rewardLevel, err := tx.GetWeeklyRewardLevel(tx.ctx, tx.commanderID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		phase := slices.Index(weeklyTargets, rewardLevel) + 1 // phases claimed
		if phase >= len(weeklyTargets) || phase >= len(weeklyPhaseDrops) {
			return errRefused
		}
		pt, err := tx.GetVirtualItem(tx.ctx, db.GetVirtualItemParams{CommanderID: tx.commanderID, ItemID: VirtualItemWeeklyPt})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if pt < weeklyTargets[phase] {
			return errRefused
		}
		awards = configDrops(weeklyPhaseDrops[phase])
		if err := tx.grantDrops(awards); err != nil {
			return err
		}
		return tx.SetWeeklyRewardLevel(tx.ctx, db.SetWeeklyRewardLevelParams{CommanderID: tx.commanderID, RewardLv: weeklyTargets[phase]})
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		awards = nil
	}
	return client.Send(&protobuf.SC_20111{Result: result(err), AwardList: awards})
}
