package answer

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandTaskCompleteTypeSubmit = uint32(3)
	islandTaskTypeSeason         = uint32(8)
	islandTaskTriggerServerOnly  = uint32(1)
)

var errIslandTaskInvalid = errors.New("invalid island task operation")

func IslandAcceptTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21032
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21033, err
	}

	response := protobuf.SC_21033{TaskList: []*protobuf.PB_TASK{}}
	requestIDs := dedupeTaskIDs(payload.GetTaskIdList())
	if len(requestIDs) == 0 {
		return client.SendMessage(21033, &response)
	}

	tasksByID, targetNums, err := loadIslandTaskConfigMaps()
	if err != nil {
		return 0, 21033, err
	}
	seasonTaskSet, err := loadIslandSeasonTaskSet()
	if err != nil {
		return 0, 21033, err
	}

	now := uint32(time.Now().UTC().Unix())
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadIslandTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		activeSet := make(map[uint32]struct{}, len(state.ActiveTasks))
		for _, active := range state.ActiveTasks {
			activeSet[active.TaskID] = struct{}{}
		}
		finishedSet := make(map[uint32]struct{}, len(state.FinishedTaskIDs))
		for _, finishedID := range state.FinishedTaskIDs {
			finishedSet[finishedID] = struct{}{}
		}

		accepted := make([]*protobuf.PB_TASK, 0, len(requestIDs))
		for _, taskID := range requestIDs {
			task, ok := tasksByID[taskID]
			if !ok {
				continue
			}
			if !canAcceptIslandTask(task, seasonTaskSet, activeSet, finishedSet) {
				continue
			}

			entry := orm.IslandTaskEntry{
				TaskID:      taskID,
				Timestamp:   now,
				ProcessList: buildIslandTaskInitialProcesses(task, targetNums),
			}
			state.ActiveTasks = append(state.ActiveTasks, entry)
			activeSet[taskID] = struct{}{}
			accepted = append(accepted, buildIslandTaskPBTask(entry))
		}

		if len(state.ActiveTasks) > 0 {
			state.TraceTaskID = state.ActiveTasks[0].TaskID
			state.TraceDailyTaskID = state.ActiveTasks[0].TaskID
		}

		if err := orm.SaveIslandTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}

		sort.Slice(accepted, func(i, j int) bool { return accepted[i].GetId() < accepted[j].GetId() })
		response.TaskList = accepted
		return nil
	})
	if err != nil {
		return 0, 21033, err
	}

	return client.SendMessage(21033, &response)
}

func IslandUpdateTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21036
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21037, err
	}

	response := protobuf.SC_21037{Result: proto.Uint32(taskResultFailed), TaskList: []*protobuf.PB_TASK{}}
	if payload.TaskId == nil || payload.TargetId == nil || payload.TargetCount == nil {
		return client.SendMessage(21037, &response)
	}
	if payload.GetTaskId() == 0 {
		return client.SendMessage(21037, &response)
	}
	if payload.GetTargetId() == 0 || payload.GetTargetCount() == 0 {
		return client.SendMessage(21037, &response)
	}

	tasksByID, targetNums, err := loadIslandTaskConfigMaps()
	if err != nil {
		return 0, 21037, err
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadIslandTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		updated := make([]*protobuf.PB_TASK, 0)
		for idx := range state.ActiveTasks {
			entry := &state.ActiveTasks[idx]
			if entry.TaskID != payload.GetTaskId() {
				continue
			}
			template, ok := tasksByID[entry.TaskID]
			if !ok {
				continue
			}
			entry.ProcessList = ensureIslandTaskProcessList(entry.ProcessList, template, targetNums)

			changed := false
			for processIdx := range entry.ProcessList {
				process := &entry.ProcessList[processIdx]
				if process.TargetID != payload.GetTargetId() {
					continue
				}
				targetMax, ok := targetNums[process.TargetID]
				if !ok {
					continue
				}
				next := process.TargetCount + payload.GetTargetCount()
				if next < process.TargetCount || next > targetMax {
					next = targetMax
				}
				process.TargetCount = next
				changed = true
			}
			if changed {
				updated = append(updated, buildIslandTaskPBTask(*entry))
			}
		}

		if len(updated) == 0 {
			return errIslandTaskInvalid
		}
		if err := orm.SaveIslandTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}
		sort.Slice(updated, func(i, j int) bool { return updated[i].GetId() < updated[j].GetId() })
		response.Result = proto.Uint32(taskResultSuccess)
		response.TaskList = updated
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandTaskInvalid) {
			return client.SendMessage(21037, &response)
		}
		return 0, 21037, err
	}

	return client.SendMessage(21037, &response)
}

func IslandSubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21038
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21039, err
	}

	response := protobuf.SC_21039{Result: proto.Uint32(taskResultFailed), DropList: []*protobuf.DROPINFO{}}
	if payload.TaskId == nil || payload.GetTaskId() == 0 {
		return client.SendMessage(21039, &response)
	}

	tasksByID, targetNums, err := loadIslandTaskConfigMaps()
	if err != nil {
		return 0, 21039, err
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadIslandTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		template, ok := tasksByID[payload.GetTaskId()]
		if !ok {
			return errIslandTaskInvalid
		}
		entry, activeIndex := findIslandActiveTask(state.ActiveTasks, payload.GetTaskId())
		if activeIndex < 0 {
			return errIslandTaskInvalid
		}
		if !isIslandTaskComplete(entry, template, targetNums) {
			return errIslandTaskInvalid
		}

		drops := buildIslandTaskDrops(template)
		if err := applyIslandTaskDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}

		state.ActiveTasks = removeIslandActiveTask(state.ActiveTasks, activeIndex)
		if !containsIslandTaskID(state.FinishedTaskIDs, payload.GetTaskId()) {
			state.FinishedTaskIDs = append(state.FinishedTaskIDs, payload.GetTaskId())
		}
		state.TraceTaskID, state.TraceDailyTaskID = nextIslandTraceTask(state.ActiveTasks)
		if err := orm.SaveIslandTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(taskResultSuccess)
		response.DropList = dropMapToSortedList(drops)
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandTaskInvalid) {
			return client.SendMessage(21039, &response)
		}
		return 0, 21039, err
	}

	return client.SendMessage(21039, &response)
}

func IslandSubmitTaskOneStep(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21041
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21042, err
	}

	response := protobuf.SC_21042{Result: proto.Uint32(taskResultFailed), DropList: []*protobuf.DROPINFO{}}
	taskIDs := dedupeTaskIDs(payload.GetTaskIds())
	if len(taskIDs) == 0 {
		return client.SendMessage(21042, &response)
	}

	tasksByID, targetNums, err := loadIslandTaskConfigMaps()
	if err != nil {
		return 0, 21042, err
	}
	seasonTaskSet, err := loadIslandSeasonTaskSet()
	if err != nil {
		return 0, 21042, err
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadIslandTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		activeIndexes := make([]int, 0, len(taskIDs))
		mergedDrops := make(map[string]*protobuf.DROPINFO)
		for _, taskID := range taskIDs {
			template, ok := tasksByID[taskID]
			if !ok || !seasonTaskSet[taskID] || template.Type != islandTaskTypeSeason || template.CompleteType != islandTaskCompleteTypeSubmit {
				return errIslandTaskInvalid
			}

			entry, activeIndex := findIslandActiveTask(state.ActiveTasks, taskID)
			if activeIndex < 0 {
				return errIslandTaskInvalid
			}
			if !isIslandTaskComplete(entry, template, targetNums) {
				return errIslandTaskInvalid
			}

			activeIndexes = append(activeIndexes, activeIndex)
			taskDrops := buildIslandTaskDrops(template)
			for _, drop := range taskDrops {
				accumulateDrop(mergedDrops, drop.GetType(), drop.GetId(), drop.GetNumber())
			}
		}

		sort.Sort(sort.Reverse(sort.IntSlice(activeIndexes)))
		for _, activeIndex := range activeIndexes {
			taskID := state.ActiveTasks[activeIndex].TaskID
			state.ActiveTasks = removeIslandActiveTask(state.ActiveTasks, activeIndex)
			if !containsIslandTaskID(state.FinishedTaskIDs, taskID) {
				state.FinishedTaskIDs = append(state.FinishedTaskIDs, taskID)
			}
		}

		if err := applyIslandTaskDropsTx(context.Background(), tx, client, mergedDrops); err != nil {
			return err
		}

		state.TraceTaskID, state.TraceDailyTaskID = nextIslandTraceTask(state.ActiveTasks)
		if err := orm.SaveIslandTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(taskResultSuccess)
		response.DropList = dropMapToSortedList(mergedDrops)
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandTaskInvalid) {
			return client.SendMessage(21042, &response)
		}
		return 0, 21042, err
	}

	return client.SendMessage(21042, &response)
}

