package game

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// The back-office's Activities screen: what a commander holds of an activity
// read the way its client page reads it, and the edits of it. Every write
// kicks the commander first (AdminWrite) and runs in one transaction. No
// edit gives rewards: marking a task or a pass level claimed only stores it
// so.

// What AdminActivity.Kind says the activity is shown as.
const (
	AdminActivityTasks    = "tasks"     // commander_activity_tasks (TotalTaskProxy's activities)
	AdminActivityCruise   = "cruise"    // ACTIVITY_TYPE_PT_CRUSING, its pass and weekly tasks
	AdminActivityTaskList = "tasklist"  // ACTIVITY_TYPE_TASK_LIST, tasks handed out by day
	AdminActivityLogin7   = "login7"    // ACTIVITY_TYPE_7DAYSLOGIN
	AdminActivityMonth    = "monthsign" // ACTIVITY_TYPE_MONTHSIGN
	AdminActivityScore    = "score"     // ACTIVITY_TYPE_ISLAND_CHEAT_BAR, the Cheater's Tavern season
)

// activityTypeCheatBar is ACTIVITY_TYPE_ISLAND_CHEAT_BAR: a Cheater's
// Tavern season, data1 the score and data2 the best
// (IslandCheaterTavernMonitor, PlayRoomTools.GetPtScoreIcon).
const activityTypeCheatBar = 801

// AdminActivityTask is a task as the activity's page shows it: its desc
// with progress against target_num, and award_display.
type AdminActivityTask struct {
	ID       uint32
	Desc     string
	Progress uint32
	Target   uint32
	Claimed  bool
	// a task list's task the commander was given (a commander_tasks row);
	// always true for the other activities, whose tasks run unasked
	Held   bool
	Awards [][3]uint32
}

// AdminTaskGroup is a tab of tasks: a cruise week (group_mask, 0 the
// seasonal ones), a task list day, or the activity's only list (0).
type AdminTaskGroup struct {
	ID     uint32
	Locked bool
	Tasks  []AdminActivityTask
}

// AdminCruiseLevel is one level of the cruise pass (battlepass_event_pt
// target / award / award_pay): the pt it needs and both tracks' rewards.
type AdminCruiseLevel struct {
	Level       uint32
	Pt          uint32
	Key         bool // key_point_display
	Free, Paid  [3]uint32
	FreeClaimed bool
	PaidClaimed bool
}

// AdminCruise is the pass: Pt the points earned (the server's counter),
// Shown what the client's page reads (data1), Paid the bonus track bought
// (data2).
type AdminCruise struct {
	Item   uint32 // the pt's virtual item
	Pt     uint32
	Shown  uint32
	Paid   bool
	Levels []AdminCruiseLevel
}

// AdminSigns is a login calendar: Count days signed of Days (each day's
// drops), Last the 7-day login's last sign, Dates the monthly sign-in's
// days of the month signed.
type AdminSigns struct {
	Count   uint32
	Days    [][][3]uint32
	Last    int64
	Year    uint32
	Month   uint32
	Dates   []uint32
	Resigns uint32
}

// AdminScoreBracket is an island_integral_rank row.
type AdminScoreBracket struct {
	ID, Lower, Upper uint32
	Icon             string
}

// AdminScore is a Cheater's Tavern season's score.
type AdminScore struct {
	Score, Best uint32
	Brackets    []AdminScoreBracket
}

// AdminActivity is one activity of a commander, read as its client page
// reads it.
type AdminActivity struct {
	Kind   string
	Type   uint32
	Groups []AdminTaskGroup
	Cruise *AdminCruise
	Signs  *AdminSigns
	Score  *AdminScore
}

// battlepass_event_pt row (CrusingActivity:GetCrusingInfo): the pt each
// level needs, its battlepass_event_award ids on both tracks and the
// levels shown larger.
type battlepassConfig struct {
	Pt        uint32   `json:"pt"`
	Target    []uint32 `json:"target"`
	Award     []uint32 `json:"award"`
	AwardPay  []uint32 `json:"award_pay"`
	KeyPoints []uint32 `json:"key_point_display"`
}

