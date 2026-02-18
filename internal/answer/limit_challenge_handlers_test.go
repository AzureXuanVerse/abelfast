package answer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestLimitChallengeInfoReturnsState(t *testing.T) {
	client := setupConfigTest(t)
	seedLimitChallengeConfig(t, uint32(time.Now().UTC().Month()), []uint32{1001, 0, 1002})

	if err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadLimitChallengeStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}
		state.BestTimes = map[uint32]uint32{1001: 120}
		state.Awarded = map[uint32]bool{1002: true}
		state.PassIDs = []uint32{1001, 1002}
		return orm.SaveLimitChallengeStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("seed state failed: %v", err)
	}

	request := protobuf.CS_24020{Type: proto.Uint32(limitChallengeInfoTypeMonthly)}
	data, _ := proto.Marshal(&request)
	buffer := data
	if _, _, err := LimitChallengeInfo(&buffer, client); err != nil {
		t.Fatalf("limit challenge info failed: %v", err)
	}

	var response protobuf.SC_24021
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result")
	}
	if len(response.GetTimes()) != 2 || len(response.GetAwards()) != 2 {
		t.Fatalf("expected times and awards for monthly challenges")
	}
	for _, kv := range response.GetTimes() {
		if kv.GetKey() == 0 {
			t.Fatalf("expected zero challenge ID to be excluded from times")
		}
	}
	for _, kv := range response.GetAwards() {
		if kv.GetKey() == 0 {
			t.Fatalf("expected zero challenge ID to be excluded from awards")
		}
	}
	if len(response.GetPassIds()) != 2 {
		t.Fatalf("expected two pass ids")
	}
}

func TestLimitChallengeInfoUnsupportedTypeFails(t *testing.T) {
	client := setupConfigTest(t)
	request := protobuf.CS_24020{Type: proto.Uint32(2)}
	data, _ := proto.Marshal(&request)
	buffer := data
	if _, _, err := LimitChallengeInfo(&buffer, client); err != nil {
		t.Fatalf("limit challenge info failed: %v", err)
	}
	var response protobuf.SC_24021
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure result")
	}
}

func TestLimitChallengeAwardHappyPathAndReplay(t *testing.T) {
	client := setupConfigTest(t)
	month := uint32(time.Now().UTC().Month())
	seedLimitChallengeConfig(t, month, []uint32{1001})

	if err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadLimitChallengeStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}
		state.PassIDs = []uint32{1001}
		state.BestTimes = map[uint32]uint32{1001: 120}
		return orm.SaveLimitChallengeStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("seed limit challenge state failed: %v", err)
	}

	before := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = 1), 0)", int64(client.Commander.CommanderID))

	request := protobuf.CS_24022{Challengeids: []uint32{1001}}
	data, _ := proto.Marshal(&request)
	buffer := data
	if _, _, err := LimitChallengeAward(&buffer, client); err != nil {
		t.Fatalf("limit challenge award failed: %v", err)
	}
	var response protobuf.SC_24023
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected award success")
	}
	if len(response.GetDropList()) == 0 {
		t.Fatalf("expected reward drop list")
	}

	after := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = 1", int64(client.Commander.CommanderID))
	if after <= before {
		t.Fatalf("expected resource increase")
	}

	client.Buffer.Reset()
	buffer = data
	if _, _, err := LimitChallengeAward(&buffer, client); err != nil {
		t.Fatalf("limit challenge replay award failed: %v", err)
	}
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected replay claim failure")
	}
}

func TestLimitChallengeAwardRejectsInvalidAndNotPassed(t *testing.T) {
	client := setupConfigTest(t)
	month := uint32(time.Now().UTC().Month())
	seedLimitChallengeConfig(t, month, []uint32{1001})

	invalidReq := protobuf.CS_24022{Challengeids: []uint32{9999}}
	data, _ := proto.Marshal(&invalidReq)
	buffer := data
	if _, _, err := LimitChallengeAward(&buffer, client); err != nil {
		t.Fatalf("invalid award request failed: %v", err)
	}
	var response protobuf.SC_24023
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid challenge claim failure")
	}

	notPassedReq := protobuf.CS_24022{Challengeids: []uint32{1001}}
	client.Buffer.Reset()
	data, _ = proto.Marshal(&notPassedReq)
	buffer = data
	if _, _, err := LimitChallengeAward(&buffer, client); err != nil {
		t.Fatalf("not passed request failed: %v", err)
	}
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected not passed claim failure")
	}
}

func TestSaveLimitChallengeClearTracksBestTime(t *testing.T) {
	client := setupConfigTest(t)
	month := uint32(time.Now().UTC().Month())
	seedLimitChallengeConfig(t, month, []uint32{1001})

	if err := saveLimitChallengeClear(client, 5103, 150, 3); err != nil {
		t.Fatalf("save clear failed: %v", err)
	}
	if err := saveLimitChallengeClear(client, 5103, 180, 3); err != nil {
		t.Fatalf("save clear worse time failed: %v", err)
	}
	if err := saveLimitChallengeClear(client, 5103, 90, 3); err != nil {
		t.Fatalf("save clear better time failed: %v", err)
	}

	state, err := orm.LoadLimitChallengeState(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load limit challenge state failed: %v", err)
	}
	if len(state.PassIDs) != 1 || state.PassIDs[0] != 1001 {
		t.Fatalf("expected pass id to be recorded")
	}
	if state.BestTimes[1001] != 90 {
		t.Fatalf("expected best time to keep lowest clear time")
	}
}

func seedLimitChallengeConfig(t *testing.T, month uint32, stage []uint32) {
	t.Helper()
	seedConfigEntry(t, constellationChallengeMonthCategory, fmt.Sprintf("%d", month), fmt.Sprintf(`{"id":%d,"stage":[%d,%d,%d]}`,
		month,
		stageValue(stage, 0),
		stageValue(stage, 1),
		stageValue(stage, 2),
	))
	for _, challengeID := range stage {
		if challengeID == 0 {
			continue
		}
		seedConfigEntry(t, constellationChallengeTemplateCategory, fmt.Sprintf("%d", challengeID), fmt.Sprintf(`{"id":%d,"dungeon_id":5103,"award_display":[[1,1,10]]}`,
			challengeID,
		))
	}
}

func stageValue(stage []uint32, idx int) uint32 {
	if idx >= len(stage) {
		return 0
	}
	return stage[idx]
}
