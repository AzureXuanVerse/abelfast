package orm

import (
	"context"
	"encoding/json"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandCommanderDressState struct {
	CommanderID uint32
	DressID     uint32
	State       uint32
	Color       uint32
	ColorList   []uint32
}

func (IslandCommanderDressState) TableName() string {
	return "island_commander_dresses"
}

func ListIslandCommanderDressStates(commanderID uint32) ([]IslandCommanderDressState, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, dress_id, state, color, color_list
FROM island_commander_dresses
WHERE commander_id = $1
ORDER BY dress_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := []IslandCommanderDressState{}
	for rows.Next() {
		var (
			commanderIDRaw int64
			dressIDRaw     int64
			stateRaw       int64
			colorRaw       int64
			colorListJSON  []byte
			entry          IslandCommanderDressState
		)
		if err := rows.Scan(&commanderIDRaw, &dressIDRaw, &stateRaw, &colorRaw, &colorListJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(colorListJSON, &entry.ColorList); err != nil {
			return nil, err
		}
		if entry.ColorList == nil {
			entry.ColorList = []uint32{}
		}
		entry.CommanderID = uint32(commanderIDRaw)
		entry.DressID = uint32(dressIDRaw)
		entry.State = uint32(stateRaw)
		entry.Color = uint32(colorRaw)
		states = append(states, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func GetIslandCommanderDressState(commanderID uint32, dressID uint32) (*IslandCommanderDressState, error) {
	var (
		commanderIDRaw int64
		dressIDRaw     int64
		stateRaw       int64
		colorRaw       int64
		colorListJSON  []byte
		entry          IslandCommanderDressState
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, dress_id, state, color, color_list
FROM island_commander_dresses
WHERE commander_id = $1 AND dress_id = $2
`, int64(commanderID), int64(dressID)).Scan(&commanderIDRaw, &dressIDRaw, &stateRaw, &colorRaw, &colorListJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(colorListJSON, &entry.ColorList); err != nil {
		return nil, err
	}
	if entry.ColorList == nil {
		entry.ColorList = []uint32{}
	}
	entry.CommanderID = uint32(commanderIDRaw)
	entry.DressID = uint32(dressIDRaw)
	entry.State = uint32(stateRaw)
	entry.Color = uint32(colorRaw)
	return &entry, nil
}

func UpsertIslandCommanderDressState(entry *IslandCommanderDressState) error {
	colorList := entry.ColorList
	if colorList == nil {
		colorList = []uint32{}
	}
	colorListJSON, err := json.Marshal(colorList)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_commander_dresses (commander_id, dress_id, state, color, color_list)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (commander_id, dress_id)
DO UPDATE SET
	state = EXCLUDED.state,
	color = EXCLUDED.color,
	color_list = EXCLUDED.color_list,
	updated_at = CURRENT_TIMESTAMP
`, int64(entry.CommanderID), int64(entry.DressID), int64(entry.State), int64(entry.Color), colorListJSON)
	return err
}

func MarkCommanderIslandDressRead(commanderID uint32, dressIDs []uint32) error {
	if len(dressIDs) == 0 {
		return nil
	}
	ctx := context.Background()
	for _, dressID := range dressIDs {
		_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO island_commander_dresses (commander_id, dress_id, state, color, color_list)
VALUES ($1, $2, 1, 0, '[]'::jsonb)
ON CONFLICT (commander_id, dress_id)
DO UPDATE SET
	state = 1,
	updated_at = CURRENT_TIMESTAMP
`, int64(commanderID), int64(dressID))
		if err != nil {
			return err
		}
	}
	return nil
}
