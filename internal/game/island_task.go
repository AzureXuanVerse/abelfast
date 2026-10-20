package game

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's tasks, development season and achievements (IslandTaskAgency,
// IslandSeasonAgency, IslandAchievementAgency, mod/island/command/{task,
// season,achievement}). Built from the Lua and the island_task*,
// island_season and island_achievement tables alone (no capture).
//
// Tasks are accepted by the client itself once their conditions are met
// (IslandTaskAgency.TryAcceptAutoTasks, on approach...), the daily and
// weekly ones only from the offers the server lists (task_list_random,
// refreshed by CS_21030). A task target's progress is kept three ways
// (IslandTaskTargetType): the client counts the "runtime" ones from the
// island itself (items owned, level...), reports the "client" ones
// (CS_21036: talking to someone, reaching a place...), and the server
// counts the rest from what happens (orders, production...: islandTaskEvent,
// pushed in SC_21040).

// IslandTaskType: what a task is
const (
	islandTaskMain   = 1
	islandTaskDaily  = 3
	islandTaskWeekly = 4
)

// IslandTaskTargetType values the server counts or checks itself
const (
	IslandTargetRecycle  = 4  // hand over items (taken at submission)
	islandTargetObtain   = 5  // island items obtained
	IslandTargetTaskType = 23 // tasks of a type submitted
)

// IslandTaskTargetType.GetClientTypes: targets the client reports (CS_21036)
var islandClientTargets = []uint32{1, 2, 36, 43}

// IslandTaskTargetType.GetRuntimeTypes: targets the client counts from the
// island itself; the server takes its word for them at submission.
var islandRuntimeTargets = []uint32{4, 8, 9, 10, 11, 15, 16, 17, 18, 19, 20, 21, 22, 24, 30, 34, 35, 44, 45, 39, 54, 55, 56, 1001, 1002}

// islandTaskConfig is an island_task row.
type islandTaskConfig struct {
	Type            uint32          `json:"type"`
	TriggerType     uint32          `json:"trigger_type"`
	UnlockCondition json.RawMessage `json:"unlock_condition"`
	UnlockTime      gamedata.Window `json:"unlock_time"`
	Targets         []uint32        `json:"target_id"`
	RewardShow      json.RawMessage `json:"reward_show"`
	RewardExp       uint32          `json:"reward_exp"`
	CountOffset     uint32          `json:"count_offset"`
}

// islandTaskTargetConfig is an island_task_target row: what to do how
// many times; target_param lists the ids that count (none: anything).
type islandTaskTargetConfig struct {
	Type   uint32          `json:"type"`
	Param  json.RawMessage `json:"target_param"`
	Num    uint32          `json:"target_num"`
	params []uint32
}

// islandAchievementConfig is an island_achievement row: a record
// (target_type, target_value1) to reach target_num.
type islandAchievementConfig struct {
	Award      [][]uint32 `json:"award"`
	TargetType uint32     `json:"target_type"`
	TargetArg  uint32     `json:"target_value1"`
	TargetNum  uint32     `json:"target_num"`
}

// islandSeasonConfig is an island_season row: when it runs and the rewards
// of its point thresholds.
type islandSeasonConfig struct {
	Time    [2][2][3]int `json:"time"`
	Target  []uint32     `json:"target"`
	Rewards [][]uint32   `json:"ptaward_display"`
}

var (
	islandTasks        map[uint32]islandTaskConfig
	islandTaskTargets  map[uint32]islandTaskTargetConfig
	islandAchievements map[uint32]islandAchievementConfig
	islandSeasons      map[uint32]islandSeasonConfig
	// achievement records the server keeps, by type then argument
	islandAchievementRecords map[uint32]map[uint32]bool
	islandTaskRules          struct {
		dailyCount  uint32 // island_daily_task_num: daily offers a day
		weeklyCount uint32 // task_weekly's length: weekly offers a week
	}
)

func (game *Game) registerIslandTasks() {
	packets.Handle(game.handleIslandTaskRefresh)
	packets.Handle(game.handleIslandTaskAccept)
	packets.Handle(game.handleIslandTaskTrace)
	packets.Handle(game.handleIslandTaskUpdate)
	packets.Handle(game.handleIslandTaskSubmit)
	packets.Handle(game.handleIslandTaskSubmitAll)
	packets.Handle(game.handleIslandSeasonPtAward)
	packets.Handle(game.handleIslandSeasonReset)
	packets.Handle(game.handleIslandAchievementAward)
	packets.Handle(game.handleIslandAchievementRecords)
	game.registerIslandBook()
}

