package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type ActivityStoreState struct {
	CommanderID uint32
	ActivityID  uint32
	Data1       uint32
	StrData1    string
}

func (ActivityStoreState) TableName() string {
	return "activity_store_states"
}

func GetActivityStoreState(commanderID uint32, activityID uint32) (*ActivityStoreState, error) {
	state := &ActivityStoreState{}
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, activity_id, data1, str_data1
FROM activity_store_states
WHERE commander_id = $1 AND activity_id = $2
`, int64(commanderID), int64(activityID)).Scan(&state.CommanderID, &state.ActivityID, &state.Data1, &state.StrData1)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func UpsertActivityStoreState(state *ActivityStoreState) error {
	if state == nil {
		return nil
	}
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO activity_store_states (commander_id, activity_id, data1, str_data1)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id, activity_id)
DO UPDATE SET
  data1 = EXCLUDED.data1,
  str_data1 = EXCLUDED.str_data1,
  updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.ActivityID), int64(state.Data1), state.StrData1)
	return err
}

func UpsertActivityStoreStateTx(ctx context.Context, tx pgx.Tx, state *ActivityStoreState) error {
	if state == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO activity_store_states (commander_id, activity_id, data1, str_data1)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id, activity_id)
DO UPDATE SET
  data1 = EXCLUDED.data1,
  str_data1 = EXCLUDED.str_data1,
  updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.ActivityID), int64(state.Data1), state.StrData1)
	return err
}
