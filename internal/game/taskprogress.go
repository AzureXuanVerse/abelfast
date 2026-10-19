package game

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// TASK_UPDATE / ACT_TASK_UPDATE mode (Task.TASK_PROGRESS_*,
// model/vo/task/task.lua)
const (
	taskProgressSet    = 0 // progress is the new value
	taskProgressAppend = 1 // progress is added
)

// clientTaskSubType tells whether the client counts the sub type's progress
// itself and reports it (Task.isClientTrigger: 2001-2999, fleets formed,
// commander manual pages visited, secretary or backyard interactions...).
func clientTaskSubType(subType uint32) bool {
	return subType > 2000 && subType < 3000
}

// taskUpdate is the progress a TASK_UPDATE / ACT_TASK_UPDATE gives a task at
// current, false when the mode isn't one.
func taskUpdate(mode, progress, current uint32) (uint32, bool) {
	switch mode {
	case taskProgressSet:
		return progress, true
	case taskProgressAppend:
		return current + progress, true
	}
	return 0, false
}

// handleUpdateTaskProgress takes the progress the client checked itself on
// its tasks (UpdateTaskProgressCommand, controller/command/task: fleets of
// the right size or level, the commander manual, secretary and backyard
// interactions, mini games). The client updates the task itself on result
// 0, nothing is pushed. TaskProxy tasks stop at their target, as for the
// progress the server counts. Never captured: built from the Lua alone.
func (game *Game) handleUpdateTaskProgress(client *connection.Client, req *protobuf.CS_20009) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		for _, info := range req.GetProgressinfo() {
			config, ok := taskConfigs[info.GetId()]
			if !ok || !clientTaskSubType(config.SubType) {
				return errRefused
			}
			task, err := queries.GetTask(ctx, db.GetTaskParams{CommanderID: client.CommanderID, TaskID: info.GetId()})
			if errors.Is(err, sql.ErrNoRows) {
				return errRefused
			}
			if err != nil {
				return err
			}
			progress, ok := taskUpdate(info.GetMode(), info.GetProgress(), task.Progress)
			if !ok || task.SubmitTime != 0 {
				return errRefused
			}
			if err := queries.SetTaskProgress(ctx, db.SetTaskProgressParams{Progress: min(progress, config.TargetNum), CommanderID: client.CommanderID, TaskID: task.TaskID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		logger.WithFields("Task", logger.FieldValue("update", req.GetProgressinfo())).Warn("progress refused")
	}
	return client.Send(&protobuf.SC_20010{Result: result(err)})
}

// handleUpdateActivityTaskProgress is handleUpdateTaskProgress for the
// tasks of a running task activity TaskProxy holds
// (UpdateLowPriorityTaskProgressCommand, the "roll" task of an EX/SP event
// map, LevelMediator2.ON_UPDATE_LOWPRIORITY_TASK). Activity tasks keep
// counting past their target, as for the progress the server counts.
func (game *Game) handleUpdateActivityTaskProgress(client *connection.Client, req *protobuf.CS_20209) error {
	ctx := context.Background()
	now := time.Now()
	var updates []db.SetActivityTaskProgressParams
	err := func() error {
		for _, info := range req.GetProgressinfo() {
			config, ok := taskConfigs[info.GetTaskId()]
			activity, isActivity := activityConfigs[info.GetActId()]
			if !ok || !isActivity || !clientTaskSubType(config.SubType) || !normalTaskActivityTypes[activity.Type] {
				return errRefused
			}
			task, ok, err := game.openActivityTask(ctx, client.CommanderID, info.GetActId(), info.GetTaskId(), now)
			if err != nil {
				return err
			}
			progress, valid := taskUpdate(info.GetMode(), info.GetProgress(), task.progress)
			if !ok || !valid || task.finished {
				return errRefused
			}
			updates = append(updates, db.SetActivityTaskProgressParams{CommanderID: client.CommanderID, ActID: info.GetActId(), TaskID: info.GetTaskId(), Progress: progress})
		}
		return game.DB.InTx(ctx, func(queries *db.Queries) error {
			for _, update := range updates {
				if err := queries.SetActivityTaskProgress(ctx, update); err != nil {
					return err
				}
			}
			return nil
		})
	}()
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		logger.WithFields("Task", logger.FieldValue("update", req.GetProgressinfo())).Warn("progress refused")
	}
	return client.Send(&protobuf.SC_20210{Result: result(err)})
}
