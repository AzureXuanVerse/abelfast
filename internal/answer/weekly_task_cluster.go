package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	weeklyTaskSuccessResult uint32 = 0
	weeklyTaskFailedResult  uint32 = 1

	weeklyTaskTemplateCategory = "ShareCfg/weekly_task_template.json"
	gamesetCategory            = "ShareCfg/gameset.json"
)

type weeklyTaskTemplate struct {
	ID           uint32   `json:"id"`
	Level        uint32   `json:"level"`
	SubType      uint32   `json:"sub_type"`
	TargetNum    uint32   `json:"target_num"`
	AwardDisplay []uint32 `json:"award_display"`
}

type weeklyGamesetEntry struct {
	Description json.RawMessage `json:"description"`
}

type weeklyTaskConfig struct {
	templatesByID  map[uint32]weeklyTaskTemplate
	templatesBySub map[uint32][]weeklyTaskTemplate
	targets        []uint32
	dropClient     [][][]uint32
}

func WeeklyMissions(buffer *[]byte, client *connection.Client) (int, int, error) {
	state, err := orm.LoadWeeklyTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		logger.LogEvent("WeeklyTask", "Load", fmt.Sprintf("commander=%d load failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		state = &orm.WeeklyTaskProgress{Tasks: []orm.WeeklyTaskEntry{}}
	}
	response := protobuf.SC_20101{
		Info: &protobuf.WEEKLY_INFO{
			Task:     toWeeklyTaskProto(state.Tasks),
			Pt:       proto.Uint32(state.Pt),
			RewardLv: proto.Uint32(state.RewardLv),
		},
	}
	return client.SendMessage(20101, &response)
}

func SubmitWeeklyTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20106
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 0, err
	}
	config, err := loadWeeklyTaskConfig()
	if err != nil {
		logger.LogEvent("WeeklyTask", "Config", fmt.Sprintf("commander=%d config load failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		return client.SendMessage(20107, &protobuf.SC_20107{Result: proto.Uint32(weeklyTaskFailedResult)})
	}

	result := weeklyTaskSuccessResult
	var next *protobuf.WEEKLY_TASK_P20
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadWeeklyTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, nowUTC())
		if err != nil {
			return err
		}
		nextTask, ok := submitSingleWeeklyTask(state, payload.GetId(), config)
		if !ok {
			result = weeklyTaskFailedResult
			return nil
		}
		if err := orm.SaveWeeklyTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}
		next = toWeeklyTaskPointer(nextTask)
		return nil
	})
	if err != nil {
		logger.LogEvent("WeeklyTask", "SubmitSingle", fmt.Sprintf("commander=%d submit failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		result = weeklyTaskFailedResult
	}

	response := protobuf.SC_20107{Result: proto.Uint32(result), Next: next}
	return client.SendMessage(20107, &response)
}

func SubmitWeeklyTaskBatch(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20108
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 0, err
	}
	config, err := loadWeeklyTaskConfig()
	if err != nil {
		logger.LogEvent("WeeklyTask", "Config", fmt.Sprintf("commander=%d config load failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		response := protobuf.SC_20109{Result: proto.Uint32(weeklyTaskFailedResult), Pt: proto.Uint32(0), Next: []*protobuf.WEEKLY_TASK_P20{}}
		return client.SendMessage(20109, &response)
	}

	result := weeklyTaskSuccessResult
	pt := uint32(0)
	next := []*protobuf.WEEKLY_TASK_P20{}
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadWeeklyTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, nowUTC())
		if err != nil {
			return err
		}
		nextTasks, ok := submitBatchWeeklyTasks(state, payload.GetId(), config)
		if !ok {
			result = weeklyTaskFailedResult
			pt = state.Pt
			return nil
		}
		if err := orm.SaveWeeklyTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}
		pt = state.Pt
		next = toWeeklyTaskProto(nextTasks)
		return nil
	})
	if err != nil {
		logger.LogEvent("WeeklyTask", "SubmitBatch", fmt.Sprintf("commander=%d submit failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		result = weeklyTaskFailedResult
	}

	response := protobuf.SC_20109{Result: proto.Uint32(result), Pt: proto.Uint32(pt), Next: next}
	return client.SendMessage(20109, &response)
}

func ClaimWeeklyTaskProgressReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_20110
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 0, err
	}
	config, err := loadWeeklyTaskConfig()
	if err != nil {
		logger.LogEvent("WeeklyTask", "Config", fmt.Sprintf("commander=%d config load failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		return client.SendMessage(20111, &protobuf.SC_20111{Result: proto.Uint32(weeklyTaskFailedResult), AwardList: []*protobuf.DROPINFO{}})
	}

	result := weeklyTaskSuccessResult
	awards := []*protobuf.DROPINFO{}
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadWeeklyTaskProgressForUpdateTx(context.Background(), tx, client.Commander.CommanderID, nowUTC())
		if err != nil {
			return err
		}
		drops, ok := claimWeeklyTaskProgressReward(state, config)
		if !ok {
			result = weeklyTaskFailedResult
			return nil
		}
		if err := applyLoveLetterDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}
		if err := orm.SaveWeeklyTaskProgressTx(context.Background(), tx, state); err != nil {
			return err
		}
		awards = dropMapToList(drops)
		return nil
	})
	if err != nil {
		logger.LogEvent("WeeklyTask", "ClaimProgress", fmt.Sprintf("commander=%d claim failed: %v", client.Commander.CommanderID, err), logger.LOG_LEVEL_ERROR)
		result = weeklyTaskFailedResult
		awards = []*protobuf.DROPINFO{}
	}

	response := protobuf.SC_20111{Result: proto.Uint32(result), AwardList: awards}
	return client.SendMessage(20111, &response)
}

