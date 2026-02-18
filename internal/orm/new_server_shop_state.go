package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggmolly/belfast/internal/db"
)

type NewServerShopGoodsState struct {
	ID           uint32   `json:"id"`
	Count        uint32   `json:"count"`
	BoughtRecord []uint32 `json:"bought_record"`
}

type NewServerShopState struct {
	CommanderID uint32
	ActivityID  uint32
	Goods       []NewServerShopGoodsState
}

func (NewServerShopState) TableName() string {
	return "new_server_shop_states"
}

func GetNewServerShopState(commanderID uint32, activityID uint32) (*NewServerShopState, error) {
	state := &NewServerShopState{}
	var goodsJSON []byte
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, activity_id, goods
FROM new_server_shop_states
WHERE commander_id = $1 AND activity_id = $2
`, int64(commanderID), int64(activityID)).Scan(&state.CommanderID, &state.ActivityID, &goodsJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(goodsJSON, &state.Goods); err != nil {
		return nil, err
	}
	if state.Goods == nil {
		state.Goods = []NewServerShopGoodsState{}
	}
	for i := range state.Goods {
		if state.Goods[i].BoughtRecord == nil {
			state.Goods[i].BoughtRecord = []uint32{}
		}
	}
	return state, nil
}

func GetNewServerShopStateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, activityID uint32) (*NewServerShopState, error) {
	state := &NewServerShopState{}
	var goodsJSON []byte
	err := tx.QueryRow(ctx, `
SELECT commander_id, activity_id, goods
FROM new_server_shop_states
WHERE commander_id = $1 AND activity_id = $2
FOR UPDATE
`, int64(commanderID), int64(activityID)).Scan(&state.CommanderID, &state.ActivityID, &goodsJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(goodsJSON, &state.Goods); err != nil {
		return nil, err
	}
	if state.Goods == nil {
		state.Goods = []NewServerShopGoodsState{}
	}
	for i := range state.Goods {
		if state.Goods[i].BoughtRecord == nil {
			state.Goods[i].BoughtRecord = []uint32{}
		}
	}
	return state, nil
}

func UpsertNewServerShopState(state *NewServerShopState) error {
	return upsertNewServerShopStateWithTx(context.Background(), db.DefaultStore.Pool, state)
}

func UpsertNewServerShopStateTx(ctx context.Context, tx pgx.Tx, state *NewServerShopState) error {
	return upsertNewServerShopStateWithTx(ctx, tx, state)
}

type newServerShopStateExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func upsertNewServerShopStateWithTx(ctx context.Context, execer newServerShopStateExecer, state *NewServerShopState) error {
	if state == nil {
		return nil
	}
	goods := state.Goods
	if goods == nil {
		goods = []NewServerShopGoodsState{}
	}
	for i := range goods {
		if goods[i].BoughtRecord == nil {
			goods[i].BoughtRecord = []uint32{}
		}
	}
	goodsJSON, err := json.Marshal(goods)
	if err != nil {
		return err
	}
	_, err = execer.Exec(ctx, `
INSERT INTO new_server_shop_states (commander_id, activity_id, goods)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, activity_id)
DO UPDATE SET
  goods = EXCLUDED.goods,
  updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), int64(state.ActivityID), goodsJSON)
	return err
}
