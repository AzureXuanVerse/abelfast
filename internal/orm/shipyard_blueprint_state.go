package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type CommanderShipyardBlueprint struct {
	CommanderID    uint32
	BlueprintID    uint32
	ShipID         uint32
	StartTime      uint32
	BluePrintLevel uint32
	Exp            uint32
	StartDuration  uint32
}

func (CommanderShipyardBlueprint) TableName() string {
	return "commander_shipyard_blueprints"
}

type CommanderShipyardState struct {
	CommanderID              uint32
	ColdTime                 uint32
	DailyCatchupStrengthen   uint32
	DailyCatchupStrengthenUR uint32
}

func (CommanderShipyardState) TableName() string {
	return "commander_shipyard_states"
}

func GetCommanderShipyardState(commanderID uint32) (*CommanderShipyardState, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT commander_id, cold_time, daily_catchup_strengthen, daily_catchup_strengthen_ur
FROM commander_shipyard_states
WHERE commander_id = $1
`, int64(commanderID))

	var state CommanderShipyardState
	err := row.Scan(&state.CommanderID, &state.ColdTime, &state.DailyCatchupStrengthen, &state.DailyCatchupStrengthenUR)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func GetOrCreateCommanderShipyardStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*CommanderShipyardState, error) {
	row := tx.QueryRow(ctx, `
INSERT INTO commander_shipyard_states (commander_id, cold_time, daily_catchup_strengthen, daily_catchup_strengthen_ur)
VALUES ($1, 0, 0, 0)
ON CONFLICT (commander_id)
DO UPDATE SET commander_id = EXCLUDED.commander_id
RETURNING commander_id, cold_time, daily_catchup_strengthen, daily_catchup_strengthen_ur
`, int64(commanderID))

	var state CommanderShipyardState
	if err := row.Scan(&state.CommanderID, &state.ColdTime, &state.DailyCatchupStrengthen, &state.DailyCatchupStrengthenUR); err != nil {
		return nil, err
	}
	return &state, nil
}

func UpsertCommanderShipyardStateTx(ctx context.Context, tx pgx.Tx, state *CommanderShipyardState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO commander_shipyard_states (commander_id, cold_time, daily_catchup_strengthen, daily_catchup_strengthen_ur)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id)
DO UPDATE SET
  cold_time = EXCLUDED.cold_time,
  daily_catchup_strengthen = EXCLUDED.daily_catchup_strengthen,
  daily_catchup_strengthen_ur = EXCLUDED.daily_catchup_strengthen_ur
`, int64(state.CommanderID), int64(state.ColdTime), int64(state.DailyCatchupStrengthen), int64(state.DailyCatchupStrengthenUR))
	return err
}

func ListCommanderShipyardBlueprints(commanderID uint32) ([]CommanderShipyardBlueprint, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT commander_id, blueprint_id, ship_id, start_time, blue_print_level, exp, start_duration
FROM commander_shipyard_blueprints
WHERE commander_id = $1
ORDER BY blueprint_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]CommanderShipyardBlueprint, 0)
	for rows.Next() {
		var row CommanderShipyardBlueprint
		if err := rows.Scan(&row.CommanderID, &row.BlueprintID, &row.ShipID, &row.StartTime, &row.BluePrintLevel, &row.Exp, &row.StartDuration); err != nil {
			return nil, err
		}
		entries = append(entries, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func GetCommanderShipyardBlueprintTx(ctx context.Context, tx pgx.Tx, commanderID uint32, blueprintID uint32) (*CommanderShipyardBlueprint, error) {
	row := tx.QueryRow(ctx, `
SELECT commander_id, blueprint_id, ship_id, start_time, blue_print_level, exp, start_duration
FROM commander_shipyard_blueprints
WHERE commander_id = $1 AND blueprint_id = $2
`, int64(commanderID), int64(blueprintID))

	var entry CommanderShipyardBlueprint
	err := row.Scan(&entry.CommanderID, &entry.BlueprintID, &entry.ShipID, &entry.StartTime, &entry.BluePrintLevel, &entry.Exp, &entry.StartDuration)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func UpsertCommanderShipyardBlueprintTx(ctx context.Context, tx pgx.Tx, entry *CommanderShipyardBlueprint) error {
	_, err := tx.Exec(ctx, `
INSERT INTO commander_shipyard_blueprints (
  commander_id,
  blueprint_id,
  ship_id,
  start_time,
  blue_print_level,
  exp,
  start_duration
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (commander_id, blueprint_id)
DO UPDATE SET
  ship_id = EXCLUDED.ship_id,
  start_time = EXCLUDED.start_time,
  blue_print_level = EXCLUDED.blue_print_level,
  exp = EXCLUDED.exp,
  start_duration = EXCLUDED.start_duration
`, int64(entry.CommanderID), int64(entry.BlueprintID), int64(entry.ShipID), int64(entry.StartTime), int64(entry.BluePrintLevel), int64(entry.Exp), int64(entry.StartDuration))
	return err
}