type battlepassAward struct {
	Drop []uint32 `json:"drop_client"`
}

// taskDesc is what a task's page shows of it (task_data_template desc).
type taskDesc struct {
	Desc string `json:"desc"`
}

type islandRankRow struct {
	Icon       string `json:"icon"`
	LowerLimit uint32 `json:"lower_limit"`
	UpperLimit uint32 `json:"upper_limit"`
}

// adminActivityData are the tables only the back-office reads.
type adminActivityData struct {
	passes      map[uint32]battlepassConfig
	passAwards  map[uint32]battlepassAward
	sevenDays   map[uint32]sevenDaySignConfig
	islandRanks map[uint32]islandRankRow
	taskDescs   map[uint32]taskDesc
}

// adminActivityTables reads adminActivityData once.
var adminActivityTables = sync.OnceValues(func() (*adminActivityData, error) {
	tables := &adminActivityData{}
	err := gamedata.LoadAll(
		gamedata.Load("battlepass_event_pt", &tables.passes),
		gamedata.Load("battlepass_event_award", &tables.passAwards),
		gamedata.Load("activity_7_day_sign", &tables.sevenDays),
		gamedata.Load("island_integral_rank", &tables.islandRanks),
		gamedata.Load("task_data_template", &tables.taskDescs),
	)
	return tables, err
})

func drop3(drop []uint32) [3]uint32 {
	var out [3]uint32
	copy(out[:], drop)
	return out
}

func drops3(drops [][]uint32) [][3]uint32 {
	out := make([][3]uint32, 0, len(drops))
	for _, drop := range drops {
		if len(drop) >= 3 {
			out = append(out, drop3(drop))
		}
	}
	return out
}

// adminTask is a task of the activity with its config.
func adminTask(id, progress uint32, claimed, held bool) AdminActivityTask {
	config := taskConfigs[id]
	desc := ""
	if tables, err := adminActivityTables(); err == nil {
		desc = tables.taskDescs[id].Desc
	}
	return AdminActivityTask{ID: id, Desc: desc, Progress: progress, Target: config.TargetNum, Claimed: claimed, Held: held, Awards: drops3(config.awards)}
}

// AdminActivity reads one activity of the commander, false when the
// activity_template doesn't know it.
func (game *Game) AdminActivity(ctx context.Context, commanderID, id uint32) (AdminActivity, bool, error) {
	config, ok := activityConfigs[id]
	if !ok {
		return AdminActivity{}, false, nil
	}
	tables, err := adminActivityTables()
	if err != nil {
		return AdminActivity{}, false, err
	}
	q := game.DB.Queries
	now := time.Now()
	view := AdminActivity{Type: config.Type}
	state, err := activityState(ctx, q, commanderID, config)
	if err != nil {
		return view, false, err
	}
	switch config.Type {
	case activityTypeCruise:
		view.Kind = AdminActivityCruise
		if view.Cruise, err = adminCruise(ctx, q, commanderID, state, tables.passes[id], tables.passAwards); err != nil {
			return view, false, err
		}
		view.Groups, err = adminCruiseGroups(ctx, q, commanderID, config, now)
	case activityTypeTaskList:
		view.Kind = AdminActivityTaskList
		view.Groups, err = adminTaskListDays(ctx, q, commanderID, config, state, now)
	case activityType7DaysLogin:
		view.Kind = AdminActivityLogin7
		signs := &AdminSigns{Count: state.Data1, Last: int64(state.Data2)}
		for _, drop := range sevenDays(tables, config) {
			signs.Days = append(signs.Days, drops3([][]uint32{drop}))
		}
		view.Signs = signs
	case activityTypeMonthSign:
		view.Kind = AdminActivityMonth
		state = monthSignState(state, now)
		signs := &AdminSigns{Count: uint32(len(state.Data1List)), Year: state.Data1, Month: state.Data2, Dates: slices.Sorted(slices.Values(state.Data1List)), Resigns: state.Data3}
		for day := range monthDays(state.Data1, state.Data2) {
			signs.Days = append(signs.Days, drops3([][]uint32{monthSignDrops[state.Data2-1][day]}))
		}
		view.Signs = signs
	case activityTypeCheatBar:
		view.Kind = AdminActivityScore
		score := &AdminScore{Score: state.Data1, Best: state.Data2}
		for _, rankID := range slices.Sorted(maps.Keys(tables.islandRanks)) {
			row := tables.islandRanks[rankID]
			score.Brackets = append(score.Brackets, AdminScoreBracket{ID: rankID, Lower: row.LowerLimit, Upper: row.UpperLimit, Icon: row.Icon})
		}
		view.Score = score
	default:
		view.Kind = AdminActivityTasks
		var group AdminTaskGroup
		group, err = adminActivityTaskGroup(ctx, q, commanderID, id, config.tasks)
		if err == nil && len(group.Tasks) == 0 {
			view.Kind = ""
		}
		view.Groups = []AdminTaskGroup{group}
	}
	return view, true, err
}

