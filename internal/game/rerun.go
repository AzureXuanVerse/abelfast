package game

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Capsule Reruns (ActivityRemasterProxy, ActivityRemasterScene opened from
// the main screen's rerun button, MainActRemasterBtn): during an
// activity_re_timer period the commander picks one past event (activity_re)
// and its activities (act_id) run for them alone. Once picked it can't be
// changed for the period and can't be picked again in a later one; the
// is_special row can be picked any number of times, the client offers it
// once every other was (ActivityRemasterScene:InitList). The wiki (Capsule
// Rerun): "The Event runs for the duration of the Capsule Rerun period;
// choosing the Event later in the period will not extend its duration".

// activity_re row: act_time lists [activity, hours, banner] for some of
// act_id; the hours move that activity's end from the period's end (the
// shops and PT activities stay a week longer: 168)
type rerunConfig struct {
	ActID     []uint32        `json:"act_id"`
	ActTime   json.RawMessage `json:"act_time"`
	IsSpecial uint32          `json:"is_special"`

	hours map[uint32]int // act_time, by activity
}

// activity_re_timer row: a period the reruns can be picked in
type rerunTimer struct {
	Timer gamedata.Window `json:"timer"`
}

var (
	rerunConfigs map[uint32]rerunConfig
	rerunTimers  map[uint32]rerunTimer
)

func loadRerunData() error {
	err := gamedata.LoadAll(
		gamedata.Load("activity_re", &rerunConfigs),
		gamedata.Load("activity_re_timer", &rerunTimers),
	)
	if err != nil {
		return err
	}
	for id, config := range rerunConfigs {
		var times [][]int
		json.Unmarshal(config.ActTime, &times) // "" for the special one
		config.hours = map[uint32]int{}
		for _, entry := range times {
			if len(entry) >= 2 && entry[0] > 0 {
				config.hours[uint32(entry[0])] = entry[1]
			}
		}
		rerunConfigs[id] = config
	}
	return nil
}

// openRerunTimer is ActivityRemasterProxy:InActTime: the period running at
// now, 0 when none.
func openRerunTimer(now time.Time) uint32 {
	var open uint32
	for id, timer := range rerunTimers {
		if timer.Timer.Open(now) && (open == 0 || id < open) {
			open = id
		}
	}
	return open
}

// rerunActivityStops are rerun id's activities picked in period timerID and
// when each ends: the period's end, moved by its act_time hours.
func rerunActivityStops(id, timerID uint32) map[uint32]time.Time {
	config, ok := rerunConfigs[id]
	timer, okTimer := rerunTimers[timerID]
	if !ok || !okTimer || timer.Timer.End.IsZero() {
		return nil
	}
	stops := make(map[uint32]time.Time, len(config.ActID))
	for _, act := range config.ActID {
		stops[act] = timer.Timer.End.Add(time.Duration(config.hours[act]) * time.Hour)
	}
	return stops
}

// rerunState is SC_11213: the commander's latest pick and its period, and
// every event picked so far (the client marks them finished, which is what
// keeps them from being picked again).
func rerunState(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_11213, error) {
	rows, err := queries.ListRemasters(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_11213{ActivityReTimerId: proto.Uint32(0), ActivityReId: proto.Uint32(0)}
	for _, row := range rows {
		msg.ActivityReTimerId, msg.ActivityReId = proto.Uint32(row.TimerID), proto.Uint32(row.RemasterID)
		msg.FinishReId = append(msg.FinishReId, row.RemasterID)
	}
	return msg, nil
}

// handleRerunActivate answers ActiveActReamsterCommand (CS_11214) with the
// checks the client makes before sending it: a period is open
// (InActTime), the commander hasn't picked an event in it yet
// (IsActivating) and this one was never picked unless it is the special one
// (CanActiveRemaster, act_remaster_active_erro). The event's activities
// start for the commander, pushed (SC_11201) ahead of the reply: the client
// goes back to the main screen, whose banners and buttons read them.
func (game *Game) handleRerunActivate(client *connection.Client, req *protobuf.CS_11214) error {
	ctx := context.Background()
	now := time.Now()
	id := req.GetActivityReId()
	config, ok := rerunConfigs[id]
	timerID := openRerunTimer(now)
	if !ok || timerID == 0 {
		return client.Send(&protobuf.SC_11215{Result: result(errRefused)})
	}
	err := game.tx(ctx, client, func(tx *Tx) error {
		rows, err := tx.ListRemasters(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.TimerID == timerID || (row.RemasterID == id && config.IsSpecial != 1) {
				return errRefused
			}
		}
		if err := tx.ActivateRemaster(ctx, db.ActivateRemasterParams{CommanderID: client.CommanderID, RemasterID: id, TimerID: timerID, ActivatedAt: now.Unix()}); err != nil {
			return err
		}
		fleets, err := activityFleets(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		start := rerunTimers[timerID].Timer.Start
		stops := rerunActivityStops(id, timerID)
		for _, act := range config.ActID {
			activity, ok := activityConfigs[act]
			if !ok || !stops[act].After(now) {
				continue
			}
			// a rerun starts over: Activity:getStartTime is the period's
			// start for these "stop" activities
			state := newActivityStateAt(activity, start)
			state.CommanderID = client.CommanderID
			if err := tx.SaveActivity(ctx, db.SaveActivityParams(state)); err != nil {
				return err
			}
			tx.push(&protobuf.SC_11201{ActivityInfo: activityInfo(activity, state, uint32(stops[act].Unix()), fleets[act])})
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_11215{Result: result(err)})
}
