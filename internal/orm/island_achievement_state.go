package orm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandAchievementProgressEntry struct {
	EventType uint32 `json:"event_type"`
	EventArg  uint32 `json:"event_arg"`
	Value     uint32 `json:"value"`
}

type IslandAchievementState struct {
	CommanderID     uint32
	ProgressEntries []IslandAchievementProgressEntry
	FinishList      []uint32
}

func (IslandAchievementState) TableName() string {
	return "island_achievement_states"
}

func NewIslandAchievementState(commanderID uint32) *IslandAchievementState {
	return &IslandAchievementState{
		CommanderID:     commanderID,
		ProgressEntries: []IslandAchievementProgressEntry{},
		FinishList:      []uint32{},
	}
}

func GetIslandAchievementState(commanderID uint32) (*IslandAchievementState, error) {
	state, err := queryIslandAchievementState(context.Background(), db.DefaultStore.Pool, commanderID, false)
	if err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func GetIslandAchievementStateForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandAchievementState, error) {
	state, err := queryIslandAchievementState(ctx, tx, commanderID, true)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_achievement_states (commander_id)
VALUES ($1)
ON CONFLICT (commander_id) DO NOTHING
`, int64(commanderID))
	if err != nil {
		return nil, err
	}

	return queryIslandAchievementState(ctx, tx, commanderID, true)
}

func SaveIslandAchievementStateTx(ctx context.Context, tx pgx.Tx, state *IslandAchievementState) error {
	progressRaw, err := marshalIslandAchievementProgress(state.ProgressEntries)
	if err != nil {
		return err
	}
	finishRaw, err := marshalIslandAchievementFinishList(state.FinishList)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO island_achievement_states (commander_id, progress_list, finish_list)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id)
DO UPDATE SET
	progress_list = EXCLUDED.progress_list,
	finish_list = EXCLUDED.finish_list,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.CommanderID), progressRaw, finishRaw)
	return err
}

func (state *IslandAchievementState) SetProgress(eventType uint32, eventArg uint32, value uint32) {
	for idx := range state.ProgressEntries {
		if state.ProgressEntries[idx].EventType != eventType || state.ProgressEntries[idx].EventArg != eventArg {
			continue
		}
		state.ProgressEntries[idx].Value = value
		return
	}
	state.ProgressEntries = append(state.ProgressEntries, IslandAchievementProgressEntry{
		EventType: eventType,
		EventArg:  eventArg,
		Value:     value,
	})
}

func (state *IslandAchievementState) ProgressValue(eventType uint32, eventArg uint32) (uint32, bool) {
	for _, entry := range state.ProgressEntries {
		if entry.EventType == eventType && entry.EventArg == eventArg {
			return entry.Value, true
		}
	}
	return 0, false
}

func (state *IslandAchievementState) HasFinished(achievementID uint32) bool {
	for _, finishedID := range state.FinishList {
		if finishedID == achievementID {
			return true
		}
	}
	return false
}

type islandAchievementQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryIslandAchievementState(ctx context.Context, queryer islandAchievementQueryer, commanderID uint32, forUpdate bool) (*IslandAchievementState, error) {
	query := `
SELECT progress_list, finish_list
FROM island_achievement_states
WHERE commander_id = $1
`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var progressRaw []byte
	var finishRaw []byte
	err := queryer.QueryRow(ctx, query, int64(commanderID)).Scan(&progressRaw, &finishRaw)
	if err != nil {
		return nil, err
	}

	progressEntries, err := unmarshalIslandAchievementProgress(progressRaw)
	if err != nil {
		return nil, err
	}
	finishList, err := unmarshalIslandAchievementFinishList(finishRaw)
	if err != nil {
		return nil, err
	}

	return &IslandAchievementState{
		CommanderID:     commanderID,
		ProgressEntries: progressEntries,
		FinishList:      finishList,
	}, nil
}

func marshalIslandAchievementProgress(entries []IslandAchievementProgressEntry) ([]byte, error) {
	if len(entries) == 0 {
		return []byte("[]"), nil
	}
	copyEntries := append([]IslandAchievementProgressEntry(nil), entries...)
	sort.Slice(copyEntries, func(i, j int) bool {
		if copyEntries[i].EventType == copyEntries[j].EventType {
			return copyEntries[i].EventArg < copyEntries[j].EventArg
		}
		return copyEntries[i].EventType < copyEntries[j].EventType
	})
	return json.Marshal(copyEntries)
}

func unmarshalIslandAchievementProgress(raw []byte) ([]IslandAchievementProgressEntry, error) {
	if len(raw) == 0 {
		return []IslandAchievementProgressEntry{}, nil
	}
	entries := []IslandAchievementProgressEntry{}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return []IslandAchievementProgressEntry{}, nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].EventType == entries[j].EventType {
			return entries[i].EventArg < entries[j].EventArg
		}
		return entries[i].EventType < entries[j].EventType
	})
	return entries, nil
}

func marshalIslandAchievementFinishList(values []uint32) ([]byte, error) {
	if len(values) == 0 {
		return []byte("[]"), nil
	}
	copyValues := append([]uint32(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	return json.Marshal(copyValues)
}

func unmarshalIslandAchievementFinishList(raw []byte) ([]uint32, error) {
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