func submitSingleWeeklyTask(state *orm.WeeklyTaskProgress, taskID uint32, config *weeklyTaskConfig) (*orm.WeeklyTaskEntry, bool) {
	if taskID == 0 {
		return nil, false
	}
	taskMap := tasksToMap(state.Tasks)
	entry, ok := taskMap[taskID]
	if !ok {
		return nil, false
	}
	template, ok := config.templatesByID[taskID]
	if !ok || len(template.AwardDisplay) < 3 {
		return nil, false
	}
	if entry.Progress < template.TargetNum {
		return nil, false
	}
	delete(taskMap, taskID)
	state.Pt += template.AwardDisplay[2]
	nextTask, hasNext := nextWeeklyTaskTemplate(config.templatesBySub[template.SubType], template, entry.Progress)
	if hasNext {
		taskMap[nextTask.ID] = orm.WeeklyTaskEntry{ID: nextTask.ID, Progress: entry.Progress}
	}
	state.Tasks = mapToTasks(taskMap)
	if hasNext {
		next := taskMap[nextTask.ID]
		return &next, true
	}
	return nil, true
}

func submitBatchWeeklyTasks(state *orm.WeeklyTaskProgress, taskIDs []uint32, config *weeklyTaskConfig) ([]orm.WeeklyTaskEntry, bool) {
	if len(taskIDs) == 0 {
		return nil, false
	}
	seen := make(map[uint32]struct{}, len(taskIDs))
	taskMap := tasksToMap(state.Tasks)
	type claim struct {
		id       uint32
		progress uint32
		template weeklyTaskTemplate
	}
	claims := make([]claim, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID == 0 {
			return nil, false
		}
		if _, ok := seen[taskID]; ok {
			return nil, false
		}
		seen[taskID] = struct{}{}
		entry, ok := taskMap[taskID]
		if !ok {
			return nil, false
		}
		template, ok := config.templatesByID[taskID]
		if !ok || len(template.AwardDisplay) < 3 {
			return nil, false
		}
		if entry.Progress < template.TargetNum {
			return nil, false
		}
		claims = append(claims, claim{id: taskID, progress: entry.Progress, template: template})
	}

	next := make([]orm.WeeklyTaskEntry, 0, len(claims))
	for _, claimed := range claims {
		delete(taskMap, claimed.id)
		state.Pt += claimed.template.AwardDisplay[2]
		template, hasNext := nextWeeklyTaskTemplate(config.templatesBySub[claimed.template.SubType], claimed.template, claimed.progress)
		if hasNext {
			nextEntry := orm.WeeklyTaskEntry{ID: template.ID, Progress: claimed.progress}
			taskMap[nextEntry.ID] = nextEntry
			next = append(next, nextEntry)
		}
	}
	state.Tasks = mapToTasks(taskMap)
	return next, true
}

func claimWeeklyTaskProgressReward(state *orm.WeeklyTaskProgress, config *weeklyTaskConfig) (map[string]*protobuf.DROPINFO, bool) {
	rewardLv := int(state.RewardLv)
	if rewardLv < 0 || rewardLv >= len(config.targets) || rewardLv >= len(config.dropClient) {
		return nil, false
	}
	if state.Pt < config.targets[rewardLv] {
		return nil, false
	}
	drops := make(map[string]*protobuf.DROPINFO)
	for _, entry := range config.dropClient[rewardLv] {
		if len(entry) < 3 {
			return nil, false
		}
		accumulateDrop(drops, entry[0], entry[1], entry[2])
	}
	state.RewardLv++
	return drops, true
}

