package orm

import (
	"context"
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandWildGatherSignState struct {
	IslandID          uint32
	GatherID          uint32
	SignerCommanderID uint32
	Mark              uint32
	UpdatedAt         time.Time
}

func UpsertIslandWildGatherSignState(state *IslandWildGatherSignState) error {
	if db.DefaultStore == nil {
		return fmt.Errorf("database is not initialized")
	}
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO island_wild_gather_sign_states (island_id, gather_id, signer_commander_id, mark, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (island_id, gather_id, signer_commander_id)
DO UPDATE SET
  mark = EXCLUDED.mark,
  updated_at = NOW()
`, int64(state.IslandID), int64(state.GatherID), int64(state.SignerCommanderID), int64(state.Mark))
	return err
}

func ListIslandWildGatherSignStates(islandID uint32, gatherID uint32) ([]IslandWildGatherSignState, error) {
	if db.DefaultStore == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT island_id, gather_id, signer_commander_id, mark, updated_at
FROM island_wild_gather_sign_states
WHERE island_id = $1 AND gather_id = $2
ORDER BY signer_commander_id
`, int64(islandID), int64(gatherID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]IslandWildGatherSignState, 0)
	for rows.Next() {
		var state IslandWildGatherSignState
		if err := rows.Scan(&state.IslandID, &state.GatherID, &state.SignerCommanderID, &state.Mark, &state.UpdatedAt); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}
