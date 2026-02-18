package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	miniGameHubCategory            = "ShareCfg/mini_game_hub.json"
	miniGameCategory               = "ShareCfg/mini_game.json"
	miniGameHubStateCategory       = "Runtime/minigame_hub_state"
	miniGameDataStateCategory      = "Runtime/minigame_data_state"
	miniGameTelemetryStateCategory = "Runtime/minigame_telemetry_state"
	islandNodeStateCategory        = "Runtime/island_node_state"
	miniGameFriendRankLimit        = 100
)

type MiniGameHubConfig struct {
	ID            uint32   `json:"id"`
	ActID         uint32   `json:"act_id"`
	RebornTimes   uint32   `json:"reborn_times"`
	RewardNeed    uint32   `json:"reward_need"`
	RewardTarget  uint32   `json:"reward_target"`
	RewardDisplay []uint32 `json:"reward_display"`
}

type MiniGameConfig struct {
	ID    uint32 `json:"id"`
	HubID uint32 `json:"hub_id"`
}

type MiniGameScoreEntry struct {
	Score uint32 `json:"score"`
	Extra uint32 `json:"extra"`
}

type MiniGameHubState struct {
	CommanderID  uint32                        `json:"commander_id"`
	HubID        uint32                        `json:"hub_id"`
	AvailableCnt uint32                        `json:"available_cnt"`
	UsedCnt      uint32                        `json:"used_cnt"`
	Ultimate     uint32                        `json:"ultimate"`
	MaxScores    map[uint32]MiniGameScoreEntry `json:"max_scores"`
}

type MiniGameKVState struct {
	Key    uint32 `json:"key"`
	Value  uint32 `json:"value"`
	Value2 uint32 `json:"value2"`
}

type MiniGameKVListState struct {
	Key    uint32            `json:"key"`
	Values []MiniGameKVState `json:"values"`
}

type MiniGameDataState struct {
	CommanderID uint32                `json:"commander_id"`
	GameID      uint32                `json:"game_id"`
	Datas       []uint32              `json:"datas"`
	KVLists     []MiniGameKVListState `json:"kv_lists"`
}

type MiniGameTelemetryState struct {
	CommanderID uint32            `json:"commander_id"`
	GameTimes   map[uint32]uint32 `json:"game_times"`
}

type IslandNodeState struct {
	ID      uint32 `json:"id"`
	EventID uint32 `json:"event_id"`
	IsNew   uint32 `json:"is_new"`
}

func GetMiniGameHubConfig(hubID uint32) (*MiniGameHubConfig, error) {
	entry, err := GetConfigEntry(miniGameHubCategory, strconv.FormatUint(uint64(hubID), 10))
	if err != nil {
		return nil, err
	}
	config := &MiniGameHubConfig{}
	if err := json.Unmarshal(entry.Data, config); err != nil {
		return nil, err
	}
	if config.ID == 0 {
		config.ID = hubID
	}
	if config.RewardDisplay == nil {
		config.RewardDisplay = []uint32{}
	}
	return config, nil
}

func GetMiniGameConfig(gameID uint32) (*MiniGameConfig, error) {
	entry, err := GetConfigEntry(miniGameCategory, strconv.FormatUint(uint64(gameID), 10))
	if err != nil {
		return nil, err
	}
	config := &MiniGameConfig{}
	if err := json.Unmarshal(entry.Data, config); err != nil {
		return nil, err
	}
	if config.ID == 0 {
		config.ID = gameID
	}
	return config, nil
}

func GetOrCreateMiniGameHubState(commanderID uint32, config *MiniGameHubConfig) (*MiniGameHubState, error) {
	entry, err := GetConfigEntry(miniGameHubStateCategory, miniGameHubStateKey(commanderID, config.ID))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := &MiniGameHubState{
			CommanderID:  commanderID,
			HubID:        config.ID,
			AvailableCnt: config.RebornTimes,
			MaxScores:    map[uint32]MiniGameScoreEntry{},
		}
		if err := SaveMiniGameHubState(state); err != nil {
			return nil, err
		}
		return state, nil
	}
	state := &MiniGameHubState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	normalizeMiniGameHubState(state, commanderID, config)
	return state, nil
}