func nextWeeklyTaskTemplate(templates []weeklyTaskTemplate, current weeklyTaskTemplate, carryProgress uint32) (weeklyTaskTemplate, bool) {
	for _, candidate := range templates {
		if candidate.TargetNum > current.TargetNum {
			return candidate, true
		}
		if candidate.TargetNum == current.TargetNum && candidate.ID > current.ID {
			return candidate, true
		}
	}
	_ = carryProgress
	return weeklyTaskTemplate{}, false
}

func loadWeeklyTaskConfig() (*weeklyTaskConfig, error) {
	entries, err := orm.ListConfigEntries(weeklyTaskTemplateCategory)
	if err != nil {
		return nil, err
	}
	templatesByID := make(map[uint32]weeklyTaskTemplate, len(entries))
	templatesBySub := make(map[uint32][]weeklyTaskTemplate)
	for _, entry := range entries {
		var template weeklyTaskTemplate
		if err := json.Unmarshal(entry.Data, &template); err != nil {
			return nil, err
		}
		templatesByID[template.ID] = template
		templatesBySub[template.SubType] = append(templatesBySub[template.SubType], template)
	}
	for subType := range templatesBySub {
		templates := templatesBySub[subType]
		sort.Slice(templates, func(i, j int) bool {
			if templates[i].TargetNum == templates[j].TargetNum {
				return templates[i].ID < templates[j].ID
			}
			return templates[i].TargetNum < templates[j].TargetNum
		})
		templatesBySub[subType] = templates
	}

	weeklyTarget, err := orm.GetConfigEntry(gamesetCategory, "weekly_target")
	if err != nil {
		return nil, err
	}
	weeklyDropClient, err := orm.GetConfigEntry(gamesetCategory, "weekly_drop_client")
	if err != nil {
		return nil, err
	}

	var targetEntry weeklyGamesetEntry
	if err := json.Unmarshal(weeklyTarget.Data, &targetEntry); err != nil {
		return nil, err
	}
	var targets []uint32
	if err := json.Unmarshal(targetEntry.Description, &targets); err != nil {
		return nil, err
	}

	var dropEntry weeklyGamesetEntry
	if err := json.Unmarshal(weeklyDropClient.Data, &dropEntry); err != nil {
		return nil, err
	}
	var drops [][][]uint32
	if err := json.Unmarshal(dropEntry.Description, &drops); err != nil {
		return nil, err
	}

	return &weeklyTaskConfig{
		templatesByID:  templatesByID,
		templatesBySub: templatesBySub,
		targets:        targets,
		dropClient:     drops,
	}, nil
}

func toWeeklyTaskProto(tasks []orm.WeeklyTaskEntry) []*protobuf.WEEKLY_TASK_P20 {
	if len(tasks) == 0 {
		return []*protobuf.WEEKLY_TASK_P20{}
	}
	result := make([]*protobuf.WEEKLY_TASK_P20, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, &protobuf.WEEKLY_TASK_P20{Id: proto.Uint32(task.ID), Progress: proto.Uint32(task.Progress)})
	}
	return result
}

func toWeeklyTaskPointer(task *orm.WeeklyTaskEntry) *protobuf.WEEKLY_TASK_P20 {
	if task == nil || task.ID == 0 {
		return nil
	}
	return &protobuf.WEEKLY_TASK_P20{Id: proto.Uint32(task.ID), Progress: proto.Uint32(task.Progress)}
}

func tasksToMap(tasks []orm.WeeklyTaskEntry) map[uint32]orm.WeeklyTaskEntry {
	result := make(map[uint32]orm.WeeklyTaskEntry, len(tasks))
	for _, task := range tasks {
		result[task.ID] = task
	}
	return result
}

func mapToTasks(taskMap map[uint32]orm.WeeklyTaskEntry) []orm.WeeklyTaskEntry {
	if len(taskMap) == 0 {
		return []orm.WeeklyTaskEntry{}
	}
	ids := make([]uint32, 0, len(taskMap))
	for id := range taskMap {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	tasks := make([]orm.WeeklyTaskEntry, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, taskMap[id])
	}
	return tasks
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
