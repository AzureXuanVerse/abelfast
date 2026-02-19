package answer

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	worldBossResultFailure               = uint32(1)
	worldBossResultSuccess               = uint32(0)
	worldBossResultBossUnavailable       = uint32(1)
	worldBossResultBossDead              = uint32(3)
	worldBossResultChallengeCountReached = uint32(6)
	worldBossResultLastTimeMismatch      = uint32(20)
)

func WorldBossOtherBossLookup(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34503
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34504, err
	}

	seen := make(map[uint32]bool, len(payload.GetUserIdList()))
	bosses := make([]*protobuf.WORLDBOSS_INFO_P34, 0, len(payload.GetUserIdList()))
	for _, userID := range payload.GetUserIdList() {
		if seen[userID] {
			continue
		}
		seen[userID] = true

		state, err := orm.GetCommanderWorldBossState(userID)
		if err != nil {
			if db.IsNotFound(err) {
				continue
			}
			return 0, 34504, err
		}
		if state.SelfBoss == nil {
			continue
		}
		bosses = append(bosses, worldBossStateToProto(state.SelfBoss))
	}

	sort.Slice(bosses, func(i, j int) bool {
		if bosses[i].GetLastTime() == bosses[j].GetLastTime() {
			return bosses[i].GetId() < bosses[j].GetId()
		}
		return bosses[i].GetLastTime() > bosses[j].GetLastTime()
	})

	return client.SendMessage(34504, &protobuf.SC_34504{BossList: bosses})
}

func WorldBossDamageRank(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34505
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34506, err
	}

	state, err := resolveWorldBossStateByBossID(client.Commander.CommanderID, payload.GetBossId())
	if err != nil {
		return 0, 34506, err
	}
	if state == nil {
		return client.SendMessage(34506, &protobuf.SC_34506{RankList: []*protobuf.WORLDBOSS_RANK_P34{}})
	}

	rankEntries := state.GetRankings(payload.GetBossId())
	sort.Slice(rankEntries, func(i, j int) bool {
		if rankEntries[i].Damage == rankEntries[j].Damage {
			return rankEntries[i].CommanderID < rankEntries[j].CommanderID
		}
		return rankEntries[i].Damage > rankEntries[j].Damage
	})

	ranks := make([]*protobuf.WORLDBOSS_RANK_P34, 0, len(rankEntries))
	for _, item := range rankEntries {
		ranks = append(ranks, &protobuf.WORLDBOSS_RANK_P34{
			Id:     proto.Uint32(item.CommanderID),
			Name:   proto.String(item.Name),
			Damage: proto.Uint32(item.Damage),
		})
	}

	return client.SendMessage(34506, &protobuf.SC_34506{RankList: ranks})
}

func WorldBossSupport(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34509
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34510, err
	}

	response := &protobuf.SC_34510{Result: proto.Uint32(worldBossResultFailure)}
	if payload.GetType() < 1 || payload.GetType() > 3 {
		return client.SendMessage(34510, response)
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34510, err
	}
	if state.SelfBoss == nil {
		return client.SendMessage(34510, response)
	}

	until := uint32(time.Now().Unix()) + worldBossGamesetSeconds("joint_boss_world_time", 1800)
	switch payload.GetType() {
	case 1:
		state.FriendSupport = until
	case 2:
		state.GuildSupport = until
	case 3:
		state.WorldSupport = until
	}
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34510, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	return client.SendMessage(34510, response)
}

func WorldBossSubmitAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34511
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34512, err
	}

	response := &protobuf.SC_34512{
		Result: proto.Uint32(worldBossResultFailure),
		Drops:  []*protobuf.DROPINFO{},
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34512, err
	}
	if state.SelfBoss == nil || state.SelfBoss.ID != payload.GetBossId() {
		return client.SendMessage(34512, response)
	}
	if state.SelfBoss.Hp > 0 {
		return client.SendMessage(34512, response)
	}
	if state.SelfBoss.LastTime != 0 && uint32(time.Now().Unix()) >= state.SelfBoss.LastTime {
		return client.SendMessage(34512, response)
	}
	if state.IsRewardClaimed(payload.GetBossId()) {
		return client.SendMessage(34512, response)
	}

	state.SetRewardClaimed(payload.GetBossId(), true)
	state.SelfBoss = nil
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34512, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	response.Drops = []*protobuf.DROPINFO{{
		Type:   proto.Uint32(1),
		Id:     proto.Uint32(1),
		Number: proto.Uint32(100),
	}}
	return client.SendMessage(34512, response)
}

func WorldBossOvertimeClear(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34513
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34514, err
	}

	response := &protobuf.SC_34514{Result: proto.Uint32(worldBossResultFailure)}
	if payload.GetType() != 0 {
		return client.SendMessage(34514, response)
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34514, err
	}
	state.SelfBoss = nil
	state.AutoBattleBossID = 0
	state.AutoBattleStartTime = 0
	state.AutoFightFinishTime = 0
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34514, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	return client.SendMessage(34514, response)
}

func WorldBossStateCheck(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34515
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34516, err
	}

	result := worldBossResultBossUnavailable
	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34516, err
	}
	if state.SelfBoss != nil && state.SelfBoss.ID == payload.GetBossId() {
		now := uint32(time.Now().Unix())
		switch {
		case payload.GetLastTime() != 0 && payload.GetLastTime() != state.SelfBoss.LastTime:
			result = worldBossResultLastTimeMismatch
		case state.SelfBoss.Hp == 0:
			result = worldBossResultBossDead
		case state.SelfBoss.LastTime != 0 && now >= state.SelfBoss.LastTime:
			result = worldBossResultBossUnavailable
		case state.SelfBoss.FightCount >= 10:
			result = worldBossResultChallengeCountReached
		default:
			result = worldBossResultSuccess
		}
	}

	return client.SendMessage(34516, &protobuf.SC_34516{Result: proto.Uint32(result)})
}

func WorldBossCacheHpRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34517
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34518, err
	}

	seen := make(map[uint32]bool, len(payload.GetBossId()))
	result := make([]*protobuf.WORLDBOSS_SIMPLE, 0, len(payload.GetBossId()))
	for _, bossID := range payload.GetBossId() {
		if seen[bossID] {
			continue
		}
		seen[bossID] = true
		state, err := resolveWorldBossStateByBossID(client.Commander.CommanderID, bossID)
		if err != nil {
			return 0, 34518, err
		}
		if state == nil || state.SelfBoss == nil || state.SelfBoss.ID != bossID {
			continue
		}
		result = append(result, &protobuf.WORLDBOSS_SIMPLE{
			Id:        proto.Uint32(state.SelfBoss.ID),
			Hp:        proto.Uint32(state.SelfBoss.Hp),
			RankCount: proto.Uint32(state.SelfBoss.RankCount),
		})
	}

	return client.SendMessage(34518, &protobuf.SC_34518{List: result})
}