// sevenDays are the 7-day login's days: Day7LoginPage shows the first 7
// front_drops (activity_7_day_sign 1 lists an 8th).
func sevenDays(tables *adminActivityData, config activityConfig) [][]uint32 {
	drops := tables.sevenDays[config.ConfigID].FrontDrops
	return drops[:min(len(drops), 7)]
}

// monthDays is how many days month has (TimeMgr:CalcMonthDays).
func monthDays(year, month uint32) int {
	return time.Date(int(year), time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// storedActivityTasks are the commander's rows of an activity's tasks.
func storedActivityTasks(ctx context.Context, q *db.Queries, commanderID, id uint32) (map[uint32]db.AdminActivityListTasksRow, error) {
	rows, err := q.AdminActivityListTasks(ctx, db.AdminActivityListTasksParams{CommanderID: commanderID, ActID: id})
	if err != nil {
		return nil, err
	}
	stored := make(map[uint32]db.AdminActivityListTasksRow, len(rows))
	for _, row := range rows {
		stored[row.TaskID] = row
	}
	return stored, nil
}

// adminActivityTaskGroup is an activity's tasks (config_data), then the
// ones it no longer lists the commander has rows of.
func adminActivityTaskGroup(ctx context.Context, q *db.Queries, commanderID, id uint32, tasks []uint32) (AdminTaskGroup, error) {
	stored, err := storedActivityTasks(ctx, q, commanderID, id)
	if err != nil {
		return AdminTaskGroup{}, err
	}
	group := AdminTaskGroup{}
	seen := map[uint32]bool{}
	for _, task := range tasks {
		if _, ok := taskConfigs[task]; !ok || seen[task] {
			continue
		}
		seen[task] = true
		row := stored[task]
		group.Tasks = append(group.Tasks, adminTask(task, row.Progress, row.Finished, true))
	}
	for _, task := range slices.Sorted(maps.Keys(stored)) {
		if !seen[task] {
			row := stored[task]
			group.Tasks = append(group.Tasks, adminTask(task, row.Progress, row.Finished, true))
		}
	}
	return group, nil
}

// cruiseWeeks is how many weeks of the cruise pass are open
// (GetServerOverWeek of its start, as activityTaskIDs counts them).
func cruiseWeeks(config activityConfig, now time.Time) uint32 {
	elapsed := now.Sub(periodStart(config.Time.Start, week)).Truncate(time.Second)
	return uint32((elapsed + week - 1) / week)
}

// adminCruiseGroups are the pass's battlepass_task_group tabs by
// group_mask (WorldCruiseTaskPage): locked until their week when is_ontime.
func adminCruiseGroups(ctx context.Context, q *db.Queries, commanderID uint32, config activityConfig, now time.Time) ([]AdminTaskGroup, error) {
	stored, err := storedActivityTasks(ctx, q, commanderID, config.ID)
	if err != nil {
		return nil, err
	}
	weeks := cruiseWeeks(config, now)
	var groups []AdminTaskGroup
	for _, groupID := range config.tasks {
		row, ok := battlepassTaskGroups[groupID]
		if !ok {
			continue
		}
		group := AdminTaskGroup{ID: row.GroupMask, Locked: row.IsOntime == 1 && row.GroupMask > weeks}
		for _, chain := range row.TaskGroup {
			for _, task := range chain {
				if _, ok := taskConfigs[task]; ok {
					stored := stored[task]
					group.Tasks = append(group.Tasks, adminTask(task, stored.Progress, stored.Finished, true))
				}
			}
		}
		groups = append(groups, group)
	}
	slices.SortStableFunc(groups, func(a, b AdminTaskGroup) int { return int(a.ID) - int(b.ID) })
	return groups, nil
}

// adminCruise is the pass (CrusingActivity:GetCrusingInfo): data1 the pt
// its page shows, data2 1 once the bonus track is bought, data1_list the
// pt of the base levels claimed. The bonus track's claims (data2_list)
// aren't stored.
func adminCruise(ctx context.Context, q *db.Queries, commanderID uint32, state db.CommanderActivity, pass battlepassConfig, awards map[uint32]battlepassAward) (*AdminCruise, error) {
	pt, err := taskCounter(ctx, q, commanderID, TaskSubCruisePt, 0)
	if err != nil {
		return nil, err
	}
	cruise := &AdminCruise{Item: pass.Pt, Pt: pt, Shown: state.Data1, Paid: state.Data2 == 1}
	for i, target := range pass.Target {
		level := AdminCruiseLevel{Level: uint32(i + 1), Pt: target, Key: slices.Contains(pass.KeyPoints, uint32(i+1)), FreeClaimed: slices.Contains(state.Data1List, target)}
		if i < len(pass.Award) {
			level.Free = drop3(awards[pass.Award[i]].Drop)
		}
		if i < len(pass.AwardPay) {
			level.Paid = drop3(awards[pass.AwardPay[i]].Drop)
		}
		cruise.Levels = append(cruise.Levels, level)
	}
	return cruise, nil
}

// adminTaskListDays are a task list's days (config_data), open from the
// activity's start (data1) a day at a time (openTaskListDays); their tasks
// are commander_tasks once given.
func adminTaskListDays(ctx context.Context, q *db.Queries, commanderID uint32, config activityConfig, state db.CommanderActivity, now time.Time) ([]AdminTaskGroup, error) {
	days := taskListDays(config)
	open := len(days)
	if state.Data1 != 0 {
		start := startOfDay(time.Unix(int64(state.Data1), 0))
		open = min(open, int(startOfDay(now).Sub(start).Hours()/24)+1)
	}
	var groups []AdminTaskGroup
	for i, day := range days {
		group := AdminTaskGroup{ID: uint32(i + 1), Locked: i >= open}
		for _, id := range day {
			if _, ok := taskConfigs[id]; !ok {
				continue
			}
			task, err := q.GetTask(ctx, db.GetTaskParams{CommanderID: commanderID, TaskID: id})
			if errors.Is(err, sql.ErrNoRows) {
				group.Tasks = append(group.Tasks, adminTask(id, 0, false, false))
				continue
			}
			if err != nil {
				return nil, err
			}
			group.Tasks = append(group.Tasks, adminTask(id, task.Progress, task.SubmitTime != 0, true))
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// adminActivityTask finds a task of the activity, as AdminActivity lists it.
func (game *Game) adminActivityTask(ctx context.Context, commanderID, id, taskID uint32) (AdminActivity, AdminActivityTask, error) {
	view, ok, err := game.AdminActivity(ctx, commanderID, id)
	if err != nil {
		return view, AdminActivityTask{}, err
	}
	if !ok {
		return view, AdminActivityTask{}, adminRefused("unknown activity %d", id)
	}
	for _, group := range view.Groups {
		for _, task := range group.Tasks {
			if task.ID == taskID {
				return view, task, nil
			}
		}
	}
	return view, AdminActivityTask{}, adminRefused("activity %d has no task %d", id, taskID)
}

// AdminSetActivityTask sets one of the activity's tasks: its progress, at
// most its target, and whether its reward was claimed (claiming brings the
// progress to the target, as only a finished task is claimed). A task
// list's task not given yet is given.
func (game *Game) AdminSetActivityTask(ctx context.Context, commanderID, id, taskID, progress uint32, claimed bool) (bool, error) {
	view, task, err := game.adminActivityTask(ctx, commanderID, id, taskID)
	if err != nil {
		return false, err
	}
	if progress > task.Target {
		return false, adminRefused("task %d: progress at most %d", taskID, task.Target)
	}
	if claimed {
		progress = task.Target
	}
	now := uint32(time.Now().Unix())
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		if view.Kind != AdminActivityTaskList {
			return tx.AdminActivitySetTask(ctx, db.AdminActivitySetTaskParams{CommanderID: commanderID, ActID: id, TaskID: taskID, Progress: progress, Finished: claimed})
		}
		if !task.Held {
			if _, err := tx.CreateTask(ctx, db.CreateTaskParams{CommanderID: commanderID, TaskID: taskID, Progress: progress, AcceptTime: now}); err != nil {
				return err
			}
		}
		submit := uint32(0)
		if claimed {
			submit = now
		}
		return tx.AdminTaskSet(ctx, db.AdminTaskSetParams{Progress: progress, SubmitTime: submit, CommanderID: commanderID, TaskID: taskID})
	})
}

// AdminResetActivityTask starts one of the activity's tasks over: no
// progress, not claimed (an activity task's row is removed, the game starts
// it from none).
func (game *Game) AdminResetActivityTask(ctx context.Context, commanderID, id, taskID uint32) (bool, error) {
	view, task, err := game.adminActivityTask(ctx, commanderID, id, taskID)
	if err != nil {
		return false, err
	}
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		if view.Kind != AdminActivityTaskList {
			return tx.AdminActivityDeleteTask(ctx, db.AdminActivityDeleteTaskParams{CommanderID: commanderID, ActID: id, TaskID: taskID})
		}
		if !task.Held {
			return nil
		}
		return tx.AdminTaskSet(ctx, db.AdminTaskSetParams{CommanderID: commanderID, TaskID: taskID})
	})
}

// adminActivityOf is activity id's config when it is of kind, and its
// stored state.
func adminActivityOf(ctx context.Context, q *db.Queries, commanderID, id, kind uint32) (activityConfig, db.CommanderActivity, error) {
	config, ok := activityConfigs[id]
	if !ok || config.Type != kind {
		return config, db.CommanderActivity{}, adminRefused("activity %d isn't of type %d", id, kind)
	}
	state, err := activityState(ctx, q, commanderID, config)
	return config, state, err
}

// AdminSetCruise sets the pass's pt (nil keeps it) and whether the bonus
// track is bought (nil keeps it). The pt are the server's counter, which
// the tasks counting them follow, and the page's data1; base levels
// claimed above the new pt are unclaimed.
func (game *Game) AdminSetCruise(ctx context.Context, commanderID, id uint32, pt *uint32, paid *bool) (bool, error) {
	tables, err := adminActivityTables()
	if err != nil {
		return false, err
	}
	pass := tables.passes[id]
	if pt != nil && len(pass.Target) > 0 && *pt > pass.Target[len(pass.Target)-1] {
		return false, adminRefused("pt at most %d", pass.Target[len(pass.Target)-1])
	}
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		_, state, err := adminActivityOf(ctx, tx.Queries, commanderID, id, activityTypeCruise)
		if err != nil {
			return err
		}
		if pt != nil {
			tx.event(TaskEvent{SubType: TaskSubCruisePt, Count: *pt})
			state.Data1 = *pt
			state.Data1List = slices.DeleteFunc(state.Data1List, func(target uint32) bool { return target > *pt })
		}
		if paid != nil {
			state.Data2 = 0
			if *paid {
				state.Data2 = 1
			}
		}
		return tx.SaveActivity(ctx, db.SaveActivityParams(state))
	})
}

