package orm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandCardLabelCount struct {
	ID  uint32 `json:"id"`
	Num uint32 `json:"num"`
}

type IslandCardState struct {
	CommanderID       uint32
	Picture           string
	VisitWord         string
	SocialFlag        uint32
	LabelViewFlag     uint32
	LabelCounts       []IslandCardLabelCount
	AchieveDisplayIDs []uint32
	VisitNum          uint32
	GoodNum           uint32
	ShipNum           uint32
	BookNum           uint32
	AchievementTotal  uint32
}

func (IslandCardState) TableName() string {
	return "island_card_states"
}

func NewIslandCardState(commanderID uint32) *IslandCardState {
	return &IslandCardState{
		CommanderID:       commanderID,
		SocialFlag:        1,
		LabelViewFlag:     1,
		LabelCounts:       []IslandCardLabelCount{},
		AchieveDisplayIDs: []uint32{},
	}
}

func GetIslandCardState(commanderID uint32) (*IslandCardState, error) {
	state, err := queryIslandCardState(context.Background(), db.DefaultStore.Pool, commanderID, false)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func GetIslandCardStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandCardState, error) {
	state, err := queryIslandCardState(ctx, tx, commanderID, true)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_card_states (commander_id)
VALUES ($1)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID))
	if err != nil {
		return nil, err
	}

	return queryIslandCardState(ctx, tx, commanderID, true)
}