func loadIslandTaskConfigMaps() (map[uint32]islandTaskTemplate, map[uint32]uint32, error) {
	taskEntries, err := listConfigEntriesWithFallback(islandTaskCategory, islandTaskCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, nil, err
	}
	targetEntries, err := listConfigEntriesWithFallback(islandTaskTargetCategory, islandTaskTargetCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, nil, err
	}

	tasksByID := make(map[uint32]islandTaskTemplate, len(taskEntries))
	for _, entry := range taskEntries {
		var task islandTaskTemplate
		if err := json.Unmarshal(entry.Data, &task); err != nil {
			return nil, nil, err
		}
		if task.ID == 0 {
			parsedID, parseErr := strconv.ParseUint(entry.Key, 10, 32)
			if parseErr != nil {
				continue
			}
			task.ID = uint32(parsedID)
		}
		tasksByID[task.ID] = task
	}

	targetNums := make(map[uint32]uint32, len(targetEntries))
	for _, entry := range targetEntries {
		var target islandTaskTargetTemplate
		if err := json.Unmarshal(entry.Data, &target); err != nil {
			return nil, nil, err
		}
		if target.ID == 0 {
			parsedID, parseErr := strconv.ParseUint(entry.Key, 10, 32)
			if parseErr != nil {
				continue
			}
			target.ID = uint32(parsedID)
		}
		targetNums[target.ID] = target.TargetNum
	}

	return tasksByID, targetNums, nil
}

