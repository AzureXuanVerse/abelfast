package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	limitChallengeSuccessResult uint32 = 0
	limitChallengeFailureResult uint32 = 1

	limitChallengeInfoTypeMonthly uint32 = 1
)

const (
	constellationChallengeMonthCategory    = "ShareCfg/constellation_challenge_month.json"
	constellationChallengeTemplateCategory = "ShareCfg/expedition_constellation_challenge_template.json"
)

type constellationChallengeMonth struct {
	ID    uint32   `json:"id"`
	Stage []uint32 `json:"stage"`
}

type constellationChallengeTemplate struct {
	ID           uint32     `json:"id"`
	DungeonID    uint32     `json:"dungeon_id"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

func LimitChallengeInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_24020
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		response := protobuf.SC_24021{Result: proto.Uint32(limitChallengeFailureResult), Times: []*protobuf.KVDATA{}, Awards: []*protobuf.KVDATA{}, PassIds: []uint32{}}
		return client.SendMessage(24021, &response)
	}
	if payload.GetType() != limitChallengeInfoTypeMonthly {
		response := protobuf.SC_24021{Result: proto.Uint32(limitChallengeFailureResult), Times: []*protobuf.KVDATA{}, Awards: []*protobuf.KVDATA{}, PassIds: []uint32{}}
		return client.SendMessage(24021, &response)
	}

	monthConfig, err := loadCurrentConstellationChallengeMonth(time.Now().UTC())
	if err != nil {
		response := protobuf.SC_24021{Result: proto.Uint32(limitChallengeFailureResult), Times: []*protobuf.KVDATA{}, Awards: []*protobuf.KVDATA{}, PassIds: []uint32{}}
		return client.SendMessage(24021, &response)
	}
	state, err := orm.LoadLimitChallengeState(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		response := protobuf.SC_24021{Result: proto.Uint32(limitChallengeFailureResult), Times: []*protobuf.KVDATA{}, Awards: []*protobuf.KVDATA{}, PassIds: []uint32{}}
		return client.SendMessage(24021, &response)
	}

	challengeIDs := normalizedChallengeIDs(monthConfig.Stage)

	stageSet := make(map[uint32]struct{}, len(challengeIDs))
	for _, challengeID := range challengeIDs {
		stageSet[challengeID] = struct{}{}
	}
	times := []*protobuf.KVDATA{}
	awards := []*protobuf.KVDATA{}
	for _, challengeID := range challengeIDs {
		times = append(times, &protobuf.KVDATA{Key: proto.Uint32(challengeID), Value: proto.Uint32(state.BestTimes[challengeID])})
		awardValue := uint32(0)
		if state.Awarded[challengeID] {
			awardValue = 1
		}
		awards = append(awards, &protobuf.KVDATA{Key: proto.Uint32(challengeID), Value: proto.Uint32(awardValue)})
	}
	passIDs := make([]uint32, 0, len(state.PassIDs))
	for _, challengeID := range state.PassIDs {
		if _, ok := stageSet[challengeID]; ok {
			passIDs = append(passIDs, challengeID)
		}
	}
	passIDs = sortedUint32s(passIDs)

	response := protobuf.SC_24021{
		Result:  proto.Uint32(limitChallengeSuccessResult),
		Times:   times,
		Awards:  awards,
		PassIds: passIDs,
	}
	return client.SendMessage(24021, &response)
}

func loadCurrentConstellationChallengeMonth(now time.Time) (*constellationChallengeMonth, error) {
	monthID := uint32(now.UTC().Month())
	entry, err := orm.GetConfigEntry(constellationChallengeMonthCategory, strconv.FormatUint(uint64(monthID), 10))
	if err != nil {
		if !db.IsNotFound(err) {
			return nil, err
		}
		entries, listErr := orm.ListConfigEntries(constellationChallengeMonthCategory)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range entries {
			var parsed constellationChallengeMonth
			if err := json.Unmarshal(item.Data, &parsed); err != nil {
				continue
			}
			if parsed.ID == monthID {
				return &parsed, nil
			}
		}
		return nil, fmt.Errorf("constellation month %d not found", monthID)
	}
	var config constellationChallengeMonth
	if err := json.Unmarshal(entry.Data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadConstellationChallengeTemplate(challengeID uint32) (*constellationChallengeTemplate, error) {
	entry, err := orm.GetConfigEntry(constellationChallengeTemplateCategory, strconv.FormatUint(uint64(challengeID), 10))
	if err != nil {
		return nil, err
	}
	var config constellationChallengeTemplate
	if err := json.Unmarshal(entry.Data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func normalizedChallengeIDs(ids []uint32) []uint32 {
	if len(ids) == 0 {
		return []uint32{}
	}
	seen := make(map[uint32]struct{}, len(ids))
	result := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return sortedUint32s(result)
}

func sortedUint32s(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	result := make([]uint32, len(values))
	copy(result, values)
	sort.Slice(result, func(i int, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func saveLimitChallengeClear(client *connection.Client, dungeonID uint32, totalTime uint32, score uint32) error {
	if dungeonID == 0 || score == 0 {
		return nil
	}
	monthConfig, err := loadCurrentConstellationChallengeMonth(time.Now().UTC())
	if err != nil {
		return nil
	}
	challengeID, err := challengeIDForDungeon(monthConfig.Stage, dungeonID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil
		}
		return err
	}

	return orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadLimitChallengeStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}
		orm.MarkLimitChallengePass(state, challengeID, totalTime)
		return orm.SaveLimitChallengeStateTx(context.Background(), tx, state)
	})
}

func challengeIDForDungeon(challengeIDs []uint32, dungeonID uint32) (uint32, error) {
	for _, challengeID := range challengeIDs {
		template, err := loadConstellationChallengeTemplate(challengeID)
		if err != nil {
			if db.IsNotFound(err) {
				continue
			}
			return 0, err
		}
		if template.DungeonID == dungeonID {
			return challengeID, nil
		}
	}
	return 0, db.ErrNotFound
}
