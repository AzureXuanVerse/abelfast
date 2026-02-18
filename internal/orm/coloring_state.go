package orm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ColoringCellState struct {
	PageID uint32 `json:"page_id"`
	Row    uint32 `json:"row"`
	Column uint32 `json:"column"`
	Color  uint32 `json:"color"`
}

type ColoringDropState struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Number uint32 `json:"number"`
}

type ColoringAwardState struct {
	PageID uint32              `json:"page_id"`
	Drops  []ColoringDropState `json:"drops"`
}

type CommanderColoringState struct {
	CommanderID uint32
	ActivityID  uint32
	StartTime   uint32
	Cells       []ColoringCellState
	Awards      []ColoringAwardState
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func GetCommanderColoringState(commanderID uint32, activityID uint32) (*CommanderColoringState, error) {
	ctx := context.Background()
	state := &CommanderColoringState{}
	var cellsRaw []byte
	var awardsRaw []byte
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, activity_id, start_time, cells, awards, created_at, updated_at
FROM commander_coloring_states
WHERE commander_id = $1 AND activity_id = $2
`, int64(commanderID), int64(activityID)).Scan(
		&state.CommanderID,
		&state.ActivityID,
		&state.StartTime,
		&cellsRaw,
		&awardsRaw,
		&state.CreatedAt,
		&state.UpdatedAt,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if len(cellsRaw) == 0 {
		state.Cells = []ColoringCellState{}
	} else if err := json.Unmarshal(cellsRaw, &state.Cells); err != nil {
		return nil, err
	}
	if len(awardsRaw) == 0 {
		state.Awards = []ColoringAwardState{}
	} else if err := json.Unmarshal(awardsRaw, &state.Awards); err != nil {
		return nil, err
	}
	return state, nil
}

func GetOrCreateCommanderColoringState(commanderID uint32, activityID uint32, startTime uint32) (*CommanderColoringState, error) {
	state, err := GetCommanderColoringState(commanderID, activityID)
	if err == nil {
		if state.StartTime == 0 {
			state.StartTime = startTime
			if saveErr := SaveCommanderColoringState(state); saveErr != nil {
				return nil, saveErr
			}
		}
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}
	state = &CommanderColoringState{
		CommanderID: commanderID,
		ActivityID:  activityID,
		StartTime:   startTime,
		Cells:       []ColoringCellState{},
		Awards:      []ColoringAwardState{},
	}
	if err := SaveCommanderColoringState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func SaveCommanderColoringState(state *CommanderColoringState) error {
	ctx := context.Background()
	return saveCommanderColoringStateWithExec(ctx, db.DefaultStore.Pool, state)
}

func SaveCommanderColoringStateTx(ctx context.Context, tx pgx.Tx, state *CommanderColoringState) error {
	return saveCommanderColoringStateWithExec(ctx, tx, state)
}

type coloringStateExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func saveCommanderColoringStateWithExec(ctx context.Context, executor coloringStateExecutor, state *CommanderColoringState) error {
	cellsRaw, err := json.Marshal(state.Cells)
	if err != nil {
		return err
	}
	awardsRaw, err := json.Marshal(state.Awards)
	if err != nil {
		return err
	}
	_, err = executor.Exec(ctx, `
INSERT INTO commander_coloring_states (commander_id, activity_id, start_time, cells, awards, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (commander_id, activity_id)
DO UPDATE SET
  start_time = EXCLUDED.start_time,
  cells = EXCLUDED.cells,
  awards = EXCLUDED.awards,
  updated_at = NOW()
`, int64(state.CommanderID), int64(state.ActivityID), int64(state.StartTime), cellsRaw, awardsRaw)
	return err
}
