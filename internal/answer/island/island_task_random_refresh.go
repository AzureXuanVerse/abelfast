package island

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandTaskCategory         = "ShareCfg/island_task.json"
	islandTaskCategoryLC       = "sharecfgdata/island_task.json"
	islandTaskTargetCategory   = "ShareCfg/island_task_target.json"
	islandTaskTargetCategoryLC = "sharecfgdata/island_task_target.json"
	islandSeasonCategory       = "ShareCfg/island_season.json"
	islandSeasonCategoryLC     = "sharecfgdata/island_season.json"

	islandRandomTaskType        = uint32(4)
	islandRandomWindowSeconds   = uint32(300)
	islandRandomImmediateMaxNum = 3
)

type islandTaskTemplate struct {
	ID              uint32     `json:"id"`
	Type            uint32     `json:"type"`
	CompleteType    uint32     `json:"complete_type"`
	TriggerType     uint32     `json:"trigger_type"`
	UnlockTime      string     `json:"unlock_time"`
	UnlockCondition [][]uint32 `json:"unlock_condition"`
	LinkTask        []uint32   `json:"link_task"`
	TargetID        []uint32   `json:"target_id"`
	RewardShow      [][]uint32 `json:"reward_show"`
	RewardExp       uint32     `json:"reward_exp"`
}

type islandTaskTargetTemplate struct {
	ID        uint32 `json:"id"`
	TargetNum uint32 `json:"target_num"`
}

type islandSeasonTemplate struct {
	TaskList []uint32 `json:"task_list"`
}

type islandTaskRefreshConfig struct {
	tasksByID   map[uint32]islandTaskTemplate
	randomTasks []uint32
	targets     map[uint32]uint32
}

func IslandRandomTaskRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21030
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21031, err
	}

	response := protobuf.SC_21031{
		RemoveTaskList:   []uint32{},
		RemoveTaskFinish: []uint32{},
		TaskList:         []*protobuf.PB_TASK{},
		TaskListRandom:   []*protobuf.PB_TASK_RANDOM{},
	}

	if payload.GetType() != 0 {
		return client.SendMessage(21031, &response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		now := time.Now().UTC()
		state, err := orm.LoadIslandTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, now)
		if err != nil {
			return err
		}

		config, err := loadIslandTaskRefreshConfig()
		if err != nil {
			return err
		}

		delta := refreshIslandRandomTasks(state, config, uint32(now.Unix()))
		response.RemoveTaskList = delta.RemoveTaskList
		response.RemoveTaskFinish = delta.RemoveTaskFinish
		response.TaskList = delta.TaskList
		response.TaskListRandom = delta.TaskListRandom

		state.WeekDailyTaskNum++
		if len(state.ActiveTasks) > 0 {
			state.TraceTaskID = state.ActiveTasks[0].TaskID
			state.TraceDailyTaskID = state.ActiveTasks[0].TaskID
		} else {
			state.TraceTaskID = 0
			state.TraceDailyTaskID = 0
		}

		return orm.SaveIslandTaskProgressTx(context.Background(), tx, state)
	})
	if err != nil {
		logger.LogEvent("IslandTask", "Refresh", "failed to refresh island random tasks: "+err.Error(), logger.LOG_LEVEL_ERROR)
		return client.SendMessage(21031, &response)
	}

	return client.SendMessage(21031, &response)
}