// AdminSetCruiseAward marks a base level of the pass claimed or not
// (data1_list, CrusingActivity:AddAwardRecord): level 0 is every level the
// pt reach. A level is claimed once its pt are reached.
func (game *Game) AdminSetCruiseAward(ctx context.Context, commanderID, id, level uint32, claimed bool) (bool, error) {
	tables, err := adminActivityTables()
	if err != nil {
		return false, err
	}
	pass := tables.passes[id]
	if int(level) > len(pass.Target) {
		return false, adminRefused("the pass has %d levels", len(pass.Target))
	}
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		_, state, err := adminActivityOf(ctx, tx.Queries, commanderID, id, activityTypeCruise)
		if err != nil {
			return err
		}
		var targets []uint32
		if level == 0 {
			for _, target := range pass.Target {
				if target <= state.Data1 || !claimed {
					targets = append(targets, target)
				}
			}
		} else {
			targets = []uint32{pass.Target[level-1]}
			if claimed && targets[0] > state.Data1 {
				return adminRefused("level %d needs %d pt", level, targets[0])
			}
		}
		list := slices.DeleteFunc(slices.Clone(state.Data1List), func(target uint32) bool { return slices.Contains(targets, target) })
		if claimed {
			list = append(list, targets...)
		}
		state.Data1List = slices.Sorted(slices.Values(list))
		return tx.SaveActivity(ctx, db.SaveActivityParams(state))
	})
}

