package orm

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandWildGatherCollectState struct {
	IslandID             uint32
	GatherID             uint32
	CollectorCommanderID uint32
	CollectedAt          time.Time
}

type IslandCollectFragmentState struct {
	IslandID             uint32
	FragmentID           uint32
	CollectorCommanderID uint32
	Mark                 uint32
	CollectedAt          time.Time
}

type IslandCollectFragmentSignState struct {
	IslandID          uint32
	FragmentID        uint32
	SignerCommanderID uint32
	Mark              uint32
	UpdatedAt         time.Time
}

type IslandCollectionCompleteState struct {
	CommanderID uint32
	CollectID   uint32
	CompletedAt time.Time
}

type IslandSlotCollectState struct {
	CommanderID     uint32
	BuildID         uint32
	AreaID          uint32
	SlotType        uint32
	NextRefreshTime uint32
	CollectedCount  uint32
	Consumed        bool
	UpdatedAt       time.Time
}

func CreateIslandWildGatherCollectStateTx(ctx context.Context, tx pgx.Tx, islandID uint32, gatherID uint32, collectorCommanderID uint32) (bool, error) {
	commandTag, err := tx.Exec(ctx, `
INSERT INTO island_wild_gather_collect_states (island_id, gather_id, collector_commander_id, collected_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (island_id, gather_id) DO NOTHING
`, int64(islandID), int64(gatherID), int64(collectorCommanderID))
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() == 1, nil
}

func CreateIslandCollectFragmentStateTx(ctx context.Context, tx pgx.Tx, islandID uint32, fragmentID uint32, collectorCommanderID uint32, mark uint32) (bool, error) {
	commandTag, err := tx.Exec(ctx, `
INSERT INTO island_collect_fragment_states (island_id, fragment_id, collector_commander_id, mark, collected_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (island_id, fragment_id) DO NOTHING
`, int64(islandID), int64(fragmentID), int64(collectorCommanderID), int64(mark))
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() == 1, nil
}

func UpsertIslandCollectFragmentSignState(state *IslandCollectFragmentSignState) error {
	if db.DefaultStore == nil {
		return fmt.Errorf("database is not initialized")
	}
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO island_collect_fragment_sign_states (island_id, fragment_id, signer_commander_id, mark, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (island_id, fragment_id, signer_commander_id)
DO UPDATE SET
  mark = EXCLUDED.mark,
  updated_at = NOW()
`, int64(state.IslandID), int64(state.FragmentID), int64(state.SignerCommanderID), int64(state.Mark))
	return err
}

func IsIslandCollectionCompletedTx(ctx context.Context, tx pgx.Tx, commanderID uint32, collectID uint32) (bool, error) {
	var marker int
	err := tx.QueryRow(ctx, `
SELECT 1
FROM island_collection_complete_states
WHERE commander_id = $1 AND collect_id = $2
`, int64(commanderID), int64(collectID)).Scan(&marker)
	err = db.MapNotFound(err)
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func MarkIslandCollectionCompletedTx(ctx context.Context, tx pgx.Tx, commanderID uint32, collectID uint32) (bool, error) {
	commandTag, err := tx.Exec(ctx, `
INSERT INTO island_collection_complete_states (commander_id, collect_id, completed_at)
VALUES ($1, $2, NOW())
ON CONFLICT (commander_id, collect_id) DO NOTHING
`, int64(commanderID), int64(collectID))
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() == 1, nil
}

func HasIslandCollectFragmentTx(ctx context.Context, tx pgx.Tx, islandID uint32, fragmentID uint32) (bool, error) {
	var marker int
	err := tx.QueryRow(ctx, `
SELECT 1
FROM island_collect_fragment_states
WHERE island_id = $1 AND fragment_id = $2
`, int64(islandID), int64(fragmentID)).Scan(&marker)
	err = db.MapNotFound(err)
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GetIslandSlotCollectStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, buildID uint32, areaID uint32, slotType uint32) (*IslandSlotCollectState, error) {
	var (
		nextRefreshTimeRaw int64
		collectedCountRaw  int64
		state              IslandSlotCollectState
	)
	err := tx.QueryRow(ctx, `
SELECT next_refresh_time, collected_count, consumed, updated_at
FROM island_slot_collect_states
WHERE commander_id = $1 AND build_id = $2 AND area_id = $3 AND slot_type = $4
FOR UPDATE
`, int64(commanderID), int64(buildID), int64(areaID), int64(slotType)).Scan(&nextRefreshTimeRaw, &collectedCountRaw, &state.Consumed, &state.UpdatedAt)
	err = db.MapNotFound(err)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	state.CommanderID = commanderID
	state.BuildID = buildID
	state.AreaID = areaID
	state.SlotType = slotType
	state.NextRefreshTime = uint32(nextRefreshTimeRaw)
	state.CollectedCount = uint32(collectedCountRaw)
	return &state, nil
}

func UpsertIslandSlotCollectStateTx(ctx context.Context, tx pgx.Tx, state *IslandSlotCollectState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_slot_collect_states (commander_id, build_id, area_id, slot_type, next_refresh_time, collected_count, consumed, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (commander_id, build_id, area_id, slot_type)
DO UPDATE SET
  next_refresh_time = EXCLUDED.next_refresh_time,
  collected_count = EXCLUDED.collected_count,
  consumed = EXCLUDED.consumed,
  updated_at = NOW()
`, int64(state.CommanderID), int64(state.BuildID), int64(state.AreaID), int64(state.SlotType), int64(state.NextRefreshTime), int64(state.CollectedCount), state.Consumed)
	return err
}
