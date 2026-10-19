package game

import (
	"context"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// itemQuickTaskPass is Item.QUICK_TASK_PASS_TICKET_ID (model/vo/item.lua),
// the cruise mission voucher a quick finish costs.
const itemQuickTaskPass = 15013

// Quick finishes (QuickTaskCommand, controller/command/task): the cruise
// pass pages (CrusingTaskMediator, WorldCruiseMediator, PssHei5Mediator)
// finish a task at once for task_data_template.quick_finish vouchers,
// whatever its progress. The client checked it has them and takes them off
// its bag itself on success, then drops the task as for a claim
// (SubmitTaskCommand.OnSubmitSuccess) and shows award_list. Never captured:
// the replies are built from the Lua alone.

// handleQuickTask quick-finishes one of the commander's tasks (CS_20013,
// the tasks that aren't an activity's): a claim paid with vouchers instead
// of progress.
func (game *Game) handleQuickTask(client *connection.Client, req *protobuf.CS_20013) error {
	ctx := context.Background()
	claim, _, err := game.taskClaim(ctx, client.CommanderID, req.GetId())
	if err != nil {
		return err
	}
	cost := claim.config.QuickFinish
	if claim.task.TaskID == 0 || claim.task.SubmitTime != 0 || cost == 0 || req.GetItemCost() != cost {
		logger.WithFields("Task", logger.FieldValue("id", req.GetId()), logger.FieldValue("cost", req.GetItemCost())).Warn("quick finish refused")
		return client.Send(&protobuf.SC_20014{Result: result(errRefused)})
	}
	awards, err := game.claimTasks(ctx, client, []taskClaimed{claim}, func(tx *Tx) error {
		return tx.SpendItem(itemQuickTaskPass, cost)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_20014{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_20014{Result: result(nil), AwardList: awards})
}

// handleQuickActivityTask quick-finishes a task of a running task activity
// TaskProxy holds (CS_20207, TotalTaskProxy.normal_task_type: the cruise
// pass): it is claimed as SubmitActiveTaskCommand claims it, finished at its
// target, and its removal pushed (SC_20204) before the reply as for a claim.
func (game *Game) handleQuickActivityTask(client *connection.Client, req *protobuf.CS_20207) error {
	ctx := context.Background()
	task, ok, err := game.openActivityTask(ctx, client.CommanderID, req.GetActId(), req.GetTaskId(), time.Now())
	if err != nil {
		return err
	}
	config, known := taskConfigs[req.GetTaskId()]
	activity, isActivity := activityConfigs[req.GetActId()]
	if !ok || !known || !isActivity || !normalTaskActivityTypes[activity.Type] || task.finished || config.QuickFinish == 0 || req.GetItemCost() != config.QuickFinish {
		logger.WithFields("Task", logger.FieldValue("activity", req.GetActId()), logger.FieldValue("id", req.GetTaskId()), logger.FieldValue("cost", req.GetItemCost())).Warn("quick finish refused")
		return client.Send(&protobuf.SC_20208{Result: result(errRefused)})
	}
	task.progress = max(task.progress, config.TargetNum)
	reply := &protobuf.SC_20208{Result: result(nil)}
	var removed *protobuf.ACT_TASK_LIST
	err = game.tx(ctx, client, func(tx *Tx) (err error) {
		if err := tx.SpendItem(itemQuickTaskPass, config.QuickFinish); err != nil {
			return err
		}
		reply.AwardList, removed, err = tx.finishActivityTasks(req.GetActId(), []activityTask{task})
		return err
	})
	if refused(err) {
		return client.Send(&protobuf.SC_20208{Result: result(err)})
	}
	if err != nil {
		return err
	}
	if err := client.Push(&protobuf.SC_20204{Info: []*protobuf.ACT_TASK_LIST{removed}}); err != nil {
		return err
	}
	return client.Send(reply)
}
