package game

import (
	"cmp"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// task_data_template row (TaskProxy, ActivityTaskProxy tasks)
type taskConfig struct {
	SubType      uint32          `json:"sub_type"`
	TargetID     json.RawMessage `json:"target_id"`
	TargetNum    uint32          `json:"target_num"`
	AwardDisplay json.RawMessage `json:"award_display"`
	AwardChoice  json.RawMessage `json:"award_choice"`
	NextTask     json.RawMessage `json:"next_task"`
	AutoCommit   uint32          `json:"auto_commit"`

	awards [][]uint32 // award_display
	target taskTarget // target_id
	next   []uint32   // next_task
}

// weekly_task_template row (WeekPtTask)
type weeklyTaskConfig struct {
	ID       uint32          `json:"id"`
	Level    uint32          `json:"level"`
	SubType  uint32          `json:"sub_type"`
	TargetID json.RawMessage `json:"target_id"`

	target taskTarget // target_id
}

// activity_template row, only what task activities need
type activityConfig struct {
	ID         uint32          `json:"id"`
	Type       uint32          `json:"type"`
	ConfigID   uint32          `json:"config_id"`
	ConfigData json.RawMessage `json:"config_data"`
	Time       json.RawMessage `json:"time"`

	// for the activities with tasks (taskActivityTypes, task lists)
	tasks  []uint32       // config_data
	window activityWindow // time
}

// activityWindow is an activity_template.time: "always", or
// ["timer", [[y,m,d],[h,m,s]], [[y,m,d],[h,m,s]]] in server time.
type activityWindow struct {
	always     bool
	start, end time.Time // zero without a timer
}

func parseActivityWindow(raw json.RawMessage) activityWindow {
	var always string
	if json.Unmarshal(raw, &always) == nil {
		return activityWindow{always: always == "always"}
	}
	var timer []json.RawMessage
	if json.Unmarshal(raw, &timer) != nil || len(timer) < 3 {
		return activityWindow{}
	}
	start, okStart := configTime(timer[1])
	end, okEnd := configTime(timer[2])
	if !okStart || !okEnd {
		return activityWindow{}
	}
	return activityWindow{start: start, end: end}
}

func configTime(raw json.RawMessage) (time.Time, bool) {
	var parts [2][3]int
	if json.Unmarshal(raw, &parts) != nil {
		return time.Time{}, false
	}
	return time.Date(parts[0][0], time.Month(parts[0][1]), parts[0][2], parts[1][0], parts[1][1], parts[1][2], 0, serverZone), true
}

// open is whether the activity runs at now.
func (window activityWindow) open(now time.Time) bool {
	if window.always {
		return true
	}
	return !window.start.IsZero() && !now.Before(window.start) && now.Before(window.end)
}

// battlepass_task_group row: the cruise pass tasks of one week (group_mask),
// or always open when is_ontime is 0
type battlepassTaskGroup struct {
	GroupMask uint32     `json:"group_mask"`
	IsOntime  uint32     `json:"is_ontime"`
	TaskGroup [][]uint32 `json:"task_group"`

	tasks []uint32 // task_group flattened, backwards (as the capture lists them)
}

// tutorial_handbook_task row: a commander manual page; task_list holds
// {tasks, tasks to finish before they are given} pairs
type handbookPageConfig struct {
	ID       uint32       `json:"id"`
	TaskList [][][]uint32 `json:"task_list"`
	Unlock   []uint32     `json:"unlock"`
}

var (
	taskConfigs          map[uint32]taskConfig
	weeklyTaskConfigs    map[uint32]weeklyTaskConfig
	activityConfigs      map[uint32]activityConfig
	battlepassTaskGroups map[uint32]battlepassTaskGroup
	handbookPageConfigs  map[uint32]handbookPageConfig

	// taskActivities are the activity_template rows of taskActivityTypes, by id
	taskActivities []activityConfig
	// task_data_template / weekly_task_template ids by sub_type
	taskIDsBySubType       map[uint32][]uint32
	weeklyTaskIDsBySubType map[uint32][]uint32

	// handbookTaskPages are the commander manual pages of each task, by page id
	handbookTaskPages map[uint32][]handbookTaskPage
	// handbookUnlockTasks are the tasks that unlock pages
	// (tutorial_handbook_task.unlock), sorted
	handbookUnlockTasks []uint32
)

// handbookTaskPage is a commander manual page a task belongs to: as one of
// its tasks (task_list[i][0]), or as a task to finish to unlock the page or
// its tasks (unlock, task_list[i][1]).
type handbookTaskPage struct {
	page     uint32
	pageTask bool
}

// loadTaskData reads the tasks, weekly missions, activities, cruise pass
// task groups and commander manual pages.
func loadTaskData() error {
	err := loadAll(
		loadTable("task_data_template", &taskConfigs),
		loadTable("weekly_task_template", &weeklyTaskConfigs),
		loadTable("activity_template", &activityConfigs),
		loadTable("battlepass_task_group", &battlepassTaskGroups),
		loadTable("tutorial_handbook_task", &handbookPageConfigs),
	)
	if err != nil {
		return err
	}
	taskIDsBySubType = map[uint32][]uint32{}
	for id, config := range taskConfigs {
		// a task whose award_display isn't a drop list (one is a Lua
		// string) is left out like any row that doesn't fit
		if json.Unmarshal(config.AwardDisplay, &config.awards) != nil {
			delete(taskConfigs, id)
			continue
		}
		config.target = parseTaskTarget(config.TargetID)
		config.next = parseIDList(config.NextTask)
		taskConfigs[id] = config
		taskIDsBySubType[config.SubType] = append(taskIDsBySubType[config.SubType], id)
	}
	weeklyTaskIDsBySubType = map[uint32][]uint32{}
	for id, config := range weeklyTaskConfigs {
		config.target = parseTaskTarget(config.TargetID)
		weeklyTaskConfigs[id] = config
		weeklyTaskIDsBySubType[config.SubType] = append(weeklyTaskIDsBySubType[config.SubType], id)
	}
	taskActivities = nil
	for id, config := range activityConfigs {
		if !taskActivityTypes[config.Type] && config.Type != activityTypeTaskList {
			continue
		}
		config.tasks = parseIDList(config.ConfigData)
		config.window = parseActivityWindow(config.Time)
		activityConfigs[id] = config
		if taskActivityTypes[config.Type] {
			taskActivities = append(taskActivities, config)
		}
	}
	slices.SortFunc(taskActivities, func(a, b activityConfig) int { return cmp.Compare(a.ID, b.ID) })
	indexHandbookTasks()
	for id, group := range battlepassTaskGroups {
		for _, chain := range group.TaskGroup {
			group.tasks = append(group.tasks, chain...)
		}
		slices.Reverse(group.tasks)
		battlepassTaskGroups[id] = group
	}
	return nil
}

// indexHandbookTasks builds handbookTaskPages and handbookUnlockTasks.
func indexHandbookTasks() {
	pageTasks := map[uint32]map[uint32]bool{} // task -> page -> is a page task
	add := func(task, page uint32, pageTask bool) {
		if pageTasks[task] == nil {
			pageTasks[task] = map[uint32]bool{}
		}
		pageTasks[task][page] = pageTasks[task][page] || pageTask
	}
	handbookUnlockTasks = nil
	for id, page := range handbookPageConfigs {
		for _, task := range page.Unlock {
			add(task, id, false)
			if !slices.Contains(handbookUnlockTasks, task) {
				handbookUnlockTasks = append(handbookUnlockTasks, task)
			}
		}
		for _, entry := range page.TaskList {
			if len(entry) > 0 {
				for _, task := range entry[0] {
					add(task, id, true)
				}
			}
			if len(entry) > 1 {
				for _, task := range entry[1] {
					add(task, id, false)
				}
			}
		}
	}
	slices.Sort(handbookUnlockTasks)
	handbookTaskPages = make(map[uint32][]handbookTaskPage, len(pageTasks))
	for task, pages := range pageTasks {
		for _, page := range slices.Sorted(maps.Keys(pages)) {
			handbookTaskPages[task] = append(handbookTaskPages[task], handbookTaskPage{page: page, pageTask: pages[page]})
		}
	}
}

// taskTarget is a task's target_id: "0" / "" / [] for any target, an id, a
// list of ids, or a [">=", n] threshold.
type taskTarget struct {
	any bool
	ids []uint32
	min *uint32
}

func parseTaskTarget(raw json.RawMessage) taskTarget {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "[") {
			return parseTaskTarget(json.RawMessage(text))
		}
		id, err := strconv.ParseUint(text, 10, 32)
		if err != nil || id == 0 {
			return taskTarget{any: true}
		}
		return taskTarget{ids: []uint32{uint32(id)}}
	}
	var number uint32
	if json.Unmarshal(raw, &number) == nil {
		if number == 0 {
			return taskTarget{any: true}
		}
		return taskTarget{ids: []uint32{number}}
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil || len(list) == 0 {
		return taskTarget{any: true}
	}
	if json.Unmarshal(list[0], &text) == nil && len(list) == 2 {
		if json.Unmarshal(list[1], &number) == nil {
			return taskTarget{min: &number}
		}
	}
	target := taskTarget{}
	for _, item := range list {
		if json.Unmarshal(item, &number) == nil {
			target.ids = append(target.ids, number)
		}
	}
	if len(target.ids) == 0 {
		target.any = true
	}
	return target
}

func (target taskTarget) matches(id uint32) bool {
	switch {
	case target.any:
		return true
	case target.min != nil:
		return id >= *target.min
	}
	for _, want := range target.ids {
		if want == id {
			return true
		}
	}
	return false
}

// parseIDList reads next_task / config_data style fields: "0", "2",
// "[5,3000]", 2, [5,3000] or nested lists, flattened.
func parseIDList(raw json.RawMessage) []uint32 {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "[") {
			return parseIDList(json.RawMessage(text))
		}
		id, err := strconv.ParseUint(text, 10, 32)
		if err != nil || id == 0 {
			return nil
		}
		return []uint32{uint32(id)}
	}
	var number uint32
	if json.Unmarshal(raw, &number) == nil {
		if number == 0 {
			return nil
		}
		return []uint32{number}
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var ids []uint32
	for _, item := range list {
		ids = append(ids, parseIDList(item)...)
	}
	return ids
}