// AdminSetSigns sets how many days of a login calendar are signed. The
// 7-day login counts them in data1; one unsigned lets the commander sign
// again today (data2, the last sign, cleared). The monthly sign-in keeps
// the days of the month in data1_list: days are dropped from the latest,
// and added from the 1st, today last so it can still be signed.
func (game *Game) AdminSetSigns(ctx context.Context, commanderID, id, count uint32) (bool, error) {
	tables, err := adminActivityTables()
	if err != nil {
		return false, err
	}
	now := time.Now()
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		config, ok := activityConfigs[id]
		if !ok {
			return adminRefused("unknown activity %d", id)
		}
		state, err := activityState(ctx, tx.Queries, commanderID, config)
		if err != nil {
			return err
		}
		switch config.Type {
		case activityType7DaysLogin:
			days := uint32(len(sevenDays(tables, config)))
			if count > days {
				return adminRefused("at most %d days", days)
			}
			if count < state.Data1 {
				state.Data2 = 0
			}
			state.Data1 = count
		case activityTypeMonthSign:
			state = monthSignState(state, now)
			days := monthDays(state.Data1, state.Data2)
			if int(count) > days {
				return adminRefused("at most %d days", days)
			}
			list := slices.Sorted(slices.Values(state.Data1List))
			if int(count) < len(list) {
				list = list[:count]
			}
			today := uint32(now.In(gamedata.Zone()).Day())
			for _, pass := range []bool{false, true} {
				for day := uint32(1); day <= uint32(days) && len(list) < int(count); day++ {
					if (day == today) == pass && !slices.Contains(list, day) {
						list = append(list, day)
					}
				}
			}
			state.Data1List = slices.Sorted(slices.Values(list))
		default:
			return adminRefused("activity %d isn't a login calendar", id)
		}
		return tx.SaveActivity(ctx, db.SaveActivityParams(state))
	})
}

// AdminSetScore sets a Cheater's Tavern season's score and best, kept as
// addPlayRoomScore keeps them: the season's play room score of each game
// type it is the season of, and the activity's data1 / data2. The best is
// never under the score.
func (game *Game) AdminSetScore(ctx context.Context, commanderID, id, score, best uint32) (bool, error) {
	best = max(best, score)
	return game.AdminWrite(ctx, commanderID, func(tx *Tx) error {
		config, state, err := adminActivityOf(ctx, tx.Queries, commanderID, id, activityTypeCheatBar)
		if err != nil {
			return err
		}
		for _, gameType := range slices.Sorted(maps.Keys(modeRooms)) {
			if modeRooms[gameType].ActivityType != config.Type {
				continue
			}
			if err := tx.SetPlayRoomScore(ctx, db.SetPlayRoomScoreParams{CommanderID: commanderID, GameType: gameType, Season: id, Score: score, MaxScore: best}); err != nil {
				return err
			}
		}
		state.Data1, state.Data2 = score, best
		return tx.SaveActivity(ctx, db.SaveActivityParams(state))
	})
}
