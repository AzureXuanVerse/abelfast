package game

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ActivityConst types belfast operates (CS_11202)
const (
	activityType7DaysLogin = 3
	activityTypeMonthSign  = 6
	activityTypeTaskList   = 18
)

// activities whose ACTIVITYINFO the server computes
const (
	activityExerciseSeason = 7 // ACTIVITY_TYPE_MILITARY_EXERCISE: data1 is the season start
	activityMonthSign      = 6
	activityIslandDraw     = 990026
)

// openActivity is an activity the server runs: its id, when it ends (0:
// never) and its data fields as a new commander gets them.
type openActivity struct {
	id, stopTime               uint32
	data1, data2, data3, data4 *uint32
	data1List                  []uint32
}

// openActivities are the activities the official server ran during the
// capture, with a new commander's data (SC_11200). Which activities run is
// the server's choice, not a client table; the ones that have ended are left
// out when sent.
var openActivities = []openActivity{
	{id: 990026, stopTime: 1793861999, data1: proto.Uint32(0), data2: proto.Uint32(0), data1List: []uint32{4001, 4002, 4003}},
	{id: 990025, stopTime: 1793861999},
	{id: 990024, stopTime: 1793861999},
	{id: 990023, stopTime: 1793861999},
	{id: 990022, stopTime: 1793861999},
	{id: 990011, stopTime: 0},
	{id: 30726, stopTime: 0, data1: proto.Uint32(0), data2: proto.Uint32(0), data3: proto.Uint32(0)},
	{id: 30724, stopTime: 0},
	{id: 30723, stopTime: 1918105199},
	{id: 30547}, {id: 30546}, {id: 30545}, {id: 30544}, {id: 30543}, {id: 30542}, {id: 30541}, {id: 30540},
	{id: 30539}, {id: 30538}, {id: 30537}, {id: 30536}, {id: 30535}, {id: 30534}, {id: 30533}, {id: 30532},
	{id: 30531}, {id: 30530}, {id: 30529}, {id: 30528}, {id: 30527}, {id: 30526}, {id: 30525}, {id: 30524},
	{id: 30523}, {id: 30522}, {id: 30521}, {id: 30520}, {id: 30519}, {id: 30518}, {id: 30517}, {id: 30516},
	{id: 30515}, {id: 30514}, {id: 30513}, {id: 30512}, {id: 30511}, {id: 30510}, {id: 30509}, {id: 30508},
	{id: 30507}, {id: 30506}, {id: 30505}, {id: 30504}, {id: 30503}, {id: 30502}, {id: 30501}, {id: 30500},
	{id: 30445, stopTime: 0, data1: proto.Uint32(0), data2: proto.Uint32(0)},
	{id: 30209, stopTime: 0},
	{id: 30105, stopTime: 0, data1: proto.Uint32(0)},
	{id: 30017, stopTime: 0},
	{id: 30011, stopTime: 0},
	{id: 980001, stopTime: 0},
	{id: 7505, stopTime: 2050901999},
	{id: 7503, stopTime: 2050901999},
	{id: 7502, stopTime: 2050901999},
	{id: 7501, stopTime: 2050901999},
	{id: 7104, stopTime: 0},
	{id: 7031, stopTime: 1796108399, data1: proto.Uint32(0), data2: proto.Uint32(0), data3: proto.Uint32(0), data4: proto.Uint32(0)},
	{id: 51161, stopTime: 1793516399},
	{id: 51114, stopTime: 0},
	// ACTIVITY_TYPE_PT_BUFF: data2 is the event start, data3 the server-wide progress
	{id: 51203, stopTime: 1793861999, data1: proto.Uint32(0), data2: proto.Uint32(1792047600), data3: proto.Uint32(796790)},
	{id: 51202, stopTime: 1793861999},
	// data1 is the event start
	{id: 51201, stopTime: 1793861999, data1: proto.Uint32(1792047600), data2: proto.Uint32(0), data3: proto.Uint32(0)},
	{id: 21, stopTime: 0},
	{id: 9, stopTime: 0, data1: proto.Uint32(0)},
	{id: activityExerciseSeason},
	{id: activityMonthSign},
	{id: 4, stopTime: 0},
	// 7-day login: data1 days signed, data2 last sign-in time
	{id: 3, stopTime: 0, data1: proto.Uint32(0), data2: proto.Uint32(0)},
}

