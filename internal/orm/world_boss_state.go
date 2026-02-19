package orm

import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/ggmolly/belfast/internal/db"
)

const worldBossStateCategory = "runtime/world_boss_state"

var (
	worldBossStateMemoryMu sync.RWMutex
	worldBossStateMemory   = map[uint32]*WorldBossState{}
)

type WorldBossBossState struct {
	ID         uint32 `json:"id"`
	TemplateID uint32 `json:"template_id"`
	Lv         uint32 `json:"lv"`
	Hp         uint32 `json:"hp"`
	Owner      uint32 `json:"owner"`
	LastTime   uint32 `json:"last_time"`
	KillTime   uint32 `json:"kill_time"`
	FightCount uint32 `json:"fight_count"`
	RankCount  uint32 `json:"rank_count"`
}

type WorldBossRankEntry struct {
	CommanderID uint32 `json:"commander_id"`
	Name        string `json:"name"`
	Damage      uint32 `json:"damage"`
}

type WorldBossState struct {
	CommanderID          uint32                          `json:"commander_id"`
	FightCount           uint32                          `json:"fight_count"`
	FightCountUpdateTime uint32                          `json:"fight_count_update_time"`
	SelfBoss             *WorldBossBossState             `json:"self_boss,omitempty"`
	SummonPt             uint32                          `json:"summon_pt"`
	SummonPtOld          uint32                          `json:"summon_pt_old"`
	SummonPtDailyAcc     uint32                          `json:"summon_pt_daily_acc"`
	SummonPtOldDailyAcc  uint32                          `json:"summon_pt_old_daily_acc"`
	SummonFree           uint32                          `json:"summon_free"`
	AutoFightFinishTime  uint32                          `json:"auto_fight_finish_time"`
	DefaultBossID        uint32                          `json:"default_boss_id"`
	AutoFightMaxDamage   uint32                          `json:"auto_fight_max_damage"`
	GuildSupport         uint32                          `json:"guild_support"`
	FriendSupport        uint32                          `json:"friend_support"`
	WorldSupport         uint32                          `json:"world_support"`
	SelfBossLv           uint32                          `json:"self_boss_lv"`
	NextBossID           uint32                          `json:"next_boss_id"`
	Rankings             map[string][]WorldBossRankEntry `json:"rankings"`
	RewardClaimed        map[string]bool                 `json:"reward_claimed"`
	AutoBattleStartTime  uint32                          `json:"auto_battle_start_time"`
	AutoBattleBossID     uint32                          `json:"auto_battle_boss_id"`
}

func GetCommanderWorldBossState(commanderID uint32) (*WorldBossState, error) {
	if db.DefaultStore == nil {
		worldBossStateMemoryMu.RLock()
		cached := worldBossStateMemory[commanderID]
		worldBossStateMemoryMu.RUnlock()
		if cached == nil {
			return nil, db.ErrNotFound
		}
		state := cloneWorldBossState(cached)
		state.ensureDefaults(commanderID)
		return state, nil
	}

	entry, err := GetConfigEntry(worldBossStateCategory, strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		return nil, err
	}

	state := &WorldBossState{}
	if len(entry.Data) != 0 {
		if err := json.Unmarshal(entry.Data, state); err != nil {
			return nil, err
		}
	}
	state.ensureDefaults(commanderID)
	return state, nil
}

