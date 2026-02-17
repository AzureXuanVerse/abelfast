package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandShopGoodsState struct {
	ID  uint32 `json:"id"`
	Num uint32 `json:"num"`
}

type IslandShopState struct {
	CommanderID  uint32
	ShopID       uint32
	ExistTime    uint32
	RefreshTime  uint32
	RefreshCount uint32
	Goods        []IslandShopGoodsState
}

func (IslandShopState) TableName() string {
	return "island_shop_states"
}

func GetIslandShopState(commanderID uint32, shopID uint32) (*IslandShopState, error) {
	var (
		commanderIDRaw  int64
		shopIDRaw       int64
		existTimeRaw    int64
		refreshTimeRaw  int64
		refreshCountRaw int64
		goodsJSON       []byte
		state           IslandShopState
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, shop_id, exist_time, refresh_time, refresh_count, goods
FROM island_shop_states
WHERE commander_id = $1 AND shop_id = $2
`, int64(commanderID), int64(shopID)).Scan(
		&commanderIDRaw,
		&shopIDRaw,
		&existTimeRaw,
		&refreshTimeRaw,
		&refreshCountRaw,
		&goodsJSON,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(goodsJSON, &state.Goods); err != nil {
		return nil, err
	}
	if state.Goods == nil {
		state.Goods = []IslandShopGoodsState{}
	}
	state.CommanderID = uint32(commanderIDRaw)
	state.ShopID = uint32(shopIDRaw)
	state.ExistTime = uint32(existTimeRaw)
	state.RefreshTime = uint32(refreshTimeRaw)
	state.RefreshCount = uint32(refreshCountRaw)
	return &state, nil
}

func ListIslandShopStates(commanderID uint32) ([]IslandShopState, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, shop_id, exist_time, refresh_time, refresh_count, goods
FROM island_shop_states
WHERE commander_id = $1
ORDER BY shop_id
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := []IslandShopState{}
	for rows.Next() {
		var (
			commanderIDRaw  int64
			shopIDRaw       int64
			existTimeRaw    int64
			refreshTimeRaw  int64
			refreshCountRaw int64
			goodsJSON       []byte
			state           IslandShopState
		)
		if err := rows.Scan(&commanderIDRaw, &shopIDRaw, &existTimeRaw, &refreshTimeRaw, &refreshCountRaw, &goodsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(goodsJSON, &state.Goods); err != nil {
			return nil, err
		}
		if state.Goods == nil {
			state.Goods = []IslandShopGoodsState{}
		}
		state.CommanderID = uint32(commanderIDRaw)
		state.ShopID = uint32(shopIDRaw)
		state.ExistTime = uint32(existTimeRaw)
		state.RefreshTime = uint32(refreshTimeRaw)
		state.RefreshCount = uint32(refreshCountRaw)
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func UpsertIslandShopState(state *IslandShopState) error {
	return upsertIslandShopStateWithExecer(context.Background(), db.DefaultStore.Pool, state)
}

func UpsertIslandShopStateTx(ctx context.Context, tx pgx.Tx, state *IslandShopState) error {
	return upsertIslandShopStateWithExecer(ctx, tx, state)
}

type islandShopStateExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func upsertIslandShopStateWithExecer(ctx context.Context, execer islandShopStateExecer, state *IslandShopState) error {
	goods := state.Goods
	if goods == nil {
		goods = []IslandShopGoodsState{}
	}
	goodsJSON, err := json.Marshal(goods)
	if err != nil {
		return err
	}
	_, err = execer.Exec(ctx, `
INSERT INTO island_shop_states (commander_id, shop_id, exist_time, refresh_time, refresh_count, goods)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (commander_id, shop_id)
DO UPDATE SET
	exist_time = EXCLUDED.exist_time,
	refresh_time = EXCLUDED.refresh_time,
	refresh_count = EXCLUDED.refresh_count,
	goods = EXCLUDED.goods,
	updated_at = CURRENT_TIMESTAMP
`,
		int64(state.CommanderID),
		int64(state.ShopID),
		int64(state.ExistTime),
		int64(state.RefreshTime),
		int64(state.RefreshCount),
		goodsJSON,
	)
	return err
}
