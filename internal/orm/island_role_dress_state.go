package orm

import (
	"context"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandRoleDressState struct {
	CommanderID uint32
	DressID     uint32
	Num         uint32
	Read        uint32
	Time        uint32
	UpdatedAt   time.Time
}

func ListIslandRoleDressStates(commanderID uint32) ([]IslandRoleDressState, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, dress_id, num, read, time, updated_at
FROM island_role_dresses
WHERE commander_id = $1
ORDER BY dress_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]IslandRoleDressState, 0)
	for rows.Next() {
		var (
			commanderIDRaw int64
			dressIDRaw     int64
			numRaw         int64
			readRaw        int64
			timeRaw        int64
			entry          IslandRoleDressState
		)
		if err := rows.Scan(&commanderIDRaw, &dressIDRaw, &numRaw, &readRaw, &timeRaw, &entry.UpdatedAt); err != nil {
			return nil, err
		}
		entry.CommanderID = uint32(commanderIDRaw)
		entry.DressID = uint32(dressIDRaw)
		entry.Num = uint32(numRaw)
		entry.Read = uint32(readRaw)
		entry.Time = uint32(timeRaw)
		states = append(states, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func GetIslandRoleDressState(commanderID uint32, dressID uint32) (*IslandRoleDressState, error) {
	var (
		commanderIDRaw int64
		dressIDRaw     int64
		numRaw         int64
		readRaw        int64
		timeRaw        int64
		state          IslandRoleDressState
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, dress_id, num, read, time, updated_at
FROM island_role_dresses
WHERE commander_id = $1 AND dress_id = $2
`, int64(commanderID), int64(dressID)).Scan(
		&commanderIDRaw,
		&dressIDRaw,
		&numRaw,
		&readRaw,
		&timeRaw,
		&state.UpdatedAt,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	state.CommanderID = uint32(commanderIDRaw)
	state.DressID = uint32(dressIDRaw)
	state.Num = uint32(numRaw)
	state.Read = uint32(readRaw)
	state.Time = uint32(timeRaw)
	return &state, nil
}

func UpsertIslandRoleDressState(state *IslandRoleDressState) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_role_dresses (commander_id, dress_id, num, read, time)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (commander_id, dress_id)
DO UPDATE SET
	num = EXCLUDED.num,
	read = EXCLUDED.read,
	time = EXCLUDED.time,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.DressID), int64(state.Num), int64(state.Read), int64(state.Time))
	return err
}

func MarkRoleIslandDressRead(commanderID uint32, dressIDs []uint32) error {
	if len(dressIDs) == 0 {
		return nil
	}
	now := uint32(time.Now().UTC().Unix())
	ctx := context.Background()
	for _, dressID := range dressIDs {
		_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO island_role_dresses (commander_id, dress_id, num, read, time)
VALUES ($1, $2, 0, 1, $3)
ON CONFLICT (commander_id, dress_id)
DO UPDATE SET
	read = 1,
	time = GREATEST(island_role_dresses.time, EXCLUDED.time),
	updated_at = CURRENT_TIMESTAMP
`, int64(commanderID), int64(dressID), int64(now))
		if err != nil {
			return err
		}
	}
	return nil
}

func AddIslandRoleDressNum(commanderID uint32, dressID uint32, num int32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_role_dresses (commander_id, dress_id, num, read, time)
VALUES ($1, $2, GREATEST($3, 0), 0, 0)
ON CONFLICT (commander_id, dress_id)
DO UPDATE SET
	num = GREATEST(island_role_dresses.num + $3, 0),
	updated_at = CURRENT_TIMESTAMP
`, int64(commanderID), int64(dressID), int64(num))
	return err
}