func loadIslandSeasonTaskSet() (map[uint32]bool, error) {
	seasonEntries, err := listConfigEntriesWithFallback(islandSeasonCategory, islandSeasonCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	set := make(map[uint32]bool)
	for _, entry := range seasonEntries {
		var season islandSeasonTemplate
		if err := json.Unmarshal(entry.Data, &season); err != nil {
			return nil, err
		}
		for _, taskID := range season.TaskList {
			set[taskID] = true
		}
	}
	return set, nil
}

func canAcceptIslandTask(task islandTaskTemplate, seasonTaskSet map[uint32]bool, activeSet map[uint32]struct{}, finishedSet map[uint32]struct{}) bool {
	if task.ID == 0 {
		return false
	}
	if !seasonTaskSet[task.ID] || task.Type != islandTaskTypeSeason {
		return false
	}
	if _, exists := activeSet[task.ID]; exists {
		return false
	}
	if _, exists := finishedSet[task.ID]; exists {
		return false
	}
	if task.TriggerType == islandTaskTriggerServerOnly {
		return false
	}
	if task.UnlockTime != "" && task.UnlockTime != "always" {
		return false
	}

	for _, condition := range task.UnlockCondition {
		if len(condition) < 2 {
			continue
		}
		if condition[0] != 2 {
			return false
		}
		if _, done := finishedSet[condition[1]]; !done {
			return false
		}
	}
	for _, linkedTaskID := range task.LinkTask {
		if linkedTaskID == 0 {
			continue
		}
		if _, done := finishedSet[linkedTaskID]; !done {
			return false
		}
		if _, active := activeSet[linkedTaskID]; active {
			return false
		}
	}
	return true
}

func buildIslandTaskInitialProcesses(task islandTaskTemplate, targetNums map[uint32]uint32) []orm.IslandTaskTargetProcess {
	processes := make([]orm.IslandTaskTargetProcess, 0, len(task.TargetID))
	for _, targetID := range task.TargetID {
		if _, ok := targetNums[targetID]; !ok {
			continue
		}
		processes = append(processes, orm.IslandTaskTargetProcess{TargetID: targetID, TargetCount: 0})
	}
	return processes
}

func ensureIslandTaskProcessList(current []orm.IslandTaskTargetProcess, task islandTaskTemplate, targetNums map[uint32]uint32) []orm.IslandTaskTargetProcess {
	if len(current) > 0 {
		return current
	}
	return buildIslandTaskInitialProcesses(task, targetNums)
}

func buildIslandTaskPBTask(entry orm.IslandTaskEntry) *protobuf.PB_TASK {
	processes := make([]*protobuf.PB_TASK_PROCESS, 0, len(entry.ProcessList))
	for _, process := range entry.ProcessList {
		processes = append(processes, &protobuf.PB_TASK_PROCESS{
			TargetId:    proto.Uint32(process.TargetID),
			TargetCount: proto.Uint32(process.TargetCount),
		})
	}
	return &protobuf.PB_TASK{
		Id:          proto.Uint32(entry.TaskID),
		Timestamp:   proto.Uint32(entry.Timestamp),
		ProcessList: processes,
	}
}

func findIslandActiveTask(entries []orm.IslandTaskEntry, taskID uint32) (orm.IslandTaskEntry, int) {
	for idx, entry := range entries {
		if entry.TaskID == taskID {
			return entry, idx
		}
	}
	return orm.IslandTaskEntry{}, -1
}

func removeIslandActiveTask(entries []orm.IslandTaskEntry, removeIndex int) []orm.IslandTaskEntry {
	if removeIndex < 0 || removeIndex >= len(entries) {
		return entries
	}
	return append(entries[:removeIndex], entries[removeIndex+1:]...)
}

func nextIslandTraceTask(entries []orm.IslandTaskEntry) (uint32, uint32) {
	if len(entries) == 0 {
		return 0, 0
	}
	return entries[0].TaskID, entries[0].TaskID
}

func isIslandTaskComplete(entry orm.IslandTaskEntry, task islandTaskTemplate, targetNums map[uint32]uint32) bool {
	if len(task.TargetID) == 0 {
		return true
	}
	progressByTarget := make(map[uint32]uint32, len(entry.ProcessList))
	for _, process := range entry.ProcessList {
		progressByTarget[process.TargetID] = process.TargetCount
	}
	for _, targetID := range task.TargetID {
		targetNum, ok := targetNums[targetID]
		if !ok {
			return false
		}
		if progressByTarget[targetID] < targetNum {
			return false
		}
	}
	return true
}

func buildIslandTaskDrops(template islandTaskTemplate) map[string]*protobuf.DROPINFO {
	drops := make(map[string]*protobuf.DROPINFO)
	for _, reward := range template.RewardShow {
		if len(reward) < 3 {
			continue
		}
		accumulateDrop(drops, reward[0], reward[1], reward[2])
	}
	return drops
}

func applyIslandTaskDropsTx(ctx context.Context, tx pgx.Tx, client *connection.Client, drops map[string]*protobuf.DROPINFO) error {
	for _, drop := range drops {
		switch drop.GetType() {
		case consts.DROP_TYPE_ISLAND_ITEM:
			if err := orm.AddIslandInventoryTx(ctx, tx, client.Commander.CommanderID, drop.GetId(), drop.GetNumber()); err != nil {
				return err
			}
		case consts.VIRTUAL_DROP_TYPE_ISLAND_SEASON_PT:
			if err := orm.AddIslandSeasonPTTx(ctx, tx, client.Commander.CommanderID, drop.GetNumber()); err != nil {
				return err
			}
		default:
			single := map[string]*protobuf.DROPINFO{"reward": drop}
			if err := applyLoveLetterDropsTx(ctx, tx, client, single); err != nil {
				return err
			}
		}
	}
	return nil
}

func dedupeTaskIDs(ids []uint32) []uint32 {
	seen := make(map[uint32]struct{}, len(ids))
	uniq := make([]uint32, 0, len(ids))
	for _, taskID := range ids {
		if taskID == 0 {
			continue
		}
		if _, exists := seen[taskID]; exists {
			continue
		}
		seen[taskID] = struct{}{}
		uniq = append(uniq, taskID)
	}
	return uniq
}
