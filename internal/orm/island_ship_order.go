package orm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type IslandShipOrderAppoint struct {
	ID       uint32     `json:"id"`
	ViewTime uint32     `json:"view_time"`
	Cost     [][]uint32 `json:"cost"`
	Reward   [][]uint32 `json:"reward"`
}

type IslandShipOrderState struct {
	CommanderID uint32
	RefreshAt   uint32
	AppointList []IslandShipOrderAppoint
}

type IslandShipOrderSlot struct {
	CommanderID uint32
	SlotID      uint32
	State       uint32
	LoadTime    uint32
	GetTime     uint32
	FinishNum   uint32
	AutoTime    uint32
}

func (IslandShipOrderState) TableName() string {
	return "island_ship_order_states"
}

func (IslandShipOrderSlot) TableName() string {
	return "island_ship_order_slots"
}

func LoadIslandShipOrderStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandShipOrderState, error) {
	row := tx.QueryRow(ctx, `
SELECT refresh_at, appoint_list
FROM island_ship_order_states
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))

	var refreshAtRaw int64
	var appointJSON []byte
	err := row.Scan(&refreshAtRaw, &appointJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		state := &IslandShipOrderState{CommanderID: commanderID, RefreshAt: 0, AppointList: []IslandShipOrderAppoint{}}
		if insertErr := SaveIslandShipOrderStateTx(ctx, tx, state); insertErr != nil {
			return nil, insertErr
		}
		return state, nil
	}
	if err != nil {
		return nil, err
	}

	list, err := unmarshalIslandShipOrderAppoints(appointJSON)
	if err != nil {
		return nil, err
	}
	return &IslandShipOrderState{CommanderID: commanderID, RefreshAt: uint32(refreshAtRaw), AppointList: list}, nil
}

func SaveIslandShipOrderStateTx(ctx context.Context, tx pgx.Tx, state *IslandShipOrderState) error {
	appointJSON, err := marshalIslandShipOrderAppoints(state.AppointList)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_ship_order_states (commander_id, refresh_at, appoint_list)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id)
DO UPDATE SET refresh_at = EXCLUDED.refresh_at, appoint_list = EXCLUDED.appoint_list
`, int64(state.CommanderID), int64(state.RefreshAt), appointJSON)
	return err
}

func UpsertIslandShipOrderSlotTx(ctx context.Context, tx pgx.Tx, slot *IslandShipOrderSlot) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_ship_order_slots (commander_id, slot_id, state, load_time, get_time, finish_num, auto_time)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (commander_id, slot_id)
DO UPDATE SET
	state = EXCLUDED.state,
	load_time = EXCLUDED.load_time,
	get_time = EXCLUDED.get_time,
	finish_num = EXCLUDED.finish_num,
	auto_time = EXCLUDED.auto_time
`,
		int64(slot.CommanderID),
		int64(slot.SlotID),
		int64(slot.State),
		int64(slot.LoadTime),
		int64(slot.GetTime),
		int64(slot.FinishNum),
		int64(slot.AutoTime),
	)
	return err
}

func LoadIslandShipOrderSlotTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotID uint32) (*IslandShipOrderSlot, error) {
	row := tx.QueryRow(ctx, `
SELECT state, load_time, get_time, finish_num, auto_time
FROM island_ship_order_slots
WHERE commander_id = $1 AND slot_id = $2
FOR UPDATE
`, int64(commanderID), int64(slotID))

	var (
		stateRaw     int64
		loadTimeRaw  int64
		getTimeRaw   int64
		finishNumRaw int64
		autoTimeRaw  int64
	)
	err := row.Scan(&stateRaw, &loadTimeRaw, &getTimeRaw, &finishNumRaw, &autoTimeRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		slot := &IslandShipOrderSlot{CommanderID: commanderID, SlotID: slotID, State: 0, LoadTime: 0, GetTime: 0, FinishNum: 0, AutoTime: 0}
		if insertErr := UpsertIslandShipOrderSlotTx(ctx, tx, slot); insertErr != nil {
			return nil, insertErr
		}
		return slot, nil
	}
	if err != nil {
		return nil, err
	}
	return &IslandShipOrderSlot{
		CommanderID: commanderID,
		SlotID:      slotID,
		State:       uint32(stateRaw),
		LoadTime:    uint32(loadTimeRaw),
		GetTime:     uint32(getTimeRaw),
		FinishNum:   uint32(finishNumRaw),
		AutoTime:    uint32(autoTimeRaw),
	}, nil
}

func marshalIslandShipOrderAppoints(list []IslandShipOrderAppoint) ([]byte, error) {
	if len(list) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(list)
}

func unmarshalIslandShipOrderAppoints(payload []byte) ([]IslandShipOrderAppoint, error) {
	if len(payload) == 0 {
		return []IslandShipOrderAppoint{}, nil
	}
	var list []IslandShipOrderAppoint
	if err := json.Unmarshal(payload, &list); err != nil {
		return nil, err
	}
	if list == nil {
		return []IslandShipOrderAppoint{}, nil
	}
	return list, nil
}