func loadIslandTaskData() error {
	err := gamedata.LoadAll(
		gamedata.Load("island_task", &islandTasks),
		gamedata.Load("island_task_target", &islandTaskTargets),
		gamedata.Load("island_achievement", &islandAchievements),
		gamedata.Load("island_season", &islandSeasons),
		loadIslandBookData,
	)
	if err != nil {
		return err
	}
	for id, target := range islandTaskTargets {
		target.params = gamedata.IDs(target.Param)
		islandTaskTargets[id] = target
	}
	for id, task := range islandTasks {
		for _, target := range task.Targets {
			if _, ok := islandTaskTargets[target]; !ok {
				return gamedata.Missing("island_task_target", "target %d of task %d", target, id)
			}
		}
	}
	islandAchievementRecords = map[uint32]map[uint32]bool{}
	for _, achievement := range islandAchievements {
		if islandAchievementRecords[achievement.TargetType] == nil {
			islandAchievementRecords[achievement.TargetType] = map[uint32]bool{}
		}
		islandAchievementRecords[achievement.TargetType][achievement.TargetArg] = true
	}
	islandTaskRules.dailyCount = islandSetInt("island_daily_task_num")
	var weekly [][]uint32
	if err := islandSetList("task_weekly", &weekly); err != nil {
		return err
	}
	islandTaskRules.weeklyCount = uint32(len(weekly))
	return nil
}

// islandTaskDrops are a task's rewards (reward_show), without the island
// experience (reward_exp, added apart: IslandDropHelper.AddItems' exp).
func islandTaskDrops(task islandTaskConfig) []*protobuf.DROPINFO {
	var rewards [][]uint32
	if json.Unmarshal(task.RewardShow, &rewards) != nil {
		return nil
	}
	return configDrops(rewards)
}

// islandConditionMet checks an island_task unlock condition the server can
// see (IslandTaskConditionType); the others count as met.
func islandConditionMet(state *islandState, condition []uint32) bool {
	if len(condition) < 2 {
		return true
	}
	info := state.public().TaskInfo
	switch condition[0] {
	case 1: // GOT_ACHIEVEMENT
		return slices.Contains(state.private().AchievementSys.FinishList, condition[1])
	case 2: // FINISH_TASK
		return slices.Contains(info.TaskIdListFinish, condition[1])
	case 3: // EXIST_ABILITY
		return slices.Contains(state.public().AbilityList, condition[1])
	case 4: // MUTEX_TASK
		return !slices.Contains(info.TaskIdListFinish, condition[1]) && !slices.ContainsFunc(info.TaskList, func(task *protobuf.PB_TASK) bool { return task.GetId() == condition[1] })
	}
	return true
}

// islandTaskUnlocked tells whether the commander may take a task now.
func islandTaskUnlocked(state *islandState, task islandTaskConfig, now time.Time) bool {
	if !task.UnlockTime.Open(now) {
		return false
	}
	var conditions [][]uint32
	if json.Unmarshal(task.UnlockCondition, &conditions) != nil {
		return true
	}
	for _, condition := range conditions {
		if !islandConditionMet(state, condition) {
			return false
		}
	}
	return true
}

// islandTaskExtra is the task state the server keeps besides PB_ISLAND_TASK:
// when the daily and weekly offers were last drawn.
type islandTaskExtra struct {
	DailyRefresh  int64 `json:"daily_refresh"`
	WeeklyRefresh int64 `json:"weekly_refresh"`
}

