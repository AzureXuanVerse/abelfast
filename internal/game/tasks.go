package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// task_data_template / weekly_task_template sub_type: what a task counts.
// Requests record them on their Tx when they happen.
const (
	TaskSubDefeatEnemy    = 11   // enemy ships sunk in a battle
	TaskSubWin            = 20   // battles won
	TaskSubDefeatBoss     = 21   // boss fleets defeated
	TaskSubWinA           = 23   // battles won with A rank or better
	TaskSubWinS           = 24   // battles won with S rank
	TaskSubDailyChallenge = 26   // daily challenge cleared, target: its stage id
	TaskSubExercise       = 27   // exercises fought
	TaskSubBuild          = 30   // ships built
	TaskSubRetire         = 31   // ships retired
	TaskSubEnhanceShip    = 34   // ship enhancements
	TaskSubEnhanceGear    = 40   // gear upgrades
	TaskSubCraftGear      = 42   // gear crafted
	TaskSubAcademyLevel   = 85   // canteen / merchant level (absolute)
	TaskSubMonthSign      = 106  // monthly login rewards collected
	TaskSubHandbookPhase  = 114  // commander manual tasks claimed, target: page id
	TaskSubUseItem        = 120  // items used, target: item id
	TaskSubSpendOil       = 121  // oil spent
	TaskSubObtainItem     = 130  // items obtained, target: item id
	TaskSubWeeklyPt       = 161  // weekly mission points obtained
	TaskSubLevel          = 1011 // commander level (absolute)
	TaskSubClearStage     = 1020 // stage cleared, target: chapter id
	TaskSubCruisePt       = 1023 // cruise pass points (absolute)
	TaskSubStars          = 1026 // main campaign stars (absolute)
	TaskSubSortieExp      = 1041 // ship EXP earned from sorties
)

// Sub types whose event carries the current value (a level, a total) rather
// than an amount to add.
var taskAbsoluteSubTypes = map[uint32]bool{
	TaskSubAcademyLevel: true,
	TaskSubLevel:        true,
	TaskSubCruisePt:     true,
	TaskSubStars:        true,
}

// Sub types a newly accepted task starts from what the commander already did.
// In the capture, tasks accepted mid-tutorial came with the commander level,
// stages cleared, stars and academy level, and with the build count unless
// they are auto_commit (the manual's "Build 1 ship" started at 0 when "Build
// 10 times in total" started at 1), while gear crafts / upgrades, wins and
// kills started back at 0.
var taskInheritedSubTypes = map[uint32]bool{
	TaskSubAcademyLevel: true,
	TaskSubLevel:        true,
	TaskSubClearStage:   true,
	TaskSubCruisePt:     true,
	TaskSubStars:        true,
}

var taskInheritedCountSubTypes = map[uint32]bool{
	TaskSubBuild: true,
}

// item_virtual_data_statistics ids earned from tasks (drop type 8)
const (
	VirtualItemWeeklyPt = 59012 // weekly mission points (WeekTaskProgress)
	VirtualItemCruisePt = 59599 // cruise pass points (SubmitActiveTaskCommand)
)

// activity_template types whose tasks come in SC_20201 (TotalTaskProxy):
// ACTIVITY_TYPE_TASKS 13, PT_CRUSING 54, PT_HEI5 130, BOSS_RUSH_DAL_COLLAB 132
// go to TaskProxy; TASK_RYZA 89, HOTSPRING_2 100, NOT_TRACEABLE 121 to
// ActivityTaskProxy.
const activityTypeCruise = 54

var taskActivityTypes = map[uint32]bool{13: true, 54: true, 130: true, 132: true, 89: true, 100: true, 121: true}

// TaskEvent is something that happened that tasks count.
type TaskEvent struct {
	SubType uint32 // task_data_template.sub_type, see TaskSub*
	Target  uint32 // chapter id, item id...; 0 when the event has none
	Count   uint32 // amount to add, or the new value for absolute sub types
}