func WorldBossGetOtherFormation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34519
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34520, err
	}

	response := &protobuf.SC_34520{Result: proto.Uint32(worldBossResultFailure), ShipList: []*protobuf.SHIPINFO{}}

	requesterState, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34520, err
	}
	targetState, err := orm.GetCommanderWorldBossState(payload.GetUserId())
	if err != nil {
		if !db.IsNotFound(err) {
			return 0, 34520, err
		}
		return client.SendMessage(34520, response)
	}
	if (requesterState.SelfBoss == nil || requesterState.SelfBoss.ID != payload.GetBossId()) &&
		(targetState.SelfBoss == nil || targetState.SelfBoss.ID != payload.GetBossId()) {
		return client.SendMessage(34520, response)
	}

	ships := make([]*protobuf.SHIPINFO, 0, 6)
	if payload.GetUserId() == client.Commander.CommanderID {
		for i := range client.Commander.Ships {
			ships = append(ships, orm.ToProtoOwnedShip(client.Commander.Ships[i], nil, nil))
			if len(ships) >= 6 {
				break
			}
		}
	} else {
		commander, err := orm.LoadCommanderWithDetails(payload.GetUserId())
		if err != nil {
			if !errors.Is(err, db.ErrNotFound) {
				return 0, 34520, err
			}
			return client.SendMessage(34520, response)
		}
		for i := range commander.Ships {
			ships = append(ships, orm.ToProtoOwnedShip(commander.Ships[i], nil, nil))
			if len(ships) >= 6 {
				break
			}
		}
	}
	if len(ships) == 0 {
		return client.SendMessage(34520, response)
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	response.ShipList = ships
	return client.SendMessage(34520, response)
}

func ActivateWorldBoss(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34521
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34522, err
	}

	response := &protobuf.SC_34522{Result: proto.Uint32(worldBossResultFailure)}
	if payload.GetTemplateId() == 0 {
		return client.SendMessage(34522, response)
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34522, err
	}
	now := uint32(time.Now().Unix())
	if state.SelfBoss != nil && state.SelfBoss.Hp > 0 && state.SelfBoss.LastTime > now {
		return client.SendMessage(34522, response)
	}

	bossID := state.NextBossID
	if bossID == 0 {
		bossID = 1
	}
	boss := &orm.WorldBossBossState{
		ID:         bossID,
		TemplateID: payload.GetTemplateId(),
		Lv:         1,
		Hp:         1000000,
		Owner:      client.Commander.CommanderID,
		LastTime:   now + 3600,
		KillTime:   0,
		FightCount: 0,
		RankCount:  0,
	}

	state.SelfBoss = boss
	state.DefaultBossID = boss.ID
	state.SelfBossLv = boss.Lv
	state.NextBossID = boss.ID + 1
	if state.SummonPt > 0 {
		state.SummonPt--
	} else if state.SummonPtOld > 0 {
		state.SummonPtOld--
	}
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34522, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	response.Boss = worldBossStateToProto(boss)
	return client.SendMessage(34522, response)
}

func WorldBossArchivesAutoBattleStart(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34523
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34524, err
	}

	response := &protobuf.SC_34524{
		Result:              proto.Uint32(worldBossResultFailure),
		AutoFightFinishTime: proto.Uint32(0),
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34524, err
	}
	if state.SelfBoss == nil || state.SelfBoss.ID != payload.GetBossId() {
		return client.SendMessage(34524, response)
	}
	now := uint32(time.Now().Unix())
	if state.AutoFightFinishTime > now && state.AutoBattleBossID == payload.GetBossId() {
		return client.SendMessage(34524, response)
	}

	finishTime := now + worldBossGamesetSeconds("past_joint_boss_autofight_time", 900)
	state.AutoBattleBossID = payload.GetBossId()
	state.AutoBattleStartTime = now
	state.AutoFightFinishTime = finishTime
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34524, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	response.AutoFightFinishTime = proto.Uint32(finishTime)
	return client.SendMessage(34524, response)
}

func WorldBossArchivesStopAutoBattle(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34525
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34526, err
	}

	response := &protobuf.SC_34526{
		Result: proto.Uint32(worldBossResultFailure),
		Count:  proto.Uint32(0),
		Damage: proto.Uint32(0),
		Oil:    proto.Uint32(0),
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34526, err
	}
	now := uint32(time.Now().Unix())
	if state.AutoFightFinishTime == 0 || state.AutoBattleBossID != payload.GetBossId() {
		return client.SendMessage(34526, response)
	}

	start := state.AutoBattleStartTime
	if start == 0 || start > now {
		start = now
	}
	end := now
	if state.AutoFightFinishTime > 0 && state.AutoFightFinishTime < end {
		end = state.AutoFightFinishTime
	}
	if end < start {
		end = start
	}
	elapsed := end - start
	count := elapsed/60 + 1
	damage := count * 1000
	oil := count * 5

	state.AutoFightFinishTime = 0
	state.AutoBattleStartTime = 0
	state.AutoBattleBossID = 0
	if damage > state.AutoFightMaxDamage {
		state.AutoFightMaxDamage = damage
	}
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34526, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	response.Count = proto.Uint32(count)
	response.Damage = proto.Uint32(damage)
	response.Oil = proto.Uint32(oil)
	return client.SendMessage(34526, response)
}