// refreshIslandTasks draws new daily (each server day) and weekly (each
// Monday) offers when due: the running and submitted ones of the period
// that ended are dropped. It tells what it removed. GUESS: no table says
// how the offers are drawn; island_daily_task_num daily tasks and as many
// weekly tasks as task_weekly lists, at random among those whose
// conditions are met.
func refreshIslandTasks(state *islandState, now time.Time) (removed, removedFinished []uint32, err error) {
	var extra islandTaskExtra
	if _, err := state.getExtra("tasks", &extra); err != nil {
		return nil, nil, err
	}
	day, week := startOfDay(now).Unix(), periodStart(now, week).Unix()
	info := state.public().TaskInfo
	refresh := func(kind, count uint32) {
		drop := func(id uint32) bool { return islandTasks[id].Type == kind }
		for _, task := range info.TaskList {
			if drop(task.GetId()) {
				removed = append(removed, task.GetId())
			}
		}
		info.TaskList = slices.DeleteFunc(info.TaskList, func(task *protobuf.PB_TASK) bool { return drop(task.GetId()) })
		for _, id := range info.TaskIdListFinish {
			if drop(id) {
				removedFinished = append(removedFinished, id)
			}
		}
		info.TaskIdListFinish = slices.DeleteFunc(info.TaskIdListFinish, drop)
		info.TaskListRandom = slices.DeleteFunc(info.TaskListRandom, func(task *protobuf.PB_TASK_RANDOM) bool { return drop(task.GetTaskId()) })
		var pool []uint32
		for id, task := range islandTasks {
			if task.Type == kind && islandTaskUnlocked(state, task, now) {
				pool = append(pool, id)
			}
		}
		slices.Sort(pool)
		rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		for _, id := range pool[:min(int(count), len(pool))] {
			info.TaskListRandom = append(info.TaskListRandom, &protobuf.PB_TASK_RANDOM{TaskId: proto.Uint32(id), Timestamp: proto.Uint32(uint32(now.Unix()))})
		}
	}
	changed := false
	if extra.DailyRefresh != day {
		refresh(islandTaskDaily, islandTaskRules.dailyCount)
		extra.DailyRefresh, changed = day, true
	}
	if extra.WeeklyRefresh != week {
		refresh(islandTaskWeekly, islandTaskRules.weeklyCount)
		info.WeekDailyTaskNum = proto.Uint32(0)
		extra.WeeklyRefresh, changed = week, true
	}
	if changed {
		return removed, removedFinished, state.setExtra("tasks", extra)
	}
	return nil, nil, nil
}

// islandTask is the running task id, nil if not running.
func islandTask(state *islandState, id uint32) *protobuf.PB_TASK {
	for _, task := range state.public().TaskInfo.TaskList {
		if task.GetId() == id {
			return task
		}
	}
	return nil
}

// islandTaskProgress is a running task's progress on target.
func islandTaskProgress(task *protobuf.PB_TASK, target uint32) *protobuf.PB_TASK_PROCESS {
	for _, process := range task.ProcessList {
		if process.GetTargetId() == target {
			return process
		}
	}
	process := &protobuf.PB_TASK_PROCESS{TargetId: proto.Uint32(target), TargetCount: proto.Uint32(0)}
	task.ProcessList = append(task.ProcessList, process)
	return process
}

// islandTaskEvent counts count of something the server sees happen (an
// IslandTaskTargetType: an order delivered, an item made...) for the
// running tasks with such a target, keys being what it was about (an item,
// a production site...: a target with target_param counts only its ids).
// The updated tasks are pushed (SC_21040).
func (tx *Tx) islandTaskEvent(kind, count uint32, keys ...uint32) error {
	state, err := tx.island()
	if err != nil {
		return err
	}
	var changed []*protobuf.PB_TASK
	for _, task := range state.public().TaskInfo.TaskList {
		updated := false
		for _, target := range islandTasks[task.GetId()].Targets {
			config := islandTaskTargets[target]
			if config.Type != kind {
				continue
			}
			if len(config.params) > 0 && !slices.ContainsFunc(keys, func(key uint32) bool { return slices.Contains(config.params, key) }) {
				continue
			}
			process := islandTaskProgress(task, target)
			if process.GetTargetCount() >= config.Num {
				continue
			}
			process.TargetCount = proto.Uint32(min(config.Num, process.GetTargetCount()+count))
			updated = true
		}
		if updated {
			changed = append(changed, proto.Clone(task).(*protobuf.PB_TASK))
		}
	}
	if len(changed) > 0 {
		tx.push(&protobuf.SC_21040{TaskList: changed})
	}
	return nil
}