func refreshIslandRandomTasks(state *orm.IslandTaskProgress, config *islandTaskRefreshConfig, now uint32) *protobuf.SC_21031 {
	finishedSet := make(map[uint32]struct{}, len(state.FinishedTaskIDs))
	for _, taskID := range state.FinishedTaskIDs {
		finishedSet[taskID] = struct{}{}
	}

	activeMap := make(map[uint32]orm.IslandTaskEntry, len(state.ActiveTasks))
	removeTaskList := make([]uint32, 0)
	for _, task := range state.ActiveTasks {
		if !isIslandRandomTaskAvailable(config, finishedSet, task.TaskID) {
			removeTaskList = append(removeTaskList, task.TaskID)
			continue
		}
		activeMap[task.TaskID] = task
	}

	removeTaskFinish := make([]uint32, 0)
	finishedIDs := make([]uint32, 0, len(finishedSet))
	for taskID := range finishedSet {
		if !isIslandRandomTaskAvailable(config, finishedSet, taskID) {
			removeTaskFinish = append(removeTaskFinish, taskID)
			continue
		}
		finishedIDs = append(finishedIDs, taskID)
	}
	sort.Slice(finishedIDs, func(i, j int) bool { return finishedIDs[i] < finishedIDs[j] })

	pendingWindows := make([]orm.IslandTaskEntry, 0, len(state.RandomTaskWindows))
	addedTasks := make([]*protobuf.PB_TASK, 0)
	for _, window := range state.RandomTaskWindows {
		if !isIslandRandomTaskAvailable(config, finishedSet, window.TaskID) {
			continue
		}
		if window.Timestamp <= now {
			if _, exists := activeMap[window.TaskID]; exists {
				continue
			}
			if containsIslandTaskID(finishedIDs, window.TaskID) {
				continue
			}
			if len(activeMap) >= islandRandomImmediateMaxNum {
				pendingWindows = append(pendingWindows, window)
				continue
			}
			activeMap[window.TaskID] = orm.IslandTaskEntry{TaskID: window.TaskID, Timestamp: now}
			if task := buildIslandTaskProto(config, window.TaskID, now); task != nil {
				addedTasks = append(addedTasks, task)
			}
			continue
		}
		pendingWindows = append(pendingWindows, window)
	}

	pendingSet := make(map[uint32]struct{}, len(pendingWindows))
	for _, window := range pendingWindows {
		pendingSet[window.TaskID] = struct{}{}
	}

	for _, taskID := range config.randomTasks {
		if _, exists := activeMap[taskID]; exists {
			continue
		}
		if _, exists := pendingSet[taskID]; exists {
			continue
		}
		if containsIslandTaskID(finishedIDs, taskID) {
			continue
		}
		if !isIslandRandomTaskAvailable(config, finishedSet, taskID) {
			continue
		}

		if len(activeMap) < islandRandomImmediateMaxNum {
			activeMap[taskID] = orm.IslandTaskEntry{TaskID: taskID, Timestamp: now}
			if task := buildIslandTaskProto(config, taskID, now); task != nil {
				addedTasks = append(addedTasks, task)
			}
			continue
		}

		pendingWindows = append(pendingWindows, orm.IslandTaskEntry{TaskID: taskID, Timestamp: now + islandRandomWindowSeconds})
		pendingSet[taskID] = struct{}{}
	}

	activeTasks := make([]orm.IslandTaskEntry, 0, len(activeMap))
	for _, entry := range activeMap {
		activeTasks = append(activeTasks, entry)
	}
	sort.Slice(activeTasks, func(i, j int) bool { return activeTasks[i].TaskID < activeTasks[j].TaskID })
	state.ActiveTasks = activeTasks
	state.FinishedTaskIDs = finishedIDs
	sort.Slice(pendingWindows, func(i, j int) bool {
		if pendingWindows[i].Timestamp == pendingWindows[j].Timestamp {
			return pendingWindows[i].TaskID < pendingWindows[j].TaskID
		}
		return pendingWindows[i].Timestamp < pendingWindows[j].Timestamp
	})
	state.FutureTaskWindows = pendingWindows
	state.RandomTaskWindows = pendingWindows

	taskListRandom := make([]*protobuf.PB_TASK_RANDOM, 0, len(pendingWindows))
	for _, window := range pendingWindows {
		taskListRandom = append(taskListRandom, &protobuf.PB_TASK_RANDOM{
			TaskId:    proto.Uint32(window.TaskID),
			Timestamp: proto.Uint32(window.Timestamp),
		})
	}

	sort.Slice(removeTaskList, func(i, j int) bool { return removeTaskList[i] < removeTaskList[j] })
	sort.Slice(removeTaskFinish, func(i, j int) bool { return removeTaskFinish[i] < removeTaskFinish[j] })
	sort.Slice(addedTasks, func(i, j int) bool { return addedTasks[i].GetId() < addedTasks[j].GetId() })

	return &protobuf.SC_21031{
		RemoveTaskList:   removeTaskList,
		RemoveTaskFinish: removeTaskFinish,
		TaskList:         addedTasks,
		TaskListRandom:   taskListRandom,
	}
}

