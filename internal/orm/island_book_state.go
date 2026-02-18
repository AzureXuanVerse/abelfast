package orm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandBookCond struct {
	CommanderID uint32 `json:"commander_id"`
	Type        uint32 `json:"type"`
	UnlockID    uint32 `json:"unlock_id"`
}

func (IslandBookCond) TableName() string {
	return "island_book_conds"
}

func AddIslandBookCondTx(ctx context.Context, tx pgx.Tx, commanderID uint32, condType uint32, unlockID uint32) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_book_conds (commander_id, type, unlock_id)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, type, unlock_id) DO NOTHING
`, int64(commanderID), int64(condType), int64(unlockID))
	return err
}

func IslandBookCondExistsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, condType uint32, unlockID uint32) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM island_book_conds
  WHERE commander_id = $1 AND type = $2 AND unlock_id = $3
)
`, int64(commanderID), int64(condType), int64(unlockID)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func ListIslandBookConds(commanderID uint32) ([]IslandBookCond, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, type, unlock_id
FROM island_book_conds
WHERE commander_id = $1
ORDER BY type ASC, unlock_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conds := make([]IslandBookCond, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var condTypeRaw int64
		var unlockIDRaw int64
		if err := rows.Scan(&commanderIDRaw, &condTypeRaw, &unlockIDRaw); err != nil {
			return nil, err
		}
		conds = append(conds, IslandBookCond{
			CommanderID: uint32(commanderIDRaw),
			Type:        uint32(condTypeRaw),
			UnlockID:    uint32(unlockIDRaw),
		})
	}

	return conds, rows.Err()
}

type IslandBookCollectLevel struct {
	Lv    uint32 `json:"lv"`
	Value uint32 `json:"value"`
}

type IslandBookCollectEntry struct {
	ID       uint32                   `json:"id"`
	Base     uint32                   `json:"base"`
	LvList   []IslandBookCollectLevel `json:"lv_list"`
	StarList []IslandBookCollectLevel `json:"star_list"`
}

type IslandBookState struct {
	CommanderID  uint32
	BookList     []uint32
	BookAwards   []uint32
	BookCollects []IslandBookCollectEntry
}

func (IslandBookState) TableName() string {
	return "island_book_states"
}

func NewIslandBookState(commanderID uint32) *IslandBookState {
	return &IslandBookState{
		CommanderID:  commanderID,
		BookList:     []uint32{},
		BookAwards:   []uint32{},
		BookCollects: []IslandBookCollectEntry{},
	}
}

func GetIslandBookState(commanderID uint32) (*IslandBookState, error) {
	state, err := queryIslandBookState(context.Background(), db.DefaultStore.Pool, commanderID, false)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func GetIslandBookStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandBookState, error) {
	state, err := queryIslandBookState(ctx, tx, commanderID, true)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_book_states (commander_id)
VALUES ($1)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID))
	if err != nil {
		return nil, err
	}

	return queryIslandBookState(ctx, tx, commanderID, true)
}

func SaveIslandBookStateTx(ctx context.Context, tx pgx.Tx, state *IslandBookState) error {
	bookListRaw, err := marshalIslandBookUint32List(state.BookList)
	if err != nil {
		return err
	}
	bookAwardsRaw, err := marshalIslandBookUint32List(state.BookAwards)
	if err != nil {
		return err
	}
	bookCollectsRaw, err := marshalIslandBookCollectList(state.BookCollects)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_book_states (commander_id, book_list, book_awards, book_collects)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id)
DO UPDATE SET
	book_list = EXCLUDED.book_list,
	book_awards = EXCLUDED.book_awards,
	book_collects = EXCLUDED.book_collects,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), bookListRaw, bookAwardsRaw, bookCollectsRaw)
	return err
}

type islandBookStateQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandBookState(ctx context.Context, queryer islandBookStateQueryer, commanderID uint32, forUpdate bool) (*IslandBookState, error) {
	query := `
SELECT book_list, book_awards, book_collects
FROM island_book_states
WHERE commander_id = $1
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var bookListRaw []byte
	var bookAwardsRaw []byte
	var bookCollectsRaw []byte
	err := queryer.QueryRow(ctx, query, int64(commanderID)).Scan(&bookListRaw, &bookAwardsRaw, &bookCollectsRaw)
	if err != nil {
		return nil, err
	}
	bookList, err := unmarshalIslandBookUint32List(bookListRaw)
	if err != nil {
		return nil, err
	}
	bookAwards, err := unmarshalIslandBookUint32List(bookAwardsRaw)
	if err != nil {
		return nil, err
	}
	bookCollects, err := unmarshalIslandBookCollectList(bookCollectsRaw)
	if err != nil {
		return nil, err
	}

	return &IslandBookState{CommanderID: commanderID, BookList: bookList, BookAwards: bookAwards, BookCollects: bookCollects}, nil
}

func marshalIslandBookUint32List(values []uint32) ([]byte, error) {
	if len(values) == 0 {
		return []byte("[]"), nil
	}
	copyValues := append([]uint32(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	return json.Marshal(copyValues)
}

func unmarshalIslandBookUint32List(raw []byte) ([]uint32, error) {
	if len(raw) == 0 {
		return []uint32{}, nil
	}
	values := []uint32{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []uint32{}, nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values, nil
}

func marshalIslandBookCollectList(values []IslandBookCollectEntry) ([]byte, error) {
	if len(values) == 0 {
		return []byte("[]"), nil
	}
	copyValues := append([]IslandBookCollectEntry(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i].ID < copyValues[j].ID })
	for idx := range copyValues {
		sort.Slice(copyValues[idx].LvList, func(i, j int) bool { return copyValues[idx].LvList[i].Lv < copyValues[idx].LvList[j].Lv })
		sort.Slice(copyValues[idx].StarList, func(i, j int) bool { return copyValues[idx].StarList[i].Lv < copyValues[idx].StarList[j].Lv })
	}
	return json.Marshal(copyValues)
}

func unmarshalIslandBookCollectList(raw []byte) ([]IslandBookCollectEntry, error) {
	if len(raw) == 0 {
		return []IslandBookCollectEntry{}, nil
	}
	values := []IslandBookCollectEntry{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []IslandBookCollectEntry{}, nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	for idx := range values {
		if values[idx].LvList == nil {
			values[idx].LvList = []IslandBookCollectLevel{}
		}
		if values[idx].StarList == nil {
			values[idx].StarList = []IslandBookCollectLevel{}
		}
		sort.Slice(values[idx].LvList, func(i, j int) bool { return values[idx].LvList[i].Lv < values[idx].LvList[j].Lv })
		sort.Slice(values[idx].StarList, func(i, j int) bool { return values[idx].StarList[i].Lv < values[idx].StarList[j].Lv })
	}
	return values, nil
}