func SwitchWorldBossArchives(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_34527
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 34528, err
	}

	response := &protobuf.SC_34528{Result: proto.Uint32(worldBossResultFailure)}
	if payload.GetBossId() == 0 {
		return client.SendMessage(34528, response)
	}

	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34528, err
	}
	state.DefaultBossID = payload.GetBossId()
	if err := orm.SaveCommanderWorldBossState(state); err != nil {
		return 0, 34528, err
	}

	response.Result = proto.Uint32(worldBossResultSuccess)
	return client.SendMessage(34528, response)
}

func worldBossGamesetSeconds(key string, fallback uint32) uint32 {
	entry, err := orm.GetConfigEntry("ShareCfg/gameset.json", key)
	if err != nil || len(entry.Data) == 0 {
		return fallback
	}

	var payload map[string]any
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return fallback
	}
	if raw, ok := payload["key_value"]; ok {
		switch value := raw.(type) {
		case float64:
			if value > 0 {
				return uint32(value)
			}
		case []any:
			if len(value) > 0 {
				if head, ok := value[0].(float64); ok && head > 0 {
					return uint32(head)
				}
			}
		}
	}
	return fallback
}

func worldBossStateToProto(boss *orm.WorldBossBossState) *protobuf.WORLDBOSS_INFO_P34 {
	if boss == nil {
		return &protobuf.WORLDBOSS_INFO_P34{
			Id:         proto.Uint32(0),
			TemplateId: proto.Uint32(0),
			Lv:         proto.Uint32(0),
			Hp:         proto.Uint32(0),
			Owner:      proto.Uint32(0),
			LastTime:   proto.Uint32(0),
			KillTime:   proto.Uint32(0),
			FightCount: proto.Uint32(0),
			RankCount:  proto.Uint32(0),
		}
	}
	return &protobuf.WORLDBOSS_INFO_P34{
		Id:         proto.Uint32(boss.ID),
		TemplateId: proto.Uint32(boss.TemplateID),
		Lv:         proto.Uint32(boss.Lv),
		Hp:         proto.Uint32(boss.Hp),
		Owner:      proto.Uint32(boss.Owner),
		LastTime:   proto.Uint32(boss.LastTime),
		KillTime:   proto.Uint32(boss.KillTime),
		FightCount: proto.Uint32(boss.FightCount),
		RankCount:  proto.Uint32(boss.RankCount),
	}
}

func resolveWorldBossStateByBossID(requesterCommanderID uint32, bossID uint32) (*orm.WorldBossState, error) {
	requesterState, err := orm.GetOrCreateCommanderWorldBossState(requesterCommanderID)
	if err != nil {
		return nil, err
	}
	if requesterState.SelfBoss != nil && requesterState.SelfBoss.ID == bossID {
		return requesterState, nil
	}
	if len(requesterState.GetRankings(bossID)) > 0 {
		return requesterState, nil
	}

	states, err := orm.ListWorldBossStates()
	if err != nil {
		return nil, err
	}
	for _, state := range states {
		if state == nil || state.CommanderID == requesterCommanderID {
			continue
		}
		if state.SelfBoss != nil && state.SelfBoss.ID == bossID {
			return state, nil
		}
	}

	return nil, nil
}
