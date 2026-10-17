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

// handbookLoginPush is SC_22300, CommanderManualProxy's state: the pages the
// commander got tasks from with their pt / claimed pt rewards, and every
// commander manual task they finished (the client checks page and unlock
// tasks against that one list, CommanderManualPage.finishedTaskIds).
func (game *Game) handbookLoginPush(ctx context.Context, commander db.Commander) (*protobuf.SC_22300, error) {
	pages, err := game.DB.ListHandbookPages(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	finished, err := game.DB.ListHandbookFinished(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_22300{}
	for _, page := range pages {
		handbook := &protobuf.TUTHANDBOOK{Id: proto.Uint32(page.PageID), Pt: proto.Uint32(page.Pt), Award: proto.Uint32(page.Award)}
		for _, task := range finished {
			if task.PageID == page.PageID {
				handbook.FinishedTaskIds = append(handbook.FinishedTaskIds, task.TaskID)
			}
		}
		msg.Handbooks = append(msg.Handbooks, handbook)
	}
	for _, task := range finished {
		if !slices.Contains(msg.FinishedTaskIds, task.TaskID) {
			msg.FinishedTaskIds = append(msg.FinishedTaskIds, task.TaskID)
		}
	}
	return msg, nil
}

// handbookTaskClaimed records a claimed task on the commander manual pages
// it belongs to: a page task (task_list[i][0]) finishes and earns the page a
// pt (CommanderManualProxy.ShowTaskSubmitCall) and counts for the phase
// unlock tasks (sub_type 114, target the page); a task that unlocks a page or
// a page's tasks (unlock, task_list[i][1]) only finishes.
func handbookTaskClaimed(ctx context.Context, queries *db.Queries, commanderID uint32, taskID uint32) ([]TaskEvent, error) {
	var events []TaskEvent
	for _, page := range handbookPageConfigs {
		isPageTask, isUnlockTask := false, slices.Contains(page.Unlock, taskID)
		for _, entry := range page.TaskList {
			if len(entry) > 0 && slices.Contains(entry[0], taskID) {
				isPageTask = true
			}
			if len(entry) > 1 && slices.Contains(entry[1], taskID) {
				isUnlockTask = true
			}
		}
		if !isPageTask && !isUnlockTask {
			continue
		}
		if err := queries.CreateHandbookPage(ctx, db.CreateHandbookPageParams{CommanderID: commanderID, PageID: page.ID}); err != nil {
			return nil, err
		}
		added, err := queries.AddHandbookFinished(ctx, db.AddHandbookFinishedParams{CommanderID: commanderID, PageID: page.ID, TaskID: taskID})
		if err != nil {
			return nil, err
		}
		if isPageTask && added > 0 {
			if err := queries.AddHandbookPagePt(ctx, db.AddHandbookPagePtParams{CommanderID: commanderID, PageID: page.ID}); err != nil {
				return nil, err
			}
			events = append(events, TaskEvent{SubType: TaskSubHandbookPhase, Target: page.ID, Count: 1})
		}
	}
	return events, nil
}

// handleHandbookGetTasks gives the tasks of a commander manual page entry
// (CommanderManualCommand GET_TASK: id is the tutorial_handbook_task page,
// index the 1-based task_list entry), pushed as SC_20003 before the reply.
// The capture lists them backwards.
func (game *Game) handleHandbookGetTasks(client *connection.Client, req *protobuf.CS_22302) error {
	ctx := context.Background()
	page, ok := handbookPageConfigs[req.GetId()]
	index := int(req.GetIndex()) - 1
	if !ok || index < 0 || index >= len(page.TaskList) || len(page.TaskList[index]) == 0 {
		return client.Send(&protobuf.SC_22303{Result: result(errRefused)})
	}
	tasks := slices.Clone(page.TaskList[index][0])
	slices.Reverse(tasks)
	var added *protobuf.SC_20003
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		if err := queries.CreateHandbookPage(ctx, db.CreateHandbookPageParams{CommanderID: client.CommanderID, PageID: page.ID}); err != nil {
			return err
		}
		var err error
		added, err = acceptTasks(ctx, queries, client.CommanderID, tasks, uint32(time.Now().Unix()))
		return err
	})
	if err != nil {
		return err
	}
	if added != nil {
		if err := client.Push(added); err != nil {
			return err
		}
	}
	return client.Send(&protobuf.SC_22303{Result: result(nil)})
}
