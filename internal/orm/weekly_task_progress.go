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

type WeeklyTaskEntry struct {
	ID       uint32 `json:"id"`
	Progress uint32 `json:"progress"`
}

type WeeklyTaskProgress struct {
	CommanderID   uint32
	WeekStartUnix uint32
	Pt            uint32
	RewardLv      uint32
	Tasks         []WeeklyTaskEntry
}

func (WeeklyTaskProgress) TableName() string {
	return "weekly_task_progresses"
}

func CurrentWeeklyResetUnix(now time.Time) uint32 {
	utc := now.UTC()
	startOfDay := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(startOfDay.Weekday()) + 6) % 7
	return uint32(startOfDay.AddDate(0, 0, -offset).Unix())
}

func LoadWeeklyTaskProgress(commanderID uint32, now time.Time) (*WeeklyTaskProgress, error) {
	ctx := context.Background()
	if db.DefaultStore == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var state *WeeklyTaskProgress
	err := db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		loaded, err := loadWeeklyTaskProgressTx(ctx, tx, commanderID, now)
		if err != nil {
			return err
		}
		state = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func LoadWeeklyTaskProgressForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, now time.Time) (*WeeklyTaskProgress, error) {
	return loadWeeklyTaskProgressTx(ctx, tx, commanderID, now)
}

func WithWeeklyTaskProgressTx(commanderID uint32, fn func(state *WeeklyTaskProgress) error) error {
	ctx := context.Background()
	return db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := loadWeeklyTaskProgressTx(ctx, tx, commanderID, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := fn(state); err != nil {
			return err
		}
		return SaveWeeklyTaskProgressTx(ctx, tx, state)
	})
}

func SaveWeeklyTaskProgressTx(ctx context.Context, tx pgx.Tx, state *WeeklyTaskProgress) error {
	tasksJSON, err := marshalWeeklyTasks(state.Tasks)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
UPDATE weekly_task_progresses
SET week_start_unix = $2,
	pt = $3,
	reward_lv = $4,
	tasks = $5,
	updated_at = CURRENT_TIMESTAMP
WHERE commander_id = $1
`, int64(state.CommanderID), int64(state.WeekStartUnix), int64(state.Pt), int64(state.RewardLv), tasksJSON)
	return err
}

func loadWeeklyTaskProgressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, now time.Time) (*WeeklyTaskProgress, error) {
	weekStart := CurrentWeeklyResetUnix(now)
	row := tx.QueryRow(ctx, `
SELECT week_start_unix, pt, reward_lv, tasks
FROM weekly_task_progresses
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))
	var weekStartUnix int64
	var pt int64
	var rewardLv int64
	var tasksJSON []byte
	err := row.Scan(&weekStartUnix, &pt, &rewardLv, &tasksJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			state := &WeeklyTaskProgress{
				CommanderID:   commanderID,
				WeekStartUnix: weekStart,
				Tasks:         []WeeklyTaskEntry{},
			}
			if err := insertWeeklyTaskProgressTx(ctx, tx, state); err != nil {
				return nil, err
			}
			return state, nil
		}
		return nil, err
	}

	tasks, err := unmarshalWeeklyTasks(tasksJSON)
	if err != nil {
		return nil, err
	}
	state := &WeeklyTaskProgress{
		CommanderID:   commanderID,
		WeekStartUnix: uint32(weekStartUnix),
		Pt:            uint32(pt),
		RewardLv:      uint32(rewardLv),
		Tasks:         tasks,
	}
	if state.WeekStartUnix != weekStart {
		state.WeekStartUnix = weekStart
		state.Pt = 0
		state.RewardLv = 0
		state.Tasks = []WeeklyTaskEntry{}
		if err := SaveWeeklyTaskProgressTx(ctx, tx, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func insertWeeklyTaskProgressTx(ctx context.Context, tx pgx.Tx, state *WeeklyTaskProgress) error {
	tasksJSON, err := marshalWeeklyTasks(state.Tasks)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO weekly_task_progresses (commander_id, week_start_unix, pt, reward_lv, tasks)
VALUES ($1, $2, $3, $4, $5)
`, int64(state.CommanderID), int64(state.WeekStartUnix), int64(state.Pt), int64(state.RewardLv), tasksJSON)
	return err
}

func marshalWeeklyTasks(tasks []WeeklyTaskEntry) ([]byte, error) {
	if len(tasks) == 0 {
		return []byte("[]"), nil
	}
	copyTasks := make([]WeeklyTaskEntry, len(tasks))
	copy(copyTasks, tasks)
	sort.Slice(copyTasks, func(i, j int) bool {
		return copyTasks[i].ID < copyTasks[j].ID
	})
	return json.Marshal(copyTasks)
}

func unmarshalWeeklyTasks(data []byte) ([]WeeklyTaskEntry, error) {
	if len(data) == 0 {
		return []WeeklyTaskEntry{}, nil
	}
	var tasks []WeeklyTaskEntry
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	if tasks == nil {
		return []WeeklyTaskEntry{}, nil
	}
	return tasks, nil
}
