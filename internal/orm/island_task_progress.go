package orm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandTaskEntry struct {
	TaskID      uint32                    `json:"task_id"`
	Timestamp   uint32                    `json:"timestamp"`
	ProcessList []IslandTaskTargetProcess `json:"process_list,omitempty"`
}

type IslandTaskTargetProcess struct {
	TargetID    uint32 `json:"target_id"`
	TargetCount uint32 `json:"target_count"`
}

type IslandTaskProgress struct {
	CommanderID        uint32
	WeekStartUnix      uint32
	LastRefreshDayUnix uint32
	WeekDailyTaskNum   uint32
	TraceTaskID        uint32
	TraceDailyTaskID   uint32
	ActiveTasks        []IslandTaskEntry
	FinishedTaskIDs    []uint32
	FutureTaskWindows  []IslandTaskEntry
	RandomTaskWindows  []IslandTaskEntry
}

func (IslandTaskProgress) TableName() string {
	return "island_task_progresses"
}

func CurrentDayResetUnix(now time.Time) uint32 {
	utc := now.UTC()
	dayStart := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return uint32(dayStart.Unix())
}

func LoadIslandTaskProgress(commanderID uint32, now time.Time) (*IslandTaskProgress, error) {
	if db.DefaultStore == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	ctx := context.Background()
	var state *IslandTaskProgress
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		loaded, loadErr := loadIslandTaskProgressTx(ctx, tx, commanderID, now)
		if loadErr != nil {
			return loadErr
		}
		state = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func LoadIslandTaskProgressForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, now time.Time) (*IslandTaskProgress, error) {
	return loadIslandTaskProgressTx(ctx, tx, commanderID, now)
}

func WithIslandTaskProgressTx(commanderID uint32, now time.Time, fn func(state *IslandTaskProgress) error) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := loadIslandTaskProgressTx(ctx, tx, commanderID, now)
		if err != nil {
			return err
		}
		if err := fn(state); err != nil {
			return err
		}
		return SaveIslandTaskProgressTx(ctx, tx, state)
	})
}