func buildIslandTaskProto(config *islandTaskRefreshConfig, taskID uint32, now uint32) *protobuf.PB_TASK {
	template, ok := config.tasksByID[taskID]
	if !ok {
		return nil
	}
	processes := make([]*protobuf.PB_TASK_PROCESS, 0, len(template.TargetID))
	for _, targetID := range template.TargetID {
		targetNum := config.targets[targetID]
		processes = append(processes, &protobuf.PB_TASK_PROCESS{
			TargetId:    proto.Uint32(targetID),
			TargetCount: proto.Uint32(targetNum),
		})
	}
	return &protobuf.PB_TASK{
		Id:          proto.Uint32(taskID),
		Timestamp:   proto.Uint32(now),
		ProcessList: processes,
	}
}

func isIslandRandomTaskAvailable(config *islandTaskRefreshConfig, finished map[uint32]struct{}, taskID uint32) bool {
	template, ok := config.tasksByID[taskID]
	if !ok || template.Type != islandRandomTaskType {
		return false
	}
	if template.UnlockTime != "" && template.UnlockTime != "always" {
		return false
	}
	for _, condition := range template.UnlockCondition {
		if len(condition) < 2 {
			continue
		}
		if condition[0] == 2 {
			if _, ok := finished[condition[1]]; !ok {
				return false
			}
		}
	}
	return true
}

func containsIslandTaskID(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func listConfigEntriesWithFallback(primaryCategory string, fallbackCategory string, lister func(string) ([]orm.ConfigEntry, error)) ([]orm.ConfigEntry, error) {
	entries, err := lister(primaryCategory)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		return entries, nil
	}

	fallbackEntries, err := lister(fallbackCategory)
	if err != nil {
		return nil, err
	}
	return fallbackEntries, nil
}

func loadIslandTaskRefreshConfig() (*islandTaskRefreshConfig, error) {
	taskEntries, err := listConfigEntriesWithFallback(islandTaskCategory, islandTaskCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	targetEntries, err := listConfigEntriesWithFallback(islandTaskTargetCategory, islandTaskTargetCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}

	tasksByID := make(map[uint32]islandTaskTemplate, len(taskEntries))
	randomTasks := make([]uint32, 0)
	for _, entry := range taskEntries {
		var task islandTaskTemplate
		if err := json.Unmarshal(entry.Data, &task); err != nil {
			return nil, err
		}
		if task.ID == 0 {
			if parsedID, parseErr := strconv.ParseUint(entry.Key, 10, 32); parseErr == nil {
				task.ID = uint32(parsedID)
			}
		}
		tasksByID[task.ID] = task
		if task.Type == islandRandomTaskType {
			randomTasks = append(randomTasks, task.ID)
		}
	}
	sort.Slice(randomTasks, func(i, j int) bool { return randomTasks[i] < randomTasks[j] })

	targets := make(map[uint32]uint32, len(targetEntries))
	for _, entry := range targetEntries {
		var target islandTaskTargetTemplate
		if err := json.Unmarshal(entry.Data, &target); err != nil {
			return nil, err
		}
		if target.ID == 0 {
			if parsedID, parseErr := strconv.ParseUint(entry.Key, 10, 32); parseErr == nil {
				target.ID = uint32(parsedID)
			}
		}
		targets[target.ID] = target.TargetNum
	}

	return &islandTaskRefreshConfig{
		tasksByID:   tasksByID,
		randomTasks: randomTasks,
		targets:     targets,
	}, nil
}