func SaveMiniGameHubState(state *MiniGameHubState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(miniGameHubStateCategory, miniGameHubStateKey(state.CommanderID, state.HubID), payload)
}

func GetOrCreateMiniGameDataState(commanderID uint32, gameID uint32) (*MiniGameDataState, error) {
	entry, err := GetConfigEntry(miniGameDataStateCategory, miniGameDataStateKey(commanderID, gameID))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := &MiniGameDataState{CommanderID: commanderID, GameID: gameID, Datas: []uint32{}, KVLists: []MiniGameKVListState{}}
		if err := SaveMiniGameDataState(state); err != nil {
			return nil, err
		}
		return state, nil
	}
	state := &MiniGameDataState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	normalizeMiniGameDataState(state, commanderID, gameID)
	return state, nil
}

func SaveMiniGameDataState(state *MiniGameDataState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(miniGameDataStateCategory, miniGameDataStateKey(state.CommanderID, state.GameID), payload)
}

func GetOrCreateMiniGameTelemetryState(commanderID uint32) (*MiniGameTelemetryState, error) {
	entry, err := GetConfigEntry(miniGameTelemetryStateCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		state := &MiniGameTelemetryState{CommanderID: commanderID, GameTimes: map[uint32]uint32{}}
		if err := SaveMiniGameTelemetryState(state); err != nil {
			return nil, err
		}
		return state, nil
	}
	state := &MiniGameTelemetryState{}
	if err := json.Unmarshal(entry.Data, state); err != nil {
		return nil, err
	}
	if state.GameTimes == nil {
		state.GameTimes = map[uint32]uint32{}
	}
	state.CommanderID = commanderID
	return state, nil
}

func SaveMiniGameTelemetryState(state *MiniGameTelemetryState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(miniGameTelemetryStateCategory, strconv.FormatUint(uint64(state.CommanderID), 10), payload)
}