func SaveIslandTaskProgressTx(ctx context.Context, tx pgx.Tx, state *IslandTaskProgress) error {
	activeJSON, err := marshalIslandTaskEntries(state.ActiveTasks)
	if err != nil {
		return err
	}
	finishedJSON, err := marshalUint32List(state.FinishedTaskIDs)
	if err != nil {
		return err
	}
	futureJSON, err := marshalIslandTaskEntries(state.FutureTaskWindows)
	if err != nil {
		return err
	}
	randomJSON, err := marshalIslandTaskEntries(state.RandomTaskWindows)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
UPDATE island_task_progresses
SET week_start_unix = $2,
	last_refresh_day_unix = $3,
	week_daily_task_num = $4,
	trace_task_id = $5,
	trace_daily_task_id = $6,
	active_tasks = $7,
	finished_task_ids = $8,
	future_task_windows = $9,
	random_task_windows = $10,
	updated_at = CURRENT_TIMESTAMP
WHERE commander_id = $1
`,
		int64(state.CommanderID),
		int64(state.WeekStartUnix),
		int64(state.LastRefreshDayUnix),
		int64(state.WeekDailyTaskNum),
		int64(state.TraceTaskID),
		int64(state.TraceDailyTaskID),
		activeJSON,
		finishedJSON,
		futureJSON,
		randomJSON,
	)
	return err
}

func loadIslandTaskProgressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, now time.Time) (*IslandTaskProgress, error) {
	weekStart := CurrentWeeklyResetUnix(now)
	dayStart := CurrentDayResetUnix(now)

	row := tx.QueryRow(ctx, `
SELECT week_start_unix, last_refresh_day_unix, week_daily_task_num, trace_task_id, trace_daily_task_id,
	active_tasks, finished_task_ids, future_task_windows, random_task_windows
FROM island_task_progresses
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))

	var weekStartUnix int64
	var lastRefreshDayUnix int64
	var weekDailyTaskNum int64
	var traceTaskID int64
	var traceDailyTaskID int64
	var activeJSON []byte
	var finishedJSON []byte
	var futureJSON []byte
	var randomJSON []byte

	err := row.Scan(
		&weekStartUnix,
		&lastRefreshDayUnix,
		&weekDailyTaskNum,
		&traceTaskID,
		&traceDailyTaskID,
		&activeJSON,
		&finishedJSON,
		&futureJSON,
		&randomJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			state := &IslandTaskProgress{
				CommanderID:        commanderID,
				WeekStartUnix:      weekStart,
				LastRefreshDayUnix: dayStart,
				ActiveTasks:        []IslandTaskEntry{},
				FinishedTaskIDs:    []uint32{},
				FutureTaskWindows:  []IslandTaskEntry{},
				RandomTaskWindows:  []IslandTaskEntry{},
			}
			if insertErr := insertIslandTaskProgressTx(ctx, tx, state); insertErr != nil {
				return nil, insertErr
			}
			return state, nil
		}
		return nil, err
	}

	activeTasks, err := unmarshalIslandTaskEntries(activeJSON)
	if err != nil {
		return nil, err
	}
	finishedTaskIDs, err := unmarshalUint32List(finishedJSON)
	if err != nil {
		return nil, err
	}
	futureTaskWindows, err := unmarshalIslandTaskEntries(futureJSON)
	if err != nil {
		return nil, err
	}
	randomTaskWindows, err := unmarshalIslandTaskEntries(randomJSON)
	if err != nil {
		return nil, err
	}

	state := &IslandTaskProgress{
		CommanderID:        commanderID,
		WeekStartUnix:      uint32(weekStartUnix),
		LastRefreshDayUnix: uint32(lastRefreshDayUnix),
		WeekDailyTaskNum:   uint32(weekDailyTaskNum),
		TraceTaskID:        uint32(traceTaskID),
		TraceDailyTaskID:   uint32(traceDailyTaskID),
		ActiveTasks:        activeTasks,
		FinishedTaskIDs:    finishedTaskIDs,
		FutureTaskWindows:  futureTaskWindows,
		RandomTaskWindows:  randomTaskWindows,
	}

	changed := false
	if state.WeekStartUnix != weekStart {
		state.WeekStartUnix = weekStart
		state.WeekDailyTaskNum = 0
		changed = true
	}
	if state.LastRefreshDayUnix != dayStart {
		state.LastRefreshDayUnix = dayStart
		state.WeekDailyTaskNum = 0
		changed = true
	}
	if changed {
		if err := SaveIslandTaskProgressTx(ctx, tx, state); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func insertIslandTaskProgressTx(ctx context.Context, tx pgx.Tx, state *IslandTaskProgress) error {
	activeJSON, err := marshalIslandTaskEntries(state.ActiveTasks)
	if err != nil {
		return err
	}
	finishedJSON, err := marshalUint32List(state.FinishedTaskIDs)
	if err != nil {
		return err
	}
	futureJSON, err := marshalIslandTaskEntries(state.FutureTaskWindows)
	if err != nil {
		return err
	}
	randomJSON, err := marshalIslandTaskEntries(state.RandomTaskWindows)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_task_progresses (
	commander_id, week_start_unix, last_refresh_day_unix, week_daily_task_num,
	trace_task_id, trace_daily_task_id, active_tasks, finished_task_ids, future_task_windows, random_task_windows
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`,
		int64(state.CommanderID),
		int64(state.WeekStartUnix),
		int64(state.LastRefreshDayUnix),
		int64(state.WeekDailyTaskNum),
		int64(state.TraceTaskID),
		int64(state.TraceDailyTaskID),
		activeJSON,
		finishedJSON,
		futureJSON,
		randomJSON,
	)
	return err
}

func marshalIslandTaskEntries(entries []IslandTaskEntry) ([]byte, error) {
	if len(entries) == 0 {
		return []byte("[]"), nil
	}
	copyEntries := make([]IslandTaskEntry, len(entries))
	copy(copyEntries, entries)
	sort.Slice(copyEntries, func(i, j int) bool {
		if copyEntries[i].TaskID == copyEntries[j].TaskID {
			return copyEntries[i].Timestamp < copyEntries[j].Timestamp
		}
		return copyEntries[i].TaskID < copyEntries[j].TaskID
	})
	return json.Marshal(copyEntries)
}

func unmarshalIslandTaskEntries(data []byte) ([]IslandTaskEntry, error) {
	if len(data) == 0 {
		return []IslandTaskEntry{}, nil
	}
	var entries []IslandTaskEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return []IslandTaskEntry{}, nil
	}
	return entries, nil
}

func marshalUint32List(values []uint32) ([]byte, error) {
	if len(values) == 0 {
		return []byte("[]"), nil
	}
	copyValues := make([]uint32, len(values))
	copy(copyValues, values)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	return json.Marshal(copyValues)
}

func unmarshalUint32List(data []byte) ([]uint32, error) {
	if len(data) == 0 {
		return []uint32{}, nil
	}
	var values []uint32
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []uint32{}, nil
	}
	return values, nil
}
