package orm

import (
	"context"
	"encoding/json"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandShipSkinState struct {
	CommanderID uint32
	ShipID      uint32
	SkinID      uint32
	ColorID     uint32
	ColorList   []uint32
}

func ListIslandShipSkinStates(commanderID uint32) ([]IslandShipSkinState, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, ship_id, skin_id, color_id, color_list
FROM island_ship_skins
WHERE commander_id = $1
ORDER BY ship_id, skin_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]IslandShipSkinState, 0)
	for rows.Next() {
		var (
			commanderIDRaw int64
			shipIDRaw      int64
			skinIDRaw      int64
			colorIDRaw     int64
			colorsJSON     []byte
			entry          IslandShipSkinState
		)
		if err := rows.Scan(&commanderIDRaw, &shipIDRaw, &skinIDRaw, &colorIDRaw, &colorsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(colorsJSON, &entry.ColorList); err != nil {
			return nil, err
		}
		if entry.ColorList == nil {
			entry.ColorList = []uint32{}
		}
		entry.CommanderID = uint32(commanderIDRaw)
		entry.ShipID = uint32(shipIDRaw)
		entry.SkinID = uint32(skinIDRaw)
		entry.ColorID = uint32(colorIDRaw)
		states = append(states, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func GetIslandShipSkinState(commanderID uint32, shipID uint32, skinID uint32) (*IslandShipSkinState, error) {
	var (
		commanderIDRaw int64
		shipIDRaw      int64
		skinIDRaw      int64
		colorIDRaw     int64
		colorsJSON     []byte
		state          IslandShipSkinState
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, ship_id, skin_id, color_id, color_list
FROM island_ship_skins
WHERE commander_id = $1 AND ship_id = $2 AND skin_id = $3
`, int64(commanderID), int64(shipID), int64(skinID)).Scan(
		&commanderIDRaw,
		&shipIDRaw,
		&skinIDRaw,
		&colorIDRaw,
		&colorsJSON,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(colorsJSON, &state.ColorList); err != nil {
		return nil, err
	}
	if state.ColorList == nil {
		state.ColorList = []uint32{}
	}
	state.CommanderID = uint32(commanderIDRaw)
	state.ShipID = uint32(shipIDRaw)
	state.SkinID = uint32(skinIDRaw)
	state.ColorID = uint32(colorIDRaw)
	return &state, nil
}

func UpsertIslandShipSkinState(state *IslandShipSkinState) error {
	colors := state.ColorList
	if colors == nil {
		colors = []uint32{}
	}
	colorJSON, err := json.Marshal(colors)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_ship_skins (commander_id, ship_id, skin_id, color_id, color_list)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (commander_id, ship_id, skin_id)
DO UPDATE SET
	color_id = EXCLUDED.color_id,
	color_list = EXCLUDED.color_list,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.ShipID), int64(state.SkinID), int64(state.ColorID), colorJSON)
	return err
}