// activity_7_day_sign row: front_drops is the drop of each day of the 7-day
// login
type sevenDaySignConfig struct {
	FrontDrops [][]uint32 `json:"front_drops"`
}

var sevenDaySignConfigs map[uint32]sevenDaySignConfig

// value is *field, 0 when unset.
func value(field *uint32) uint32 {
	if field == nil {
		return 0
	}
	return *field
}

func joinList(list []uint32) string {
	parts := make([]string, len(list))
	for i, value := range list {
		parts[i] = strconv.FormatUint(uint64(value), 10)
	}
	return strings.Join(parts, ",")
}

func splitList(text string) []uint32 {
	var list []uint32
	for _, part := range strings.Split(text, ",") {
		if value, err := strconv.ParseUint(part, 10, 32); err == nil {
			list = append(list, uint32(value))
		}
	}
	return list
}

// activityState is the stored state of an activity, or the one a new
// commander starts with.
func activityState(ctx context.Context, queries *db.Queries, commanderID uint32, activity openActivity) (db.CommanderActivity, error) {
	row, err := queries.GetActivity(ctx, db.GetActivityParams{CommanderID: commanderID, ActivityID: activity.id})
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return row, err
	}
	return db.CommanderActivity{
		CommanderID: commanderID,
		ActivityID:  activity.id,
		Data1:       value(activity.data1),
		Data2:       value(activity.data2),
		Data3:       value(activity.data3),
		Data4:       value(activity.data4),
		Data1List:   joinList(activity.data1List),
	}, nil
}

// monthSignState resets the monthly sign-in when the month changed: data1
// and data2 are the year and month signed in, data3 the re-signs, data1_list
// the days signed.
func monthSignState(state db.CommanderActivity, now time.Time) db.CommanderActivity {
	local := now.In(serverZone)
	if state.Data1 != uint32(local.Year()) || state.Data2 != uint32(local.Month()) {
		state.Data1, state.Data2, state.Data3, state.Data1List = uint32(local.Year()), uint32(local.Month()), 0, ""
	}
	return state
}