func GetOrCreateCommanderWorldBossState(commanderID uint32) (*WorldBossState, error) {
	state, err := GetCommanderWorldBossState(commanderID)
	if err == nil {
		return state, nil
	}
	if !db.IsNotFound(err) {
		return nil, err
	}

	state = defaultWorldBossState(commanderID)
	if err := SaveCommanderWorldBossState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func SaveCommanderWorldBossState(state *WorldBossState) error {
	state.ensureDefaults(state.CommanderID)
	copyState := cloneWorldBossState(state)
	worldBossStateMemoryMu.Lock()
	worldBossStateMemory[state.CommanderID] = copyState
	worldBossStateMemoryMu.Unlock()

	if db.DefaultStore == nil {
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return UpsertConfigEntry(worldBossStateCategory, strconv.FormatUint(uint64(state.CommanderID), 10), data)
}

func ListWorldBossStates() ([]*WorldBossState, error) {
	if db.DefaultStore == nil {
		worldBossStateMemoryMu.RLock()
		states := make([]*WorldBossState, 0, len(worldBossStateMemory))
		for commanderID, state := range worldBossStateMemory {
			cloned := cloneWorldBossState(state)
			cloned.ensureDefaults(commanderID)
			states = append(states, cloned)
		}
		worldBossStateMemoryMu.RUnlock()
		return states, nil
	}

	entries, err := ListConfigEntries(worldBossStateCategory)
	if err != nil {
		return nil, err
	}
	states := make([]*WorldBossState, 0, len(entries))
	for _, entry := range entries {
		commanderID, parseErr := strconv.ParseUint(entry.Key, 10, 32)
		if parseErr != nil {
			continue
		}
		state := &WorldBossState{}
		if len(entry.Data) != 0 {
			if unmarshalErr := json.Unmarshal(entry.Data, state); unmarshalErr != nil {
				continue
			}
		}
		state.ensureDefaults(uint32(commanderID))
		states = append(states, state)
	}
	return states, nil
}

func (state *WorldBossState) GetRankings(bossID uint32) []WorldBossRankEntry {
	state.ensureDefaults(state.CommanderID)
	values := state.Rankings[strconv.FormatUint(uint64(bossID), 10)]
	if len(values) == 0 {
		return []WorldBossRankEntry{}
	}
	out := make([]WorldBossRankEntry, len(values))
	copy(out, values)
	return out
}

func (state *WorldBossState) SetRankings(bossID uint32, values []WorldBossRankEntry) {
	state.ensureDefaults(state.CommanderID)
	key := strconv.FormatUint(uint64(bossID), 10)
	if len(values) == 0 {
		delete(state.Rankings, key)
		return
	}
	out := make([]WorldBossRankEntry, len(values))
	copy(out, values)
	state.Rankings[key] = out
}

func (state *WorldBossState) IsRewardClaimed(bossID uint32) bool {
	state.ensureDefaults(state.CommanderID)
	return state.RewardClaimed[strconv.FormatUint(uint64(bossID), 10)]
}

func (state *WorldBossState) SetRewardClaimed(bossID uint32, claimed bool) {
	state.ensureDefaults(state.CommanderID)
	key := strconv.FormatUint(uint64(bossID), 10)
	if claimed {
		state.RewardClaimed[key] = true
		return
	}
	delete(state.RewardClaimed, key)
}

func defaultWorldBossState(commanderID uint32) *WorldBossState {
	return &WorldBossState{
		CommanderID:   commanderID,
		SummonPt:      1,
		SummonPtOld:   1,
		Rankings:      map[string][]WorldBossRankEntry{},
		RewardClaimed: map[string]bool{},
		NextBossID:    1,
	}
}

func (state *WorldBossState) ensureDefaults(commanderID uint32) {
	if state.CommanderID == 0 {
		state.CommanderID = commanderID
	}
	if state.Rankings == nil {
		state.Rankings = map[string][]WorldBossRankEntry{}
	}
	if state.RewardClaimed == nil {
		state.RewardClaimed = map[string]bool{}
	}
	if state.NextBossID == 0 {
		state.NextBossID = 1
	}
}

func cloneWorldBossState(state *WorldBossState) *WorldBossState {
	if state == nil {
		return nil
	}
	cloned := *state
	if state.SelfBoss != nil {
		selfBoss := *state.SelfBoss
		cloned.SelfBoss = &selfBoss
	}
	cloned.Rankings = make(map[string][]WorldBossRankEntry, len(state.Rankings))
	for bossID, entries := range state.Rankings {
		entryCopy := make([]WorldBossRankEntry, len(entries))
		copy(entryCopy, entries)
		cloned.Rankings[bossID] = entryCopy
	}
	cloned.RewardClaimed = make(map[string]bool, len(state.RewardClaimed))
	for bossID, claimed := range state.RewardClaimed {
		cloned.RewardClaimed[bossID] = claimed
	}
	return &cloned
}
