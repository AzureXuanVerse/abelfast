package game

import (
	"encoding/json"
	"strconv"
	"strings"
)

// task_data_template row (TaskProxy, ActivityTaskProxy tasks)
type taskConfig struct {
	ID           uint32          `json:"id"`
	Type         uint32          `json:"type"`
	SubType      uint32          `json:"sub_type"`
	TargetID     json.RawMessage `json:"target_id"`
	TargetNum    uint32          `json:"target_num"`
	AwardDisplay [][]uint32      `json:"award_display"`
	AwardChoice  json.RawMessage `json:"award_choice"`
	NextTask     json.RawMessage `json:"next_task"`
	AutoCommit   uint32          `json:"auto_commit"`
}

// weekly_task_template row (WeekPtTask)
type weeklyTaskConfig struct {
	ID           uint32          `json:"id"`
	Level        uint32          `json:"level"`
	SubType      uint32          `json:"sub_type"`
	TargetID     json.RawMessage `json:"target_id"`
	TargetNum    uint32          `json:"target_num"`
	AwardDisplay []uint32        `json:"award_display"`
}

// activity_template row, only what task activities need
type activityConfig struct {
	ID         uint32          `json:"id"`
	Type       uint32          `json:"type"`
	ConfigID   uint32          `json:"config_id"`
	ConfigData json.RawMessage `json:"config_data"`
	Time       json.RawMessage `json:"time"`
}

// battlepass_task_group row: the cruise pass tasks of one week (group_mask),
// or always open when is_ontime is 0
type battlepassTaskGroup struct {
	ID        uint32     `json:"id"`
	GroupMask uint32     `json:"group_mask"`
	IsOntime  uint32     `json:"is_ontime"`
	TaskGroup [][]uint32 `json:"task_group"`
}

// tutorial_handbook_task row: a commander manual page; task_list holds
// {tasks, tasks to finish before they are given} pairs
type handbookPageConfig struct {
	ID       uint32       `json:"id"`
	TaskList [][][]uint32 `json:"task_list"`
	Unlock   []uint32     `json:"unlock"`
}

var (
	taskConfigs          = &configTable[taskConfig]{name: "task_data_template"}
	weeklyTaskConfigs    = &configTable[weeklyTaskConfig]{name: "weekly_task_template"}
	activityConfigs      = &configTable[activityConfig]{name: "activity_template"}
	battlepassTaskGroups = &configTable[battlepassTaskGroup]{name: "battlepass_task_group"}
	handbookPageConfigs  = &configTable[handbookPageConfig]{name: "tutorial_handbook_task"}
)

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