// activityList is SC_11200, ActivityProxy's activities.
func (game *Game) activityList(ctx context.Context, commander db.Commander) (*protobuf.SC_11200, error) {
	rows, err := game.DB.ListActivities(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	stored := make(map[uint32]db.CommanderActivity, len(rows))
	for _, row := range rows {
		stored[row.ActivityID] = row
	}
	now := time.Now()
	reply := &protobuf.SC_11200{}
	for _, activity := range openActivities {
		if activity.stopTime != 0 && int64(activity.stopTime) < now.Unix() {
			continue
		}
		info := &protobuf.ACTIVITYINFO{
			Id:        proto.Uint32(activity.id),
			StopTime:  proto.Uint32(activity.stopTime),
			Data1:     activity.data1,
			Data2:     activity.data2,
			Data3:     activity.data3,
			Data4:     activity.data4,
			Data1List: activity.data1List,
		}
		switch activity.id {
		case activityExerciseSeason:
			start := exerciseSeasonStart(now)
			info.StopTime = proto.Uint32(uint32(start.Add(exerciseSeasonLength).Unix() - 1))
			info.Data1 = proto.Uint32(uint32(start.Unix()))
		case activityMonthSign:
			state := monthSignState(stored[activity.id], now)
			info.Data1, info.Data2, info.Data3 = proto.Uint32(state.Data1), proto.Uint32(state.Data2), proto.Uint32(state.Data3)
			info.Data1List = splitList(state.Data1List)
		case activityIslandDraw:
			info.Date1KeyValueList = []*protobuf.KEYVALUELIST_P11{{Key: proto.Uint32(1)}}
		default:
			if state, ok := stored[activity.id]; ok {
				// only the fields the activity uses are sent
				for _, field := range []struct {
					dst **uint32
					val uint32
				}{{&info.Data1, state.Data1}, {&info.Data2, state.Data2}, {&info.Data3, state.Data3}, {&info.Data4, state.Data4}} {
					if *field.dst != nil {
						*field.dst = proto.Uint32(field.val)
					}
				}
				info.Data1List = splitList(state.Data1List)
			}
		}
		reply.ActivityList = append(reply.ActivityList, info)
	}
	return reply, nil
}

// acceptActivityTasks hands a task list activity's tasks to the tasks area,
// which must push them (SC_20003) before SC_11203.
func (game *Game) acceptActivityTasks(ctx context.Context, client *connection.Client, taskIDs []uint32) error {
	return game.acceptTasksPush(ctx, client, taskIDs...)
}

// handleActivityOperation answers ActivityOperationCommand (CS_11202). The
// client updates the activity's data itself on success (updateActivityData),
// the server only stores it and hands out the awards: there is no SC_11201.
//   - 7-day login (cmd 1): the next day's activity_7_day_sign drop, once per
//     server day.
//   - monthly sign-in (cmd 1): activity_month_sign[month].dayN for the Nth
//     sign-in of the month, once per day.
//   - task list (cmd 1): accepts the activity's tasks.
//
// Anything else, or a refused operation, gets result 1 (activity_op_error).
func (game *Game) handleActivityOperation(client *connection.Client, req *protobuf.CS_11202) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_11203{Result: result(nil)}
	var activity openActivity
	for _, open := range openActivities {
		if open.id == req.GetActivityId() {
			activity = open
		}
	}
	config := activityConfigs[req.GetActivityId()]
	activityType := config.Type
	if activity.id == 0 || req.GetCmd() != 1 || (activity.stopTime != 0 && int64(activity.stopTime) < now.Unix()) {
		activityType = 0
	}
	var err error
	switch activityType {
	case activityType7DaysLogin:
		days := sevenDaySignConfigs[config.ConfigID].FrontDrops
		err = game.tx(ctx, client, func(tx *Tx) error {
			queries := tx.Queries
			state, err := activityState(ctx, queries, client.CommanderID, activity)
			if err != nil {
				return err
			}
			last := time.Unix(int64(state.Data2), 0).In(serverZone)
			today := now.In(serverZone)
			if int(state.Data1) >= len(days) || (state.Data2 != 0 && last.YearDay() == today.YearDay() && last.Year() == today.Year()) {
				return errRefused
			}
			reply.AwardList = configDrops(days[state.Data1 : state.Data1+1])
			if err := tx.grantDrops(reply.AwardList); err != nil {
				return err
			}
			state.Data1++
			state.Data2 = uint32(now.Unix())
			return queries.SaveActivity(ctx, db.SaveActivityParams(state))
		})
	case activityTypeMonthSign:
		err = game.tx(ctx, client, func(tx *Tx) error {
			queries := tx.Queries
			state, err := activityState(ctx, queries, client.CommanderID, activity)
			if err != nil {
				return err
			}
			state = monthSignState(state, now)
			days := splitList(state.Data1List)
			today := uint32(now.In(serverZone).Day())
			if slices.Contains(days, today) || len(days) >= len(monthSignDrops[0]) {
				return errRefused
			}
			reply.AwardList = configDrops([][]uint32{monthSignDrops[state.Data2-1][len(days)]})
			if err := tx.grantDrops(reply.AwardList); err != nil {
				return err
			}
			state.Data1List = joinList(append(days, today))
			return queries.SaveActivity(ctx, db.SaveActivityParams(state))
		})
	case activityTypeTaskList:
		err = game.acceptActivityTasks(ctx, client, activityTaskIDs(config, now))
	default:
		err = errRefused
	}
	if refused(err) {
		return client.Send(&protobuf.SC_11203{Result: result(errRefused)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}
