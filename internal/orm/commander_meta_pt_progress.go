package orm

import (
	"context"
	"encoding/json"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type CommanderMetaPtProgress struct {
	CommanderID uint32
	GroupID     uint32
	Pt          uint32
	FetchList   []uint32
}

func (CommanderMetaPtProgress) TableName() string {
	return "commander_meta_pt_progress"
}

func GetCommanderMetaPtProgress(commanderID uint32, groupID uint32) (*CommanderMetaPtProgress, error) {
	state := &CommanderMetaPtProgress{}
	var fetchListRaw []byte
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, group_id, pt, fetch_list
FROM commander_meta_pt_progress
WHERE commander_id = $1
  AND group_id = $2
`, int64(commanderID), int64(groupID)).Scan(&state.CommanderID, &state.GroupID, &state.Pt, &fetchListRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if len(fetchListRaw) == 0 {
		state.FetchList = []uint32{}
		return state, nil
	}
	if err := json.Unmarshal(fetchListRaw, &state.FetchList); err != nil {
		return nil, err
	}
	return state, nil
}

func GetOrCreateCommanderMetaPtProgress(commanderID uint32, groupID uint32) (*CommanderMetaPtProgress, error) {
	state, err := GetCommanderMetaPtProgress(commanderID, groupID)
	if err == nil {
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}
	state = &CommanderMetaPtProgress{CommanderID: commanderID, GroupID: groupID, FetchList: []uint32{}}
	if err := SaveCommanderMetaPtProgress(state); err != nil {
		return nil, err
	}
	return state, nil
}

func ListCommanderMetaPtProgress(commanderID uint32) ([]CommanderMetaPtProgress, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, group_id, pt, fetch_list
FROM commander_meta_pt_progress
WHERE commander_id = $1
ORDER BY group_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]CommanderMetaPtProgress, 0)
	for rows.Next() {
		state := CommanderMetaPtProgress{}
		var fetchListRaw []byte
		if err := rows.Scan(&state.CommanderID, &state.GroupID, &state.Pt, &fetchListRaw); err != nil {
			return nil, err
		}
		if len(fetchListRaw) == 0 {
			state.FetchList = []uint32{}
		} else if err := json.Unmarshal(fetchListRaw, &state.FetchList); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func SaveCommanderMetaPtProgress(state *CommanderMetaPtProgress) error {
	return saveCommanderMetaPtProgressWithExec(context.Background(), db.DefaultStore.Pool, state)
}

func SaveCommanderMetaPtProgressTx(ctx context.Context, tx pgx.Tx, state *CommanderMetaPtProgress) error {
	return saveCommanderMetaPtProgressWithExec(ctx, tx, state)
}

func GetOrCreateCommanderMetaPtProgressTx(ctx context.Context, tx pgx.Tx, commanderID uint32, groupID uint32) (*CommanderMetaPtProgress, error) {
	if _, err := tx.Exec(ctx, `
INSERT INTO commander_meta_pt_progress (commander_id, group_id, pt, fetch_list, created_at, updated_at)
VALUES ($1, $2, 0, '[]'::jsonb, NOW(), NOW())
ON CONFLICT (commander_id, group_id) DO NOTHING
`, int64(commanderID), int64(groupID)); err != nil {
		return nil, err
	}

	state := &CommanderMetaPtProgress{}
	var fetchListRaw []byte
	err := tx.QueryRow(ctx, `
SELECT commander_id, group_id, pt, fetch_list
FROM commander_meta_pt_progress
WHERE commander_id = $1
  AND group_id = $2
FOR UPDATE
`, int64(commanderID), int64(groupID)).Scan(&state.CommanderID, &state.GroupID, &state.Pt, &fetchListRaw)
	if err != nil {
		return nil, err
	}
	if len(fetchListRaw) == 0 {
		state.FetchList = []uint32{}
	} else if err := json.Unmarshal(fetchListRaw, &state.FetchList); err != nil {
		return nil, err
	}
	return state, nil
}

type metaPtProgressExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func saveCommanderMetaPtProgressWithExec(ctx context.Context, executor metaPtProgressExecutor, state *CommanderMetaPtProgress) error {
	fetchListRaw, err := json.Marshal(state.FetchList)
	if err != nil {
		return err
	}
	_, err = executor.Exec(ctx, `
INSERT INTO commander_meta_pt_progress (commander_id, group_id, pt, fetch_list, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (commander_id, group_id)
DO UPDATE SET
  pt = EXCLUDED.pt,
  fetch_list = EXCLUDED.fetch_list,
  updated_at = NOW()
`, int64(state.CommanderID), int64(state.GroupID), int64(state.Pt), fetchListRaw)
	return err
}
