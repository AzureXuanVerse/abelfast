package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandShipOrderCost struct {
	ID    uint32 `json:"id"`
	Num   uint32 `json:"num"`
	State uint32 `json:"state"`
}

type IslandShipOrderSlot struct {
	CommanderID uint32
	ShipSlotID  uint32
	State       uint32
	GetTime     uint32
	EndTime     uint32
	CostList    []IslandShipOrderCost
}

func (IslandShipOrderSlot) TableName() string {
	return "island_ship_order_slots"
}

func UpsertIslandShipOrderSlot(slot *IslandShipOrderSlot) error {
	return upsertIslandShipOrderSlotWithExecer(context.Background(), db.DefaultStore.Pool, slot)
}

func UpsertIslandRuntimeShipOrderSlotTx(ctx context.Context, tx pgx.Tx, slot *IslandShipOrderSlot) error {
	return upsertIslandShipOrderSlotWithExecer(ctx, tx, slot)
}

type islandShipOrderExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func upsertIslandShipOrderSlotWithExecer(ctx context.Context, execer islandShipOrderExecer, slot *IslandShipOrderSlot) error {
	costBytes, err := json.Marshal(slot.CostList)
	if err != nil {
		return err
	}
	_, err = execer.Exec(ctx, `
INSERT INTO island_ship_order_slots (commander_id, slot_id, slot_data, ship_slot_id, state, get_time, end_time, cost_list)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (commander_id, slot_id)
DO UPDATE SET
	ship_slot_id = EXCLUDED.ship_slot_id,
	state = EXCLUDED.state,
	get_time = EXCLUDED.get_time,
	end_time = EXCLUDED.end_time,
	cost_list = EXCLUDED.cost_list
`, int64(slot.CommanderID), int64(slot.ShipSlotID), []byte{}, int64(slot.ShipSlotID), int64(slot.State), int64(slot.GetTime), int64(slot.EndTime), costBytes)
	return err
}

func GetIslandRuntimeShipOrderSlotForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, shipSlotID uint32) (*IslandShipOrderSlot, error) {
	var commanderIDRaw int64
	var shipSlotIDRaw int64
	var stateRaw int64
	var getTimeRaw int64
	var endTimeRaw int64
	var costRaw []byte
	err := tx.QueryRow(ctx, `
SELECT commander_id, COALESCE(ship_slot_id, slot_id), state, get_time, end_time, cost_list
FROM island_ship_order_slots
WHERE commander_id = $1 AND COALESCE(ship_slot_id, slot_id) = $2
FOR UPDATE
`, int64(commanderID), int64(shipSlotID)).Scan(&commanderIDRaw, &shipSlotIDRaw, &stateRaw, &getTimeRaw, &endTimeRaw, &costRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	costList := make([]IslandShipOrderCost, 0)
	if len(costRaw) > 0 {
		if err := json.Unmarshal(costRaw, &costList); err != nil {
			return nil, err
		}
	}
	return &IslandShipOrderSlot{
		CommanderID: uint32(commanderIDRaw),
		ShipSlotID:  uint32(shipSlotIDRaw),
		State:       uint32(stateRaw),
		GetTime:     uint32(getTimeRaw),
		EndTime:     uint32(endTimeRaw),
		CostList:    costList,
	}, nil
}
