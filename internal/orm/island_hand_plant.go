package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandHandPlant struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	BuildID     uint32 `gorm:"column:build_id"`
	SlotID      uint32 `gorm:"primaryKey;column:slot_id"`
	State       uint32 `gorm:"column:state"`
	FormulaID   uint32 `gorm:"column:formula_id"`
	StartTime   uint32 `gorm:"column:start_time"`
	EndTime     uint32 `gorm:"column:end_time"`
}

func (IslandHandPlant) TableName() string {
	return "island_hand_plants"
}

func EnsureIslandHandPlantRowsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotIDs []uint32) error {
	if len(slotIDs) == 0 {
		return nil
	}

	slotIDList := make([]int64, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		slotIDList = append(slotIDList, int64(slotID))
	}

	_, err := tx.Exec(ctx, `
INSERT INTO island_hand_plants
	(commander_id, build_id, slot_id, state, formula_id, start_time, end_time)
SELECT
	$1, 0, slot_id, 0, 0, 0, 0
FROM unnest($2::bigint[]) AS slot_id
ON CONFLICT (commander_id, slot_id)
DO NOTHING
`, int64(commanderID), slotIDList)
	return err
}

func ListIslandHandPlantsBySlotIDsForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotIDs []uint32) ([]IslandHandPlant, error) {
	return listIslandHandPlantsBySlotIDsTx(ctx, tx, commanderID, slotIDs, true)
}

func ListIslandHandPlantsBySlotIDsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotIDs []uint32) ([]IslandHandPlant, error) {
	return listIslandHandPlantsBySlotIDsTx(ctx, tx, commanderID, slotIDs, false)
}

func listIslandHandPlantsBySlotIDsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotIDs []uint32, forUpdate bool) ([]IslandHandPlant, error) {
	if len(slotIDs) == 0 {
		return []IslandHandPlant{}, nil
	}

	query := `
SELECT commander_id, build_id, slot_id, state, formula_id, start_time, end_time
FROM island_hand_plants
WHERE commander_id = $1 AND slot_id = ANY($2)
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	slotIDList := make([]int64, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		slotIDList = append(slotIDList, int64(slotID))
	}

	rows, err := tx.Query(ctx, query, int64(commanderID), slotIDList)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]IslandHandPlant, 0, len(slotIDs))
	for rows.Next() {
		var commanderIDRaw int64
		var buildIDRaw int64
		var slotIDRaw int64
		var stateRaw int64
		var formulaIDRaw int64
		var startTimeRaw int64
		var endTimeRaw int64

		if err := rows.Scan(&commanderIDRaw, &buildIDRaw, &slotIDRaw, &stateRaw, &formulaIDRaw, &startTimeRaw, &endTimeRaw); err != nil {
			return nil, err
		}

		out = append(out, IslandHandPlant{
			CommanderID: uint32(commanderIDRaw),
			BuildID:     uint32(buildIDRaw),
			SlotID:      uint32(slotIDRaw),
			State:       uint32(stateRaw),
			FormulaID:   uint32(formulaIDRaw),
			StartTime:   uint32(startTimeRaw),
			EndTime:     uint32(endTimeRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func UpsertIslandHandPlantTx(ctx context.Context, tx pgx.Tx, value *IslandHandPlant) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_hand_plants
	(commander_id, build_id, slot_id, state, formula_id, start_time, end_time)
VALUES
	($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (commander_id, slot_id)
DO UPDATE SET
	build_id = EXCLUDED.build_id,
	state = EXCLUDED.state,
	formula_id = EXCLUDED.formula_id,
	start_time = EXCLUDED.start_time,
	end_time = EXCLUDED.end_time
`, int64(value.CommanderID), int64(value.BuildID), int64(value.SlotID), int64(value.State), int64(value.FormulaID), int64(value.StartTime), int64(value.EndTime))
	return err
}

func ResetIslandHandPlantsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotIDs []uint32) error {
	if len(slotIDs) == 0 {
		return nil
	}

	slotIDList := make([]int64, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		slotIDList = append(slotIDList, int64(slotID))
	}

	_, err := tx.Exec(ctx, `
UPDATE island_hand_plants
SET state = 0, formula_id = 0, start_time = 0, end_time = 0
WHERE commander_id = $1 AND slot_id = ANY($2)
`, int64(commanderID), slotIDList)
	return err
}

func ListIslandHandPlantsByBuild(commanderID uint32, buildID uint32) ([]IslandHandPlant, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, build_id, slot_id, state, formula_id, start_time, end_time
FROM island_hand_plants
WHERE commander_id = $1 AND build_id = $2
ORDER BY slot_id ASC
`, int64(commanderID), int64(buildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]IslandHandPlant, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var buildIDRaw int64
		var slotIDRaw int64
		var stateRaw int64
		var formulaIDRaw int64
		var startTimeRaw int64
		var endTimeRaw int64

		if err := rows.Scan(&commanderIDRaw, &buildIDRaw, &slotIDRaw, &stateRaw, &formulaIDRaw, &startTimeRaw, &endTimeRaw); err != nil {
			return nil, err
		}

		out = append(out, IslandHandPlant{
			CommanderID: uint32(commanderIDRaw),
			BuildID:     uint32(buildIDRaw),
			SlotID:      uint32(slotIDRaw),
			State:       uint32(stateRaw),
			FormulaID:   uint32(formulaIDRaw),
			StartTime:   uint32(startTimeRaw),
			EndTime:     uint32(endTimeRaw),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
