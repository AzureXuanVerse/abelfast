package game

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ActivityConst.ACTIVITY_TYPE_* belfast operates (CS_11202)
const (
	activityType7DaysLogin = 3
	activityTypeMonthSign  = 6
	activityTypeTaskList   = 18
)

// ActivityConst.ACTIVITY_TYPE_* whose new commander data isn't all zeros
const (
	activityTypePTBuff          = 74  // data2 is the event start
	activityTypeIslandDrawAward = 800 // DrawAwardActivity reads data1KeyValueList[1]
)

// ActivityConst.ACTIVITY_TYPE_MILITARY_EXERCISE: always running, a new
// season every exerciseSeasonLength (its activity_template timer is a 2017
// one); data1 is the season start. Like activityTypeMonthSign, its
// ACTIVITYINFO is computed by the server.
const activityTypeMilitaryExercise = 7

// openActivities are the activities running at now: activity_template rows
// whose time is "always" or a timer containing now, and the exercise season.
func openActivities(now time.Time) []activityConfig {
	var open []activityConfig
	for _, config := range activityConfigs {
		if config.Time.Open(now) || config.Type == activityTypeMilitaryExercise {
			open = append(open, config)
		}
	}
	slices.SortFunc(open, func(a, b activityConfig) int { return cmp.Compare(a.ID, b.ID) })
	return open
}

// openActivity is activity id's config, if it runs at now.
func openActivity(id uint32, now time.Time) (activityConfig, bool) {
	config, ok := activityConfigs[id]
	return config, ok && (config.Time.Open(now) || config.Type == activityTypeMilitaryExercise)
}

// newActivityState is what a new commander's ACTIVITYINFO of an activity
// holds: Activity.lua reads every missing field as 0, so only the types that
// start elsewhere set anything.
func newActivityState(config activityConfig) db.CommanderActivity {
	state := db.CommanderActivity{ActivityID: config.ID}
	start := uint32(0)
	if !config.Time.Start.IsZero() {
		start = uint32(config.Time.Start.Unix())
	}
	switch config.Type {
	case activityTypeTaskList:
		state.Data1 = start
	case activityTypePTBuff:
		state.Data2 = start
	}
	return state
}

// activity_7_day_sign row: front_drops is the drop of each day of the 7-day
// login
type sevenDaySignConfig struct {
	FrontDrops [][]uint32 `json:"front_drops"`
}

var sevenDaySignConfigs map[uint32]sevenDaySignConfig

// activityState is the stored state of an activity, or the one a new
// commander starts with.
func activityState(ctx context.Context, queries *db.Queries, commanderID uint32, config activityConfig) (db.CommanderActivity, error) {
	row, err := queries.GetActivity(ctx, db.GetActivityParams{CommanderID: commanderID, ActivityID: config.ID})
	if errors.Is(err, sql.ErrNoRows) {
		row, err = newActivityState(config), nil
		row.CommanderID = commanderID
	}
	return row, err
}

// monthSignState resets the monthly sign-in when the month changed: data1
// and data2 are the year and month signed in, data3 the re-signs, data1_list
// the days signed.
func monthSignState(state db.CommanderActivity, now time.Time) db.CommanderActivity {
	local := now.In(gamedata.Zone)
	if state.Data1 != uint32(local.Year()) || state.Data2 != uint32(local.Month()) {
		state.Data1, state.Data2, state.Data3, state.Data1List = uint32(local.Year()), uint32(local.Month()), 0, nil
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
	fleets, err := activityFleets(ctx, game.DB.Queries, commander.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	reply := &protobuf.SC_11200{}
	for _, config := range openActivities(now) {
		state, ok := stored[config.ID]
		if !ok {
			state = newActivityState(config)
		}
		stopTime := uint32(0)
		if !config.Time.End.IsZero() {
			stopTime = uint32(config.Time.End.Unix())
		}
		switch config.Type {
		case activityTypeMilitaryExercise:
			start := periodStart(now, exerciseSeasonLength)
			stopTime, state.Data1 = uint32(start.Add(exerciseSeasonLength).Unix()-1), uint32(start.Unix())
		case activityTypeMonthSign:
			state = monthSignState(state, now)
		}
		info := &protobuf.ACTIVITYINFO{
			Id:        proto.Uint32(config.ID),
			StopTime:  proto.Uint32(stopTime),
			Data1:     proto.Uint32(state.Data1),
			Data2:     proto.Uint32(state.Data2),
			Data3:     proto.Uint32(state.Data3),
			Data4:     proto.Uint32(state.Data4),
			Data1List: state.Data1List,
			GroupList: fleets[config.ID],
		}
		if config.Type == activityTypeIslandDrawAward {
			info.Date1KeyValueList = []*protobuf.KEYVALUELIST_P11{{Key: proto.Uint32(1)}}
		}
		reply.ActivityList = append(reply.ActivityList, info)
	}
	return reply, nil
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
	config, open := openActivity(req.GetActivityId(), now)
	activityType := config.Type
	if !open || req.GetCmd() != 1 {
		activityType = 0
	}
	var err error
	switch activityType {
	case activityType7DaysLogin:
		days := sevenDaySignConfigs[config.ConfigID].FrontDrops
		err = game.tx(ctx, client, func(tx *Tx) error {
			queries := tx.Queries
			state, err := activityState(ctx, queries, client.CommanderID, config)
			if err != nil {
				return err
			}
			if int(state.Data1) >= len(days) || (state.Data2 != 0 && sameServerDay(time.Unix(int64(state.Data2), 0), now)) {
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
			state, err := activityState(ctx, queries, client.CommanderID, config)
			if err != nil {
				return err
			}
			state = monthSignState(state, now)
			days := state.Data1List
			today := uint32(now.In(gamedata.Zone).Day())
			if slices.Contains(days, today) || len(days) >= len(monthSignDrops[0]) {
				return errRefused
			}
			reply.AwardList = configDrops([][]uint32{monthSignDrops[state.Data2-1][len(days)]})
			if err := tx.grantDrops(reply.AwardList); err != nil {
				return err
			}
			state.Data1List = append(days, today)
			return queries.SaveActivity(ctx, db.SaveActivityParams(state))
		})
	case activityTypeTaskList:
		// the tasks are pushed (SC_20003) before SC_11203
		err = game.acceptTasksPush(ctx, client, activityTaskIDs(config, now)...)
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
