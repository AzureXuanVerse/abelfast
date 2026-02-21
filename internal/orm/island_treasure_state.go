package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandTreasureSellState struct {
	IslandID uint32 `json:"island_id"`
	Num      uint32 `json:"num"`
}

type IslandTreasurePriceState struct {
	Timestamp uint32 `json:"timestamp"`
	Price     uint32 `json:"price"`
}

type IslandTreasureState struct {
	CommanderID uint32
	WeekBuyNum  uint32
	SellList    []IslandTreasureSellState
	PriceList   []IslandTreasurePriceState
}

func (IslandTreasureState) TableName() string {
	return "island_treasure_states"
}

func NewIslandTreasureState(commanderID uint32) *IslandTreasureState {
	return &IslandTreasureState{
		CommanderID: commanderID,
		SellList:    []IslandTreasureSellState{},
		PriceList:   []IslandTreasurePriceState{},
	}
}

func GetIslandTreasureState(commanderID uint32) (*IslandTreasureState, error) {
	state := NewIslandTreasureState(commanderID)
	err := scanIslandTreasureState(context.Background(), db.DefaultStore.Pool, commanderID, false, state)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func GetIslandTreasureStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandTreasureState, error) {
	state := NewIslandTreasureState(commanderID)
	err := scanIslandTreasureState(ctx, tx, commanderID, true, state)
	if err == nil {
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}
	if err := UpsertIslandTreasureStateTx(ctx, tx, state); err != nil {
		return nil, err
	}
	err = scanIslandTreasureState(ctx, tx, commanderID, true, state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func UpsertIslandTreasureState(state *IslandTreasureState) error {
	ctx := context.Background()
	return upsertIslandTreasureState(ctx, db.DefaultStore.Pool, state)
}

func UpsertIslandTreasureStateTx(ctx context.Context, tx pgx.Tx, state *IslandTreasureState) error {
	return upsertIslandTreasureState(ctx, tx, state)
}

func (s *IslandTreasureState) SellCount(islandID uint32) uint32 {
	for i := range s.SellList {
		if s.SellList[i].IslandID == islandID {
			return s.SellList[i].Num
		}
	}
	return 0
}

func (s *IslandTreasureState) AddSellCount(islandID uint32, delta uint32) {
	if delta == 0 {
		return
	}
	for i := range s.SellList {
		if s.SellList[i].IslandID == islandID {
			s.SellList[i].Num += delta
			return
		}
	}
	s.SellList = append(s.SellList, IslandTreasureSellState{IslandID: islandID, Num: delta})
}

func (s *IslandTreasureState) UpsertPrice(timestamp uint32, price uint32) {
	if timestamp == 0 {
		return
	}
	for i := range s.PriceList {
		if s.PriceList[i].Timestamp == timestamp {
			s.PriceList[i].Price = price
			return
		}
	}
	s.PriceList = append(s.PriceList, IslandTreasurePriceState{Timestamp: timestamp, Price: price})
}

func scanIslandTreasureState(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, commanderID uint32, forUpdate bool, state *IslandTreasureState) error {
	query := `
SELECT commander_id, week_buy_num, sell_list, price_list
FROM island_treasure_states
WHERE commander_id = $1
`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var commanderIDRaw int64
	var weekBuyNumRaw int64
	var sellListRaw []byte
	var priceListRaw []byte
	err := queryer.QueryRow(ctx, query, int64(commanderID)).Scan(&commanderIDRaw, &weekBuyNumRaw, &sellListRaw, &priceListRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return err
	}
	state.CommanderID = uint32(commanderIDRaw)
	state.WeekBuyNum = uint32(weekBuyNumRaw)
	if len(sellListRaw) == 0 {
		state.SellList = []IslandTreasureSellState{}
	} else if err := json.Unmarshal(sellListRaw, &state.SellList); err != nil {
		return err
	}
	if len(priceListRaw) == 0 {
		state.PriceList = []IslandTreasurePriceState{}
	} else if err := json.Unmarshal(priceListRaw, &state.PriceList); err != nil {
		return err
	}
	if state.SellList == nil {
		state.SellList = []IslandTreasureSellState{}
	}
	if state.PriceList == nil {
		state.PriceList = []IslandTreasurePriceState{}
	}
	return nil
}

func upsertIslandTreasureState(ctx context.Context, queryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, state *IslandTreasureState) error {
	sellList := state.SellList
	priceList := state.PriceList
	if sellList == nil {
		sellList = []IslandTreasureSellState{}
	}
	if priceList == nil {
		priceList = []IslandTreasurePriceState{}
	}
	sellListRaw, err := json.Marshal(sellList)
	if err != nil {
		return err
	}
	priceListRaw, err := json.Marshal(priceList)
	if err != nil {
		return err
	}
	_, err = queryer.Exec(ctx, `
INSERT INTO island_treasure_states (commander_id, week_buy_num, sell_list, price_list, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (commander_id)
DO UPDATE SET
	week_buy_num = EXCLUDED.week_buy_num,
	sell_list = EXCLUDED.sell_list,
	price_list = EXCLUDED.price_list,
	updated_at = NOW()
`, int64(state.CommanderID), int64(state.WeekBuyNum), sellListRaw, priceListRaw)
	return err
}