func SaveIslandCardStateTx(ctx context.Context, tx pgx.Tx, state *IslandCardState) error {
	labelRaw, err := marshalIslandCardLabelCounts(state.LabelCounts)
	if err != nil {
		return err
	}
	achievementRaw, err := marshalIslandCardUint32List(state.AchieveDisplayIDs)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_card_states (
	commander_id, picture, visit_word, social_flag, label_view_flag,
	label_counts, achieve_display_ids, visit_num, good_num, ship_num, book_num, achieve_num
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (commander_id)
DO UPDATE SET
	picture = EXCLUDED.picture,
	visit_word = EXCLUDED.visit_word,
	social_flag = EXCLUDED.social_flag,
	label_view_flag = EXCLUDED.label_view_flag,
	label_counts = EXCLUDED.label_counts,
	achieve_display_ids = EXCLUDED.achieve_display_ids,
	visit_num = EXCLUDED.visit_num,
	good_num = EXCLUDED.good_num,
	ship_num = EXCLUDED.ship_num,
	book_num = EXCLUDED.book_num,
	achieve_num = EXCLUDED.achieve_num,
	updated_at = CURRENT_TIMESTAMP
`,
		int64(state.CommanderID),
		state.Picture,
		state.VisitWord,
		int64(state.SocialFlag),
		int64(state.LabelViewFlag),
		labelRaw,
		achievementRaw,
		int64(state.VisitNum),
		int64(state.GoodNum),
		int64(state.ShipNum),
		int64(state.BookNum),
		int64(state.AchievementTotal),
	)
	return err
}

func UpsertIslandCardState(state *IslandCardState) error {
	return db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return SaveIslandCardStateTx(context.Background(), tx, state)
	})
}

func HasIslandCardLike(fromCommanderID uint32, toCommanderID uint32) (bool, error) {
	var count int64
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM island_card_likes
WHERE from_commander_id = $1 AND to_commander_id = $2
`, int64(fromCommanderID), int64(toCommanderID)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddIslandCardLikeTx(ctx context.Context, tx pgx.Tx, fromCommanderID uint32, toCommanderID uint32) (bool, error) {
	result, err := tx.Exec(ctx, `
INSERT INTO island_card_likes (from_commander_id, to_commander_id)
VALUES ($1, $2)
ON CONFLICT (from_commander_id, to_commander_id) DO NOTHING
`, int64(fromCommanderID), int64(toCommanderID))
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func HasIslandCardLabelGift(fromCommanderID uint32, toCommanderID uint32) (bool, error) {
	var count int64
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM island_card_label_gifts
WHERE from_commander_id = $1 AND to_commander_id = $2
`, int64(fromCommanderID), int64(toCommanderID)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddIslandCardLabelGiftTx(ctx context.Context, tx pgx.Tx, fromCommanderID uint32, toCommanderID uint32, labelID uint32) (bool, error) {
	result, err := tx.Exec(ctx, `
INSERT INTO island_card_label_gifts (from_commander_id, to_commander_id, label_id)
VALUES ($1, $2, $3)
ON CONFLICT (from_commander_id, to_commander_id) DO NOTHING
`, int64(fromCommanderID), int64(toCommanderID), int64(labelID))
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func AddIslandCardLabelCount(state *IslandCardState, labelID uint32) {
	for idx := range state.LabelCounts {
		if state.LabelCounts[idx].ID == labelID {
			state.LabelCounts[idx].Num++
			return
		}
	}
	state.LabelCounts = append(state.LabelCounts, IslandCardLabelCount{ID: labelID, Num: 1})
}

type islandCardStateQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandCardState(ctx context.Context, queryer islandCardStateQueryer, commanderID uint32, forUpdate bool) (*IslandCardState, error) {
	query := `
SELECT picture, visit_word, social_flag, label_view_flag, label_counts, achieve_display_ids,
	visit_num, good_num, ship_num, book_num, achieve_num
FROM island_card_states
WHERE commander_id = $1
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var (
		socialFlagRaw   int64
		labelFlagRaw    int64
		visitNumRaw     int64
		goodNumRaw      int64
		shipNumRaw      int64
		bookNumRaw      int64
		achieveNumRaw   int64
		labelRaw        []byte
		achievementsRaw []byte
		state           IslandCardState
	)
	err := queryer.QueryRow(ctx, query, int64(commanderID)).Scan(
		&state.Picture,
		&state.VisitWord,
		&socialFlagRaw,
		&labelFlagRaw,
		&labelRaw,
		&achievementsRaw,
		&visitNumRaw,
		&goodNumRaw,
		&shipNumRaw,
		&bookNumRaw,
		&achieveNumRaw,
	)
	if err != nil {
		return nil, err
	}
	labels, err := unmarshalIslandCardLabelCounts(labelRaw)
	if err != nil {
		return nil, err
	}
	achievementIDs, err := unmarshalIslandCardUint32List(achievementsRaw)
	if err != nil {
		return nil, err
	}

	state.CommanderID = commanderID
	state.SocialFlag = uint32(socialFlagRaw)
	state.LabelViewFlag = uint32(labelFlagRaw)
	state.LabelCounts = labels
	state.AchieveDisplayIDs = achievementIDs
	state.VisitNum = uint32(visitNumRaw)
	state.GoodNum = uint32(goodNumRaw)
	state.ShipNum = uint32(shipNumRaw)
	state.BookNum = uint32(bookNumRaw)
	state.AchievementTotal = uint32(achieveNumRaw)

	return &state, nil
}

func marshalIslandCardLabelCounts(values []IslandCardLabelCount) ([]byte, error) {
	if len(values) == 0 {
		return []byte("[]"), nil
	}
	copyValues := append([]IslandCardLabelCount(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i].ID < copyValues[j].ID })
	return json.Marshal(copyValues)
}

func unmarshalIslandCardLabelCounts(raw []byte) ([]IslandCardLabelCount, error) {
	if len(raw) == 0 {
		return []IslandCardLabelCount{}, nil
	}
	values := []IslandCardLabelCount{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []IslandCardLabelCount{}, nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func marshalIslandCardUint32List(values []uint32) ([]byte, error) {
	if len(values) == 0 {
		return []byte("[]"), nil
	}
	copyValues := append([]uint32(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	return json.Marshal(copyValues)
}

func unmarshalIslandCardUint32List(raw []byte) ([]uint32, error) {
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