func (game *Game) registerTasks() {
	packets.Handle(game.handleSubmitTask)
	packets.Handle(game.handleClientTaskEvent)
	packets.Handle(game.handleSubmitActivityTasks)
	packets.Handle(game.handleHandbookGetTasks)
	game.registerJuus()
}

// taskList is SC_20001, TaskProxy's tasks.
func taskList(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_20001, error) {
	tasks, err := queries.ListTasks(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_20001{Info: make([]*protobuf.TASKINFO, len(tasks))}
	for i, task := range tasks {
		msg.Info[i] = &protobuf.TASKINFO{
			Id:         proto.Uint32(task.TaskID),
			Progress:   proto.Uint32(task.Progress),
			AcceptTime: proto.Uint32(task.AcceptTime),
		}
	}
	return msg, nil
}

// activityTasksList is SC_20201, the tasks of the open task activities
// (TotalTaskProxy).
func (game *Game) activityTasksList(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) *protobuf.SC_20201 {
	activities, err := game.activityTaskLists(ctx, queries, commanderID, now)
	if err != nil {
		// without config data there are no activity tasks, the rest still works
		logger.WithFields("Tasks", logger.FieldValue("error", err)).Warn("activity tasks unavailable")
	}
	msg := &protobuf.SC_20201{}
	for _, activity := range activities {
		info := &protobuf.ACT_TASK_INIT_LIST{ActId: proto.Uint32(activity.id)}
		for _, task := range activity.tasks {
			if task.finished {
				info.FinishIds = append(info.FinishIds, task.taskID)
				continue
			}
			info.Tasks = append(info.Tasks, &protobuf.ACT_TASK{Id: proto.Uint32(task.taskID), Progress: proto.Uint32(task.progress)})
		}
		msg.Info = append(msg.Info, info)
	}
	return msg
}

// weeklyInfo is SC_20101.info, the weekly missions: their progress, the
// points earned and the reward level claimed.
func (game *Game) weeklyInfo(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.WEEKLY_INFO, error) {
	tasks, err := queries.ListWeeklyTasks(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	rewardLevel, err := queries.GetWeeklyRewardLevel(ctx, commanderID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	pt, err := queries.GetVirtualItem(ctx, db.GetVirtualItemParams{CommanderID: commanderID, ItemID: VirtualItemWeeklyPt})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	info := &protobuf.WEEKLY_INFO{Pt: proto.Uint32(pt), RewardLv: proto.Uint32(rewardLevel)}
	for _, task := range tasks {
		info.Task = append(info.Task, &protobuf.WEEKLY_TASK_P20{Id: proto.Uint32(task.TaskID), Progress: proto.Uint32(task.Progress)})
	}
	return info, nil
}

// starterTasks are the tasks besides the commander manual unlock ones a new
// commander has in the capture's SC_20001: the first main story task ("An
// Exercise") and two TYPE_NEW_WEEKLY (13) tasks the server picked, enhance
// gear (7307) and touch the secretary (7310).
var starterTasks = []uint32{1, 7310, 7307}

// grantTasksStarter gives a new commander the capture's SC_20001 tasks: the
// tasks that unlock commander manual pages (tutorial_handbook_task.unlock,
// 23501-23517) and starterTasks, plus the first step of every weekly mission
// chain open at level 1 (weekly_task_template, 10001, 10011, ... 10141).
func grantTasksStarter(ctx context.Context, queries *db.Queries, commander db.Commander) error {
	// what a new commander already is, for tasks that start from it
	for _, counter := range []TaskEvent{{SubType: TaskSubLevel, Count: 1}, {SubType: TaskSubAcademyLevel, Count: 1}} {
		if err := queries.SetTaskCounter(ctx, db.SetTaskCounterParams{CommanderID: commander.ID, SubType: counter.SubType, Value: counter.Count}); err != nil {
			return err
		}
	}
	pages, err := handbookPageConfigs.get()
	if err != nil {
		return err
	}
	var unlockTasks []uint32
	for _, page := range pages {
		for _, id := range page.Unlock {
			if !slices.Contains(unlockTasks, id) {
				unlockTasks = append(unlockTasks, id)
			}
		}
	}
	slices.Sort(unlockTasks)
	if _, err := acceptTasks(ctx, queries, commander.ID, append(unlockTasks, starterTasks...), uint32(commander.RegisteredAt)); err != nil {
		return err
	}
	if _, err := unlockWeeklyTasks(ctx, queries, commander.ID, 1); err != nil {
		return err
	}
	return queries.SetWeeklyRewardLevel(ctx, db.SetWeeklyRewardLevelParams{CommanderID: commander.ID, RewardLv: 0})
}

// unlockWeeklyTasks adds the first step of the weekly mission chains (ids
// grouped by tens: 10001-10007, 10011-10016...) open at level that the
// commander doesn't have yet, and returns them.
func unlockWeeklyTasks(ctx context.Context, queries *db.Queries, commanderID uint32, level uint32) ([]uint32, error) {
	configs, err := weeklyTaskConfigs.get()
	if err != nil {
		return nil, err
	}
	current, err := queries.ListWeeklyTasks(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	have := map[uint32]bool{}
	for _, task := range current {
		have[task.TaskID/10] = true
	}
	first := map[uint32]weeklyTaskConfig{}
	for _, config := range configs {
		if step, ok := first[config.ID/10]; !ok || config.ID < step.ID {
			first[config.ID/10] = config
		}
	}
	var added []uint32
	for chain, step := range first {
		if have[chain] || step.Level > level {
			continue
		}
		if err := queries.CreateWeeklyTask(ctx, db.CreateWeeklyTaskParams{CommanderID: commanderID, TaskID: step.ID, Progress: 0}); err != nil {
			return nil, err
		}
		added = append(added, step.ID)
	}
	slices.Sort(added)
	return added, nil
}

type taskCounterKey struct{ subType, target uint32 }

func listTaskCounters(ctx context.Context, queries *db.Queries, commanderID uint32) (map[taskCounterKey]uint32, error) {
	rows, err := queries.ListTaskCounters(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	counters := make(map[taskCounterKey]uint32, len(rows))
	for _, row := range rows {
		counters[taskCounterKey{row.SubType, row.Target}] = row.Value
	}
	return counters, nil
}

// initialProgress is where a task accepted now starts, see
// taskInheritedSubTypes.
func initialProgress(config taskConfig, counters map[taskCounterKey]uint32) uint32 {
	if !taskInheritedSubTypes[config.SubType] && !(taskInheritedCountSubTypes[config.SubType] && config.AutoCommit == 0) {
		return 0
	}
	target := parseTaskTarget(config.TargetID)
	var progress uint32
	switch {
	case target.any || target.min != nil:
		progress = counters[taskCounterKey{config.SubType, 0}]
	default:
		for _, id := range target.ids {
			value := counters[taskCounterKey{config.SubType, id}]
			if taskAbsoluteSubTypes[config.SubType] {
				progress = max(progress, value)
			} else {
				progress += value
			}
		}
	}
	return min(progress, config.TargetNum)
}

// acceptTasks gives the commander the tasks they don't have yet and returns
// them as SC_20003 (nil when there is none). Unknown task ids are skipped.
func acceptTasks(ctx context.Context, queries *db.Queries, commanderID uint32, ids []uint32, now uint32) (*protobuf.SC_20003, error) {
	configs, err := taskConfigs.get()
	if err != nil {
		return nil, err
	}
	counters, err := listTaskCounters(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	added := &protobuf.SC_20003{}
	for _, id := range ids {
		config, ok := configs[id]
		if !ok {
			continue
		}
		progress := initialProgress(config, counters)
		created, err := queries.CreateTask(ctx, db.CreateTaskParams{CommanderID: commanderID, TaskID: id, Progress: progress, AcceptTime: now})
		if err != nil {
			return nil, err
		}
		if created == 0 {
			continue
		}
		added.Info = append(added.Info, &protobuf.TASK_ADD{
			Id:         proto.Uint32(id),
			Progress:   proto.Uint32(progress),
			AcceptTime: proto.Uint32(now),
			SubmitTime: proto.Uint32(0),
		})
	}
	if len(added.Info) == 0 {
		return nil, nil
	}
	return added, nil
}

// acceptTasksPush gives the commander tasks (e.g. the ones an activity hands
// out) and pushes them (SC_20003).
func (game *Game) acceptTasksPush(ctx context.Context, client *connection.Client, ids ...uint32) error {
	added, err := acceptTasks(ctx, game.DB.Queries, client.CommanderID, ids, uint32(time.Now().Unix()))
	if err != nil || added == nil {
		return err
	}
	return client.Push(added)
}

// applyTaskEvents stores events (counters and task progress) and returns the
// progress pushes to send: SC_20002 (tasks), SC_20102 (weekly missions),
// SC_20103 (weekly missions unlocked by a level up), SC_20202 (activity
// tasks). Game.tx runs it for the events a request records.
func (game *Game) applyTaskEvents(ctx context.Context, queries *db.Queries, commanderID uint32, events []TaskEvent) ([]proto.Message, error) {
	if len(events) == 0 {
		return nil, nil
	}
	configs, err := taskConfigs.get()
	if err != nil {
		return nil, err
	}
	weeklyConfigs, err := weeklyTaskConfigs.get()
	if err != nil {
		return nil, err
	}

	counters, err := listTaskCounters(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	changed := map[taskCounterKey]bool{}
	var level uint32
	for _, event := range events {
		keys := []taskCounterKey{{event.SubType, event.Target}}
		if event.Target != 0 {
			keys = append(keys, taskCounterKey{event.SubType, 0})
		}
		for _, key := range keys {
			switch {
			case !taskAbsoluteSubTypes[event.SubType]:
				counters[key] += event.Count
			case key.target == event.Target || event.Count > counters[key]:
				counters[key] = event.Count
			}
			changed[key] = true
		}
		if event.SubType == TaskSubLevel {
			level = event.Count
		}
	}
	for key := range changed {
		if err := queries.SetTaskCounter(ctx, db.SetTaskCounterParams{CommanderID: commanderID, SubType: key.subType, Target: key.target, Value: counters[key]}); err != nil {
			return nil, err
		}
	}

	// progress returns the task's progress after the events, and whether any
	// of them counted for it
	progress := func(subType uint32, target taskTarget, current uint32) (uint32, bool) {
		counted := false
		for _, event := range events {
			if event.SubType != subType || !target.matches(event.Target) {
				continue
			}
			counted = true
			if taskAbsoluteSubTypes[subType] {
				current = event.Count
			} else {
				current += event.Count
			}
		}
		return current, counted
	}

	var pushes []proto.Message
	tasks, err := queries.ListTasks(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	taskProgress := &protobuf.SC_20002{}
	for _, task := range tasks {
		config, ok := configs[task.TaskID]
		if !ok {
			continue
		}
		value, counted := progress(config.SubType, parseTaskTarget(config.TargetID), task.Progress)
		if !counted {
			continue
		}
		// TaskProxy tasks stop at their target (the second win re-sends
		// "An Exercise" at 1/1)
		value = min(value, config.TargetNum)
		if err := queries.SetTaskProgress(ctx, db.SetTaskProgressParams{Progress: value, CommanderID: commanderID, TaskID: task.TaskID}); err != nil {
			return nil, err
		}
		taskProgress.Info = append(taskProgress.Info, &protobuf.TASK_PROGRESS{Id: proto.Uint32(task.TaskID), Progress: proto.Uint32(value)})
	}
	if len(taskProgress.Info) > 0 {
		pushes = append(pushes, taskProgress)
	}

	weekly, err := queries.ListWeeklyTasks(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	weeklyProgress := &protobuf.SC_20102{}
	for _, task := range weekly {
		config, ok := weeklyConfigs[task.TaskID]
		if !ok {
			continue
		}
		// weekly missions keep counting past their target (2/1 wins in the
		// capture)
		value, counted := progress(config.SubType, parseTaskTarget(config.TargetID), task.Progress)
		if !counted {
			continue
		}
		if err := queries.SetWeeklyTaskProgress(ctx, db.SetWeeklyTaskProgressParams{Progress: value, CommanderID: commanderID, TaskID: task.TaskID}); err != nil {
			return nil, err
		}
		weeklyProgress.Task = append(weeklyProgress.Task, &protobuf.WEEKLY_TASK_P20{Id: proto.Uint32(task.TaskID), Progress: proto.Uint32(value)})
	}
	if len(weeklyProgress.Task) > 0 {
		pushes = append(pushes, weeklyProgress)
	}
	if level > 0 {
		unlocked, err := unlockWeeklyTasks(ctx, queries, commanderID, level)
		if err != nil {
			return nil, err
		}
		if len(unlocked) > 0 {
			pushes = append(pushes, &protobuf.SC_20103{Id: unlocked})
		}
	}

	activities, err := game.activityTaskLists(ctx, queries, commanderID, time.Now())
	if err != nil {
		return nil, err
	}
	activityProgress := &protobuf.SC_20202{}
	for _, activity := range activities {
		info := &protobuf.ACT_TASK_LIST{ActId: proto.Uint32(activity.id)}
		for _, task := range activity.tasks {
			config, ok := configs[task.taskID]
			if !ok || task.finished {
				continue
			}
			value, counted := progress(config.SubType, parseTaskTarget(config.TargetID), task.progress)
			if !counted {
				continue
			}
			if err := queries.SetActivityTaskProgress(ctx, db.SetActivityTaskProgressParams{CommanderID: commanderID, ActID: activity.id, TaskID: task.taskID, Progress: value}); err != nil {
				return nil, err
			}
			info.Tasks = append(info.Tasks, &protobuf.ACT_TASK{Id: proto.Uint32(task.taskID), Progress: proto.Uint32(value)})
		}
		if len(info.Tasks) > 0 {
			activityProgress.Info = append(activityProgress.Info, info)
		}
	}
	if len(activityProgress.Info) > 0 {
		pushes = append(pushes, activityProgress)
	}
	return pushes, nil
}

type activityTask struct {
	taskID   uint32
	progress uint32
	finished bool
}

type taskActivity struct {
	id    uint32
	tasks []activityTask
}

// activityTaskLists are the open task activities (activity_template rows of
// taskActivityTypes running at now) with their tasks and the commander's
// progress on them.
func (game *Game) activityTaskLists(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) ([]taskActivity, error) {
	configs, err := activityConfigs.get()
	if err != nil {
		return nil, err
	}
	rows, err := queries.ListActivityTasks(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	stored := map[[2]uint32]db.ListActivityTasksRow{}
	for _, row := range rows {
		stored[[2]uint32{row.ActID, row.TaskID}] = row
	}
	var ids []uint32
	for id, config := range configs {
		if taskActivityTypes[config.Type] {
			if _, _, open := activityWindow(config, now); open {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	var activities []taskActivity
	for _, id := range ids {
		taskIDs, err := activityTaskIDs(configs[id], now)
		if err != nil {
			return nil, err
		}
		activity := taskActivity{id: id}
		for _, taskID := range taskIDs {
			row := stored[[2]uint32{id, taskID}]
			activity.tasks = append(activity.tasks, activityTask{taskID: taskID, progress: row.Progress, finished: row.Finished})
		}
		activities = append(activities, activity)
	}
	return activities, nil
}

// activityTaskIDs lists an activity's tasks: config_data holds them, except
// for the cruise pass where it holds battlepass_task_group ids, a group
// opening each week (group_mask) since the activity started
// (CrusingTaskLayer, GetServerOverWeek). Each group is listed backwards, as
// the capture does.
func activityTaskIDs(config activityConfig, now time.Time) ([]uint32, error) {
	ids := parseIDList(config.ConfigData)
	if config.Type != activityTypeCruise {
		return ids, nil
	}
	groups, err := battlepassTaskGroups.get()
	if err != nil {
		return nil, err
	}
	start, _, _ := activityWindow(config, now)
	weekStart := start.Unix() - (start.Unix()-mondayReference)%(7*86400)
	weeks := uint32((now.Unix() - weekStart + 7*86400 - 1) / (7 * 86400))
	var tasks []uint32
	for _, groupID := range ids {
		group, ok := groups[groupID]
		if !ok || (group.IsOntime == 1 && group.GroupMask > weeks) {
			continue
		}
		var groupTasks []uint32
		for _, chain := range group.TaskGroup {
			groupTasks = append(groupTasks, chain...)
		}
		slices.Reverse(groupTasks)
		tasks = append(tasks, groupTasks...)
	}
	return tasks, nil
}

// activityWindow reads activity_template.time: "always", or
// ["timer", [[y,m,d],[h,m,s]], [[y,m,d],[h,m,s]]] in server time.
func activityWindow(config activityConfig, now time.Time) (time.Time, time.Time, bool) {
	var always string
	if json.Unmarshal(config.Time, &always) == nil {
		return time.Time{}, time.Time{}, always == "always"
	}
	var timer []json.RawMessage
	if json.Unmarshal(config.Time, &timer) != nil || len(timer) < 3 {
		return time.Time{}, time.Time{}, false
	}
	start, okStart := configTime(timer[1])
	end, okEnd := configTime(timer[2])
	if !okStart || !okEnd {
		return time.Time{}, time.Time{}, false
	}
	return start, end, !now.Before(start) && now.Before(end)
}

func configTime(raw json.RawMessage) (time.Time, bool) {
	var parts [2][3]int
	if json.Unmarshal(raw, &parts) != nil {
		return time.Time{}, false
	}
	return time.Date(parts[0][0], time.Month(parts[0][1]), parts[0][2], parts[1][0], parts[1][1], parts[1][2], 0, serverZone), true
}

// awardChoice picks the award_choice option the client chose (CS_20005
// choice_award, for tasks with selectable rewards), nil if it isn't one.
func awardChoice(config taskConfig, chosen []*protobuf.DROPINFO) [][]uint32 {
	var options [][][]uint32
	if len(chosen) == 0 || json.Unmarshal(config.AwardChoice, &options) != nil {
		return nil
	}
	for _, option := range options {
		if len(option) != len(chosen) {
			continue
		}
		same := true
		for i, drop := range option {
			if len(drop) < 3 || drop[0] != chosen[i].GetType() || drop[1] != chosen[i].GetId() || drop[2] != chosen[i].GetNumber() {
				same = false
				break
			}
		}
		if same {
			return option
		}
	}
	return nil
}

// handleSubmitTask claims a finished task's reward (SubmitTaskCommand). The
// progress the rewards cause and the tasks it leads to (next_task) are pushed
// before the reply, as in the capture; the client drops the task itself.
func (game *Game) handleSubmitTask(client *connection.Client, req *protobuf.CS_20005) error {
	ctx := context.Background()
	configs, err := taskConfigs.get()
	if err != nil {
		return err
	}
	config, ok := configs[req.GetId()]
	task, err := game.DB.GetTask(ctx, db.GetTaskParams{CommanderID: client.CommanderID, TaskID: req.GetId()})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err != nil || !ok || task.SubmitTime != 0 || task.Progress < config.TargetNum {
		return client.Send(&protobuf.SC_20006{Result: result(errRefused)})
	}
	drops := config.AwardDisplay
	if len(req.GetChoiceAward()) > 0 {
		if drops = awardChoice(config, req.GetChoiceAward()); drops == nil {
			return client.Send(&protobuf.SC_20006{Result: result(errRefused)})
		}
	}
	now := uint32(time.Now().Unix())
	reply := &protobuf.SC_20006{Result: result(nil)}
	var next *protobuf.SC_20003
	err = game.tx(ctx, client, func(tx *Tx) error {
		if _, err := tx.SubmitTask(ctx, db.SubmitTaskParams{SubmitTime: now, CommanderID: client.CommanderID, TaskID: task.TaskID}); err != nil {
			return err
		}
		reply.AwardList = configDrops(drops)
		if err := tx.grantDrops(reply.AwardList); err != nil {
			return err
		}
		handbookEvents, err := handbookTaskClaimed(ctx, tx.Queries, client.CommanderID, task.TaskID)
		if err != nil {
			return err
		}
		tx.event(handbookEvents...)
		next, err = acceptTasks(ctx, tx.Queries, client.CommanderID, parseIDList(config.NextTask), now)
		return err
	})
	if err != nil {
		return err
	}
	if next != nil {
		if err := client.Push(next); err != nil {
			return err
		}
	}
	return client.Send(reply)
}

// handleClientTaskEvent takes progress the client counts itself
// (MiniGameTaskProgressUpdateCommand: event_type is the task's sub_type, e.g.
// 2020 "visit the events page"), pushes it and replies.
func (game *Game) handleClientTaskEvent(client *connection.Client, req *protobuf.CS_20016) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		tx.event(TaskEvent{SubType: req.GetEventType(), Target: req.GetEventTarget(), Count: req.GetEventCount()})
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_20017{Result: result(nil)})
}

// handleSubmitActivityTasks claims finished activity tasks
// (SubmitActiveTaskCommand, the client also sends it by itself for
// auto_commit ones): the progress the rewards cause, then the claimed tasks'
// removal (SC_20204), then the rewards.
func (game *Game) handleSubmitActivityTasks(client *connection.Client, req *protobuf.CS_20205) error {
	ctx := context.Background()
	configs, err := taskConfigs.get()
	if err != nil {
		return err
	}
	activities, err := game.activityTaskLists(ctx, game.DB.Queries, client.CommanderID, time.Now())
	if err != nil {
		return err
	}
	var claimed []activityTask
	for _, activity := range activities {
		if activity.id != req.GetActId() {
			continue
		}
		for _, task := range activity.tasks {
			if slices.Contains(req.GetTaskIds(), task.taskID) {
				claimed = append(claimed, task)
			}
		}
	}
	if len(claimed) != len(req.GetTaskIds()) {
		return client.Send(&protobuf.SC_20206{Result: result(errRefused)})
	}
	for _, task := range claimed {
		if config, ok := configs[task.taskID]; !ok || task.finished || task.progress < config.TargetNum {
			return client.Send(&protobuf.SC_20206{Result: result(errRefused)})
		}
	}
	reply := &protobuf.SC_20206{Result: result(nil)}
	removed := &protobuf.ACT_TASK_LIST{ActId: proto.Uint32(req.GetActId())}
	err = game.tx(ctx, client, func(tx *Tx) error {
		for _, task := range claimed {
			if err := tx.FinishActivityTask(ctx, db.FinishActivityTaskParams{CommanderID: client.CommanderID, ActID: req.GetActId(), TaskID: task.taskID, Progress: task.progress}); err != nil {
				return err
			}
			awards := configDrops(configs[task.taskID].AwardDisplay)
			if err := tx.grantDrops(awards); err != nil {
				return err
			}
			reply.AwardList = append(reply.AwardList, awards...)
			removed.Tasks = append(removed.Tasks, &protobuf.ACT_TASK{Id: proto.Uint32(task.taskID), Progress: proto.Uint32(task.progress)})
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := client.Push(&protobuf.SC_20204{Info: []*protobuf.ACT_TASK_LIST{removed}}); err != nil {
		return err
	}
	return client.Send(reply)
}