func GetOrCreateIslandNodeState(commanderID uint32, actID uint32) ([]IslandNodeState, error) {
	entry, err := GetConfigEntry(islandNodeStateCategory, islandNodeStateKey(commanderID, actID))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		if err := SaveIslandNodeState(commanderID, actID, []IslandNodeState{}); err != nil {
			return nil, err
		}
		return []IslandNodeState{}, nil
	}
	var nodes []IslandNodeState
	if err := json.Unmarshal(entry.Data, &nodes); err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []IslandNodeState{}
	}
	sort.Slice(nodes, func(i int, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes, nil
}

func SaveIslandNodeState(commanderID uint32, actID uint32, nodes []IslandNodeState) error {
	if nodes == nil {
		nodes = []IslandNodeState{}
	}
	sort.Slice(nodes, func(i int, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	payload, err := json.Marshal(nodes)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(islandNodeStateCategory, islandNodeStateKey(commanderID, actID), payload)
}

func ListCommanderItemBalances(commanderID uint32, itemIDs []uint32) (map[uint32]uint32, error) {
	balances := make(map[uint32]uint32)
	if len(itemIDs) == 0 {
		return balances, nil
	}
	intIDs := make([]int64, 0, len(itemIDs))
	seen := make(map[uint32]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		if itemID == 0 {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		intIDs = append(intIDs, int64(itemID))
	}
	if len(intIDs) == 0 {
		return balances, nil
	}
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT item_id, count FROM commander_items
WHERE commander_id = $1 AND item_id = ANY($2)
`, int64(commanderID), intIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemIDRaw, countRaw int64
		if err := rows.Scan(&itemIDRaw, &countRaw); err != nil {
			return nil, err
		}
		if countRaw <= 0 {
			continue
		}
		balances[uint32(itemIDRaw)] += uint32(countRaw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = db.DefaultStore.Pool.Query(context.Background(), `
SELECT item_id, data FROM commander_misc_items
WHERE commander_id = $1 AND item_id = ANY($2)
`, int64(commanderID), intIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemIDRaw, countRaw int64
		if err := rows.Scan(&itemIDRaw, &countRaw); err != nil {
			return nil, err
		}
		if countRaw <= 0 {
			continue
		}
		balances[uint32(itemIDRaw)] += uint32(countRaw)
	}
	return balances, rows.Err()
}

type CommanderMiniGameScore struct {
	CommanderID uint32
	Name        string
	Score       uint32
	TimeData    uint32
	DisplayIcon uint32
	DisplaySkin uint32
	IconFrame   uint32
	ChatFrame   uint32
	IconTheme   uint32
}

func ListCommanderMiniGameScores(gameID uint32) ([]CommanderMiniGameScore, error) {
	states, err := ListConfigEntries(miniGameHubStateCategory)
	if err != nil {
		return nil, err
	}
	bestByCommander := map[uint32]MiniGameScoreEntry{}
	for _, stateEntry := range states {
		state := &MiniGameHubState{}
		if err := json.Unmarshal(stateEntry.Data, state); err != nil {
			continue
		}
		score, ok := state.MaxScores[gameID]
		if !ok || score.Score == 0 {
			continue
		}
		current, exists := bestByCommander[state.CommanderID]
		if !exists || score.Score > current.Score || (score.Score == current.Score && score.Extra < current.Extra) {
			bestByCommander[state.CommanderID] = score
		}
	}
	if len(bestByCommander) == 0 {
		return []CommanderMiniGameScore{}, nil
	}

	commanderIDs := make([]int64, 0, len(bestByCommander))
	for commanderID := range bestByCommander {
		commanderIDs = append(commanderIDs, int64(commanderID))
	}

	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, name, display_icon_id, display_skin_id, selected_icon_frame_id, selected_chat_frame_id, display_icon_theme_id
FROM commanders
WHERE commander_id = ANY($1)
`, commanderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []CommanderMiniGameScore{}
	for rows.Next() {
		var (
			commanderIDRaw int64
			name           string
			displayIconRaw int64
			displaySkinRaw int64
			iconFrameRaw   int64
			chatFrameRaw   int64
			iconThemeRaw   int64
		)
		if err := rows.Scan(&commanderIDRaw, &name, &displayIconRaw, &displaySkinRaw, &iconFrameRaw, &chatFrameRaw, &iconThemeRaw); err != nil {
			return nil, err
		}
		commanderID := uint32(commanderIDRaw)
		score, ok := bestByCommander[commanderID]
		if !ok || score.Score == 0 {
			continue
		}
		results = append(results, CommanderMiniGameScore{
			CommanderID: commanderID,
			Name:        name,
			Score:       score.Score,
			TimeData:    score.Extra,
			DisplayIcon: uint32(displayIconRaw),
			DisplaySkin: uint32(displaySkinRaw),
			IconFrame:   uint32(iconFrameRaw),
			ChatFrame:   uint32(chatFrameRaw),
			IconTheme:   uint32(iconThemeRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i int, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].TimeData != results[j].TimeData {
			return results[i].TimeData < results[j].TimeData
		}
		return results[i].CommanderID < results[j].CommanderID
	})
	if len(results) > miniGameFriendRankLimit {
		results = results[:miniGameFriendRankLimit]
	}
	return results, nil
}

func miniGameHubStateKey(commanderID uint32, hubID uint32) string {
	return fmt.Sprintf("%d:%d", commanderID, hubID)
}

func miniGameDataStateKey(commanderID uint32, gameID uint32) string {
	return fmt.Sprintf("%d:%d", commanderID, gameID)
}

func islandNodeStateKey(commanderID uint32, actID uint32) string {
	return fmt.Sprintf("%d:%d", commanderID, actID)
}

func normalizeMiniGameHubState(state *MiniGameHubState, commanderID uint32, config *MiniGameHubConfig) {
	state.CommanderID = commanderID
	state.HubID = config.ID
	if state.MaxScores == nil {
		state.MaxScores = map[uint32]MiniGameScoreEntry{}
	}
}

func normalizeMiniGameDataState(state *MiniGameDataState, commanderID uint32, gameID uint32) {
	state.CommanderID = commanderID
	state.GameID = gameID
	if state.Datas == nil {
		state.Datas = []uint32{}
	}
	if state.KVLists == nil {
		state.KVLists = []MiniGameKVListState{}
	}
}