// handleIslandTaskRefresh answers GetIslandTaskRandomRefreshCommand
// (CS_21030, sent at each server day change while on the island): the
// daily and weekly offers drawn again when due (refreshIslandTasks), the
// tasks dropped, and always the offers standing (the client rebuilds its
// future tasks from them).
func (game *Game) handleIslandTaskRefresh(client *connection.Client, req *protobuf.CS_21030) error {
	reply := &protobuf.SC_21031{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		reply.RemoveTaskList, reply.RemoveTaskFinish, err = refreshIslandTasks(state, serverNow())
		reply.TaskListRandom = state.public().TaskInfo.TaskListRandom
		return err
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandTaskAccept answers IslandAcceptTaskCommand (CS_21032): the
// tasks the client found it can take, running from now with no progress.
// One already running or submitted, unknown, not open, not offered (daily
// and weekly) or whose conditions aren't met is left out.
func (game *Game) handleIslandTaskAccept(client *connection.Client, req *protobuf.CS_21032) error {
	reply := &protobuf.SC_21033{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := serverNow()
		info := state.public().TaskInfo
		for _, id := range req.GetTaskIdList() {
			task, ok := islandTasks[id]
			if !ok || islandTask(state, id) != nil || slices.Contains(info.TaskIdListFinish, id) || !islandTaskUnlocked(state, task, now) {
				continue
			}
			if (task.Type == islandTaskDaily || task.Type == islandTaskWeekly) && !slices.ContainsFunc(info.TaskListRandom, func(offer *protobuf.PB_TASK_RANDOM) bool { return offer.GetTaskId() == id }) {
				continue
			}
			running := &protobuf.PB_TASK{Id: proto.Uint32(id), Timestamp: proto.Uint32(uint32(now.Unix()))}
			for _, target := range task.Targets {
				islandTaskProgress(running, target)
			}
			info.TaskList = append(info.TaskList, running)
			reply.TaskList = append(reply.TaskList, running)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}

// handleIslandTaskTrace answers SetIslandTraceTaskCommand (CS_21034): the
// task followed on screen (focus_id), 0 for none.
func (game *Game) handleIslandTaskTrace(client *connection.Client, req *protobuf.CS_21034) error {
	refusal := error(nil)
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		if req.GetTaskId() != 0 && islandTask(state, req.GetTaskId()) == nil {
			refusal = errRefused
			return nil
		}
		state.public().TaskInfo.FocusId = proto.Uint32(req.GetTaskId())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21035{Result: result(refusal)})
}

// handleIslandTaskUpdate answers IslandUpdateTaskCommand (CS_21036, sent
// with task_id 0 for each running target the client counts itself:
// talking to someone, reaching a place, an action, a photo): target_count
// more on that target in every running task, up to its target_num.
func (game *Game) handleIslandTaskUpdate(client *connection.Client, req *protobuf.CS_21036) error {
	reply := &protobuf.SC_21037{}
	target, ok := islandTaskTargets[req.GetTargetId()]
	if !ok || !slices.Contains(islandClientTargets, target.Type) {
		reply.Result = result(errRefused)
		return client.Send(islandStamped(reply))
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		for _, task := range state.public().TaskInfo.TaskList {
			if req.GetTaskId() != 0 && task.GetId() != req.GetTaskId() || !slices.Contains(islandTasks[task.GetId()].Targets, req.GetTargetId()) {
				continue
			}
			process := islandTaskProgress(task, req.GetTargetId())
			process.TargetCount = proto.Uint32(min(target.Num, process.GetTargetCount()+req.GetTargetCount()))
			reply.TaskList = append(reply.TaskList, task)
		}
		return nil
	})
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// submitIslandTask finishes a running task (IslandSubmitTaskCommand): its
// targets the server counts must be reached (the runtime ones are the
// client's word), the items it asks for are handed over, the rewards given
// and the island experience added; it counts as a task of its type
// submitted (IslandTaskTargetType.TASK_TYPE, the achievements'
// FINISH_TYPE_TASK). The drops are returned for the reply.
func submitIslandTask(tx *Tx, state *islandState, id uint32) ([]*protobuf.DROPINFO, error) {
	running := islandTask(state, id)
	if running == nil {
		return nil, errRefused
	}
	task := islandTasks[id]
	for _, target := range task.Targets {
		config := islandTaskTargets[target]
		switch {
		case config.Type == IslandTargetRecycle && len(config.params) > 0:
			if err := state.takeIslandItem(config.params[0], config.Num); err != nil {
				return nil, err
			}
		case slices.Contains(islandRuntimeTargets, config.Type):
		case islandTaskProgress(running, target).GetTargetCount() < config.Num:
			return nil, errRefused
		}
	}
	info := state.public().TaskInfo
	info.TaskList = slices.DeleteFunc(info.TaskList, func(task *protobuf.PB_TASK) bool { return task.GetId() == id })
	info.TaskIdListFinish = append(info.TaskIdListFinish, id)
	if info.GetFocusId() == id {
		info.FocusId = proto.Uint32(0)
	}
	if task.Type == islandTaskDaily && task.CountOffset == 1 {
		info.WeekDailyTaskNum = proto.Uint32(info.GetWeekDailyTaskNum() + 1)
	}
	state.public().Exp = proto.Uint32(state.public().GetExp() + task.RewardExp)
	drops := islandTaskDrops(task)
	if err := tx.grantDrops(drops); err != nil {
		return nil, err
	}
	if err := tx.islandTaskEvent(IslandTargetTaskType, 1, task.Type); err != nil {
		return nil, err
	}
	if err := tx.islandAchievementEvent(islandAchievementFinishTypeTask, task.Type, 1); err != nil {
		return nil, err
	}
	return drops, nil
}

// handleIslandTaskSubmit answers IslandSubmitTaskCommand (CS_21038):
// submitIslandTask, its rewards in drop_list.
func (game *Game) handleIslandTaskSubmit(client *connection.Client, req *protobuf.CS_21038) error {
	reply := &protobuf.SC_21039{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		reply.DropList, err = submitIslandTask(tx, state, req.GetTaskId())
		return err
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21039{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// handleIslandTaskSubmitAll answers IslandSubmitTaskOneStepCommand
// (CS_21041, "claim all"): every task submitted at once, all or none.
func (game *Game) handleIslandTaskSubmitAll(client *connection.Client, req *protobuf.CS_21041) error {
	reply := &protobuf.SC_21042{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		for _, id := range req.GetTaskIds() {
			drops, err := submitIslandTask(tx, state, id)
			if err != nil {
				return err
			}
			reply.DropList = append(reply.DropList, drops...)
		}
		return nil
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21042{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// islandSeasonEnd is when a development season ends.
func islandSeasonEnd(config islandSeasonConfig) time.Time {
	end := config.Time[1]
	return time.Date(end[0][0], time.Month(end[0][1]), end[0][2], end[1][0], end[1][1], end[1][2], 0, gamedata.Zone)
}

// handleIslandSeasonPtAward answers IslandGetSeasonPtAwardCommand
// (CS_21022): the reward of a season point threshold reached (target_pt),
// or of every one reached and not taken yet (0).
func (game *Game) handleIslandSeasonPtAward(client *connection.Client, req *protobuf.CS_21022) error {
	reply := &protobuf.SC_21023{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		season := state.private().Season
		config := islandSeasons[season.GetId()]
		for i, target := range config.Target {
			if target > season.GetPt() || slices.Contains(season.FetchList, target) || req.GetTargetPt() != 0 && target != req.GetTargetPt() || i >= len(config.Rewards) {
				continue
			}
			season.FetchList = append(season.FetchList, target)
			reply.DropList = append(reply.DropList, configDrops([][]uint32{config.Rewards[i]})...)
		}
		if len(reply.DropList) == 0 {
			return errRefused
		}
		return tx.grantDrops(reply.DropList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21023{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// handleIslandSeasonReset answers IslandResetSeasonCommand (CS_21024, sent
// on entering once the island's season ended and a later one is current,
// island_set season_now): the ended season goes to the reviews (its
// counters) and the island starts the current one with no points. GUESS:
// the ranking among players (rank awards, IslandSeasonReview PT_RANK) is
// the official server's; belfast ranks nobody and gives no rank award.
func (game *Game) handleIslandSeasonReset(client *connection.Client, req *protobuf.CS_21024) error {
	reply := &protobuf.SC_21025{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		private := state.private()
		old := private.Season
		current := islandSetInt("season_now")
		if old.GetId() >= current || serverNow().Before(islandSeasonEnd(islandSeasons[old.GetId()])) {
			return errRefused
		}
		review := &protobuf.PB_ISLAND_SEASON_REVIEW{Id: proto.Uint32(old.GetId()), CountList: old.CountList}
		private.SeasonReviewList = append(private.SeasonReviewList, review)
		private.Season = &protobuf.PB_ISLAND_SEASON{Id: proto.Uint32(current), Pt: proto.Uint32(0)}
		reply.SeasonReview = review
		return nil
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21025{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// IslandAchievementType values the server counts
const (
	islandAchievementFinishTypeTask = 2
)

// islandAchievementEvent adds to an achievement record the server counts
// (IslandAchievementType: tasks submitted, orders, items made...), if an
// achievement uses it; the new value is pushed (SC_21053).
func (tx *Tx) islandAchievementEvent(kind, arg, count uint32) error {
	if !islandAchievementRecords[kind][arg] {
		return nil
	}
	state, err := tx.island()
	if err != nil {
		return err
	}
	return setIslandAchievement(tx, state, kind, arg, count, true)
}

// setIslandAchievement sets (or adds to) an achievement record and pushes
// it (SC_21053).
func setIslandAchievement(tx *Tx, state *islandState, kind, arg, value uint32, add bool) error {
	system := state.private().AchievementSys
	var record *protobuf.PB_ISLAND_ACHIEVENT
	for _, entry := range system.AchieveList {
		if entry.GetEventType() == kind && entry.GetEventArg() == arg {
			record = entry
		}
	}
	if record == nil {
		record = &protobuf.PB_ISLAND_ACHIEVENT{EventType: proto.Uint32(kind), EventArg: proto.Uint32(arg), Value: proto.Uint32(0)}
		system.AchieveList = append(system.AchieveList, record)
	}
	if add {
		value += record.GetValue()
	}
	record.Value = proto.Uint32(value)
	if tx != nil {
		tx.push(&protobuf.SC_21053{EventList: []*protobuf.PB_ISLAND_ACHIEVENT{proto.Clone(record).(*protobuf.PB_ISLAND_ACHIEVENT)}})
	}
	return nil
}

// handleIslandAchievementRecords takes IslandUpdateAchvCommand (CS_21052,
// no reply): the records the client counts itself (photos taken,
// IslandAchievementType.GetClientTypes), as values to add.
func (game *Game) handleIslandAchievementRecords(client *connection.Client, req *protobuf.CS_21052) error {
	return game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		for _, record := range req.GetEventList() {
			if record.GetEventType() != 37 || !islandAchievementRecords[record.GetEventType()][record.GetEventArg()] {
				continue
			}
			if err := setIslandAchievement(nil, state, record.GetEventType(), record.GetEventArg(), record.GetValue(), true); err != nil {
				return err
			}
		}
		return nil
	})
}

// handleIslandAchievementAward answers IslandGetAchvAwardCommand
// (CS_21050): the rewards of reached achievements, each once. A record the
// server keeps must reach target_num; the ones the client counts from the
// island itself (IslandAchievementType.GetRuntimeTypes) are its word.
func (game *Game) handleIslandAchievementAward(client *connection.Client, req *protobuf.CS_21050) error {
	reply := &protobuf.SC_21051{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		system := state.private().AchievementSys
		for _, id := range req.GetIdList() {
			config, ok := islandAchievements[id]
			if !ok || slices.Contains(system.FinishList, id) {
				return errRefused
			}
			if islandAchievementRecords[config.TargetType] != nil && !slices.Contains(islandRuntimeAchievements, config.TargetType) {
				value := uint32(0)
				for _, entry := range system.AchieveList {
					if entry.GetEventType() == config.TargetType && entry.GetEventArg() == config.TargetArg {
						value = entry.GetValue()
					}
				}
				if value < config.TargetNum {
					return errRefused
				}
			}
			system.FinishList = append(system.FinishList, id)
			reply.DropList = append(reply.DropList, configDrops(config.Award)...)
		}
		return tx.grantDrops(reply.DropList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21051{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// IslandAchievementType.GetRuntimeTypes: records the client counts from the
// island itself
var islandRuntimeAchievements = []uint32{1, 3, 5, 6, 7, 8, 9, 10, 11, 12, 14, 15, 16, 25, 26, 27, 28, 30, 33, 36}
