package orm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type IslandOrderState struct {
	CommanderID        uint32
	Favor              uint32
	DailySelect        uint32
	DailySlotNum       uint32
	TimeSlotNum        uint32
	UrgencyFinishCount uint32
	ShipRefresh        uint32
}

type IslandProsperityState struct {
	CommanderID   uint32
	Prosperity    uint32
	ClaimedLevels []uint32
}

func GetIslandOrderState(commanderID uint32) (*IslandOrderState, error) {
	ctx := context.Background()
	return queryIslandOrderState(ctx, db.DefaultStore.Pool, commanderID, false)
}

func GetIslandOrderStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandOrderState, error) {
	state, err := queryIslandOrderState(ctx, tx, commanderID, true)
	if err == nil {
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_order_states (commander_id)
VALUES ($1)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID))
	if err != nil {
		return nil, err
	}

	return queryIslandOrderState(ctx, tx, commanderID, true)
}

func SaveIslandOrderStateTx(ctx context.Context, tx pgx.Tx, state *IslandOrderState) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_order_states (commander_id, favor, daily_select, daily_slot_num, time_slot_num, urgency_finish_count, ship_refresh)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (commander_id)
DO UPDATE SET
	favor = EXCLUDED.favor,
	daily_select = EXCLUDED.daily_select,
	daily_slot_num = EXCLUDED.daily_slot_num,
	time_slot_num = EXCLUDED.time_slot_num,
	urgency_finish_count = EXCLUDED.urgency_finish_count,
	ship_refresh = EXCLUDED.ship_refresh
`,
		int64(state.CommanderID),
		int64(state.Favor),
		int64(state.DailySelect),
		int64(state.DailySlotNum),
		int64(state.TimeSlotNum),
		int64(state.UrgencyFinishCount),
		int64(state.ShipRefresh),
	)
	return err
}

func AddIslandOrderFavorTx(ctx context.Context, tx pgx.Tx, commanderID uint32, amount uint32) error {
	if amount == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
INSERT INTO island_order_states (commander_id, favor)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET favor = island_order_states.favor + EXCLUDED.favor
`, int64(commanderID), int64(amount))
	return err
}

func ListIslandOrderFavorClaimsTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]uint32, error) {
	rows, err := tx.Query(ctx, `
SELECT level
FROM island_order_favor_claims
WHERE commander_id = $1
ORDER BY level ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	levels := make([]uint32, 0)
	for rows.Next() {
		var levelRaw int64
		if err := rows.Scan(&levelRaw); err != nil {
			return nil, err
		}
		levels = append(levels, uint32(levelRaw))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return levels, nil
}

func AddIslandOrderFavorClaimTx(ctx context.Context, tx pgx.Tx, commanderID uint32, level uint32) (bool, error) {
	res, err := tx.Exec(ctx, `
INSERT INTO island_order_favor_claims (commander_id, level)
VALUES ($1, $2)
ON CONFLICT (commander_id, level) DO NOTHING
`, int64(commanderID), int64(level))
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func UpsertIslandOrderSlotTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slot *protobuf.PB_ISLAND_ORDER_SLOT) error {
	data, err := proto.Marshal(slot)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_order_slots (commander_id, slot_id, slot_data)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, slot_id)
DO UPDATE SET slot_data = EXCLUDED.slot_data
`, int64(commanderID), int64(slot.GetId()), data)
	return err
}

func DeleteIslandOrderSlotTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotID uint32) error {
	_, err := tx.Exec(ctx, `
DELETE FROM island_order_slots
WHERE commander_id = $1 AND slot_id = $2
`, int64(commanderID), int64(slotID))
	return err
}

func GetIslandOrderSlotForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotID uint32) (*protobuf.PB_ISLAND_ORDER_SLOT, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `
SELECT slot_data
FROM island_order_slots
WHERE commander_id = $1 AND slot_id = $2
FOR UPDATE
`, int64(commanderID), int64(slotID)).Scan(&raw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	var slot protobuf.PB_ISLAND_ORDER_SLOT
	if err := proto.Unmarshal(raw, &slot); err != nil {
		return nil, err
	}
	return &slot, nil
}

func ListIslandOrderSlotsTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]*protobuf.PB_ISLAND_ORDER_SLOT, error) {
	rows, err := tx.Query(ctx, `
SELECT slot_data
FROM island_order_slots
WHERE commander_id = $1
ORDER BY slot_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]*protobuf.PB_ISLAND_ORDER_SLOT, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SLOT{}
		if err := proto.Unmarshal(raw, slot); err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slots, nil
}

func UpsertIslandShipOrderSlotTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slot *protobuf.PB_ISLAND_ORDER_SHIP_SLOT) error {
	data, err := proto.Marshal(slot)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_ship_order_slots (commander_id, slot_id, slot_data)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, slot_id)
DO UPDATE SET slot_data = EXCLUDED.slot_data
`, int64(commanderID), int64(slot.GetId()), data)
	return err
}

func GetIslandShipOrderSlotForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotID uint32) (*protobuf.PB_ISLAND_ORDER_SHIP_SLOT, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `
SELECT slot_data
FROM island_ship_order_slots
WHERE commander_id = $1 AND slot_id = $2
FOR UPDATE
`, int64(commanderID), int64(slotID)).Scan(&raw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	slot := &protobuf.PB_ISLAND_ORDER_SHIP_SLOT{}
	if err := proto.Unmarshal(raw, slot); err != nil {
		return nil, err
	}
	return slot, nil
}

func ListIslandShipOrderSlotsTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]*protobuf.PB_ISLAND_ORDER_SHIP_SLOT, error) {
	rows, err := tx.Query(ctx, `
SELECT slot_data
FROM island_ship_order_slots
WHERE commander_id = $1
ORDER BY slot_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]*protobuf.PB_ISLAND_ORDER_SHIP_SLOT, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		slot := &protobuf.PB_ISLAND_ORDER_SHIP_SLOT{}
		if err := proto.Unmarshal(raw, slot); err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slots, nil
}

func UpsertIslandShipOrderAppointTx(ctx context.Context, tx pgx.Tx, commanderID uint32, appoint *protobuf.PB_SHIP_ORDER_APPOINT) error {
	data, err := proto.Marshal(appoint)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_ship_order_appoints (commander_id, appoint_id, appoint_data)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, appoint_id)
DO UPDATE SET appoint_data = EXCLUDED.appoint_data
`, int64(commanderID), int64(appoint.GetId()), data)
	return err
}

func DeleteIslandShipOrderAppointTx(ctx context.Context, tx pgx.Tx, commanderID uint32, appointID uint32) error {
	_, err := tx.Exec(ctx, `
DELETE FROM island_ship_order_appoints
WHERE commander_id = $1 AND appoint_id = $2
`, int64(commanderID), int64(appointID))
	return err
}

func GetIslandShipOrderAppointForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, appointID uint32) (*protobuf.PB_SHIP_ORDER_APPOINT, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `
SELECT appoint_data
FROM island_ship_order_appoints
WHERE commander_id = $1 AND appoint_id = $2
FOR UPDATE
`, int64(commanderID), int64(appointID)).Scan(&raw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	appoint := &protobuf.PB_SHIP_ORDER_APPOINT{}
	if err := proto.Unmarshal(raw, appoint); err != nil {
		return nil, err
	}
	return appoint, nil
}

func ListIslandShipOrderAppointsTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]*protobuf.PB_SHIP_ORDER_APPOINT, error) {
	rows, err := tx.Query(ctx, `
SELECT appoint_data
FROM island_ship_order_appoints
WHERE commander_id = $1
ORDER BY appoint_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	appoints := make([]*protobuf.PB_SHIP_ORDER_APPOINT, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		appoint := &protobuf.PB_SHIP_ORDER_APPOINT{}
		if err := proto.Unmarshal(raw, appoint); err != nil {
			return nil, err
		}
		appoints = append(appoints, appoint)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return appoints, nil
}

func ConsumeIslandInventoryTx(ctx context.Context, tx pgx.Tx, commanderID uint32, itemID uint32, count uint32) (bool, error) {
	if count == 0 {
		return true, nil
	}
	res, err := tx.Exec(ctx, `
UPDATE island_inventories
SET count = count - $3
WHERE commander_id = $1 AND item_id = $2 AND count >= $3
`, int64(commanderID), int64(itemID), int64(count))
	if err != nil {
		return false, err
	}
	if res.RowsAffected() == 0 {
		return false, nil
	}
	_, err = tx.Exec(ctx, `
DELETE FROM island_inventories
WHERE commander_id = $1 AND item_id = $2 AND count = 0
`, int64(commanderID), int64(itemID))
	if err != nil {
		return false, err
	}
	return true, nil
}

func AddIslandSeasonRewardClaimTx(ctx context.Context, tx pgx.Tx, commanderID uint32, targetPT uint32) (bool, error) {
	res, err := tx.Exec(ctx, `
INSERT INTO island_season_reward_claims (commander_id, target_pt)
VALUES ($1, $2)
ON CONFLICT (commander_id, target_pt) DO NOTHING
`, int64(commanderID), int64(targetPT))
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func ListIslandSeasonRewardClaimsTx(ctx context.Context, tx pgx.Tx, commanderID uint32) ([]uint32, error) {
	rows, err := tx.Query(ctx, `
SELECT target_pt
FROM island_season_reward_claims
WHERE commander_id = $1
ORDER BY target_pt ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]uint32, 0)
	for rows.Next() {
		var targetRaw int64
		if err := rows.Scan(&targetRaw); err != nil {
			return nil, err
		}
		values = append(values, uint32(targetRaw))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func GetIslandProsperityStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandProsperityState, error) {
	row := tx.QueryRow(ctx, `
SELECT prosperity, claimed_levels
FROM island_prosperity_states
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID))

	var prosperityRaw int64
	var claimedRaw []byte
	err := row.Scan(&prosperityRaw, &claimedRaw)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO island_prosperity_states (commander_id)
VALUES ($1)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID)); err != nil {
			return nil, err
		}
		return &IslandProsperityState{CommanderID: commanderID, ClaimedLevels: []uint32{}}, nil
	}

	state := &IslandProsperityState{
		CommanderID: commanderID,
		Prosperity:  uint32(prosperityRaw),
	}
	if len(claimedRaw) == 0 {
		state.ClaimedLevels = []uint32{}
		return state, nil
	}
	if err := json.Unmarshal(claimedRaw, &state.ClaimedLevels); err != nil {
		return nil, err
	}
	if state.ClaimedLevels == nil {
		state.ClaimedLevels = []uint32{}
	}
	return state, nil
}

func SaveIslandProsperityStateTx(ctx context.Context, tx pgx.Tx, state *IslandProsperityState) error {
	levels := append([]uint32(nil), state.ClaimedLevels...)
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	raw, err := json.Marshal(levels)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_prosperity_states (commander_id, prosperity, claimed_levels)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id)
DO UPDATE SET prosperity = EXCLUDED.prosperity, claimed_levels = EXCLUDED.claimed_levels
`, int64(state.CommanderID), int64(state.Prosperity), raw)
	return err
}

func SetIslandProsperity(commanderID uint32, prosperity uint32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_prosperity_states (commander_id, prosperity)
VALUES ($1, $2)
ON CONFLICT (commander_id)
DO UPDATE SET prosperity = EXCLUDED.prosperity
`, int64(commanderID), int64(prosperity))
	return err
}

type islandOrderStateQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandOrderState(ctx context.Context, queryer islandOrderStateQueryer, commanderID uint32, forUpdate bool) (*IslandOrderState, error) {
	query := `
SELECT commander_id, favor, daily_select, daily_slot_num, time_slot_num, urgency_finish_count, ship_refresh
FROM island_order_states
WHERE commander_id = $1
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	state := &IslandOrderState{}
	var commanderIDRaw int64
	var favorRaw int64
	var dailySelectRaw int64
	var dailySlotNumRaw int64
	var timeSlotNumRaw int64
	var urgencyRaw int64
	var shipRefreshRaw int64
	err := queryer.QueryRow(ctx, query, int64(commanderID)).Scan(
		&commanderIDRaw,
		&favorRaw,
		&dailySelectRaw,
		&dailySlotNumRaw,
		&timeSlotNumRaw,
		&urgencyRaw,
		&shipRefreshRaw,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}

	state.CommanderID = uint32(commanderIDRaw)
	state.Favor = uint32(favorRaw)
	state.DailySelect = uint32(dailySelectRaw)
	state.DailySlotNum = uint32(dailySlotNumRaw)
	state.TimeSlotNum = uint32(timeSlotNumRaw)
	state.UrgencyFinishCount = uint32(urgencyRaw)
	state.ShipRefresh = uint32(shipRefreshRaw)
	return state, nil
}
