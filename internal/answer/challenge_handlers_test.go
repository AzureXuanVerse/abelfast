package answer

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestChallengeInitialAndInfoFlow(t *testing.T) {
	client := setupConfigTest(t)
	seedChallengeConfig(t)
	seedChallengeShips(t, client)
	seedChallengeCommanders(t, client, []uint32{301, 302})

	request := protobuf.CS_24002{
		ActivityId: proto.Uint32(1),
		Mode:       proto.Uint32(challengeModeCasual),
		GroupList: []*protobuf.GROUPINFO_P24{
			{Id: proto.Uint32(1), ShipList: []uint32{101, 102}, Commanders: []*protobuf.COMMANDERSINFO{{Pos: proto.Uint32(1), Id: proto.Uint32(301)}}},
			{Id: proto.Uint32(11), ShipList: []uint32{201}, Commanders: []*protobuf.COMMANDERSINFO{{Pos: proto.Uint32(1), Id: proto.Uint32(302)}}},
		},
	}
	data, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	buffer := data
	if _, _, err := ChallengeInitial(&buffer, client); err != nil {
		t.Fatalf("challenge initial failed: %v", err)
	}
	var initial protobuf.SC_24003
	decodeResponse(t, client, &initial)
	if initial.GetResult() != 0 {
		t.Fatalf("expected challenge initial success")
	}
	client.Buffer.Reset()

	infoRequest := protobuf.CS_24004{ActivityId: proto.Uint32(1)}
	data, err = proto.Marshal(&infoRequest)
	if err != nil {
		t.Fatalf("marshal challenge info failed: %v", err)
	}
	buffer = data
	if _, _, err := ChallengeInfo(&buffer, client); err != nil {
		t.Fatalf("challenge info failed: %v", err)
	}
	var info protobuf.SC_24005
	decodeResponse(t, client, &info)
	if len(info.GetUserChallenge()) != 1 {
		t.Fatalf("expected one user challenge entry, got %d", len(info.GetUserChallenge()))
	}
	entry := info.GetUserChallenge()[0]
	if entry.GetMode() != challengeModeCasual {
		t.Fatalf("expected casual mode state")
	}
	if len(entry.GetGroupincList()) != 2 {
		t.Fatalf("expected two challenge groups")
	}
}

func TestChallengeInitialInvalidShipFails(t *testing.T) {
	client := setupConfigTest(t)
	seedChallengeConfig(t)
	seedChallengeShips(t, client)

	request := protobuf.CS_24002{
		ActivityId: proto.Uint32(1),
		Mode:       proto.Uint32(challengeModeCasual),
		GroupList: []*protobuf.GROUPINFO_P24{
			{Id: proto.Uint32(1), ShipList: []uint32{999}},
			{Id: proto.Uint32(11), ShipList: []uint32{201}},
		},
	}
	data, _ := proto.Marshal(&request)
	buffer := data
	if _, _, err := ChallengeInitial(&buffer, client); err != nil {
		t.Fatalf("challenge initial failed: %v", err)
	}
	var response protobuf.SC_24003
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected challenge initial failure")
	}
}

func TestChallengeResetClearsModeState(t *testing.T) {
	client := setupConfigTest(t)
	seedChallengeConfig(t)
	seedChallengeShips(t, client)

	state := &orm.ChallengeModeState{
		CommanderID:      client.Commander.CommanderID,
		ActivityID:       1,
		Mode:             challengeModeCasual,
		SeasonID:         1,
		Level:            1,
		RegularGroupID:   1,
		SubmarineGroupID: 11,
		RegularShipIDs:   []uint32{101},
		SubmarineShipIDs: []uint32{201},
	}
	if err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.UpsertChallengeModeStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("seed challenge state failed: %v", err)
	}

	request := protobuf.CS_24011{ActivityId: proto.Uint32(1), Mode: proto.Uint32(challengeModeCasual)}
	data, _ := proto.Marshal(&request)
	buffer := data
	if _, _, err := ChallengeReset(&buffer, client); err != nil {
		t.Fatalf("challenge reset failed: %v", err)
	}
	var response protobuf.SC_24012
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected challenge reset success")
	}
}

func TestChallengeSettleUpdatesScore(t *testing.T) {
	client := setupConfigTest(t)
	seedChallengeConfig(t)

	state := &orm.ChallengeModeState{
		CommanderID:      client.Commander.CommanderID,
		ActivityID:       1,
		Mode:             challengeModeCasual,
		SeasonID:         1,
		Level:            1,
		CurrentScore:     10,
		RegularGroupID:   1,
		SubmarineGroupID: 11,
	}
	if err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.UpsertChallengeModeStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("seed challenge state failed: %v", err)
	}

	payload := []byte{}
	payload = protowire.AppendTag(payload, 1, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 1)
	payload = protowire.AppendTag(payload, 2, protowire.VarintType)
	payload = protowire.AppendVarint(payload, uint64(challengeModeCasual))
	payload = protowire.AppendTag(payload, 3, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 50)

	buffer := payload
	if _, _, err := ChallengeSettle(&buffer, client); err != nil {
		t.Fatalf("challenge settle failed: %v", err)
	}
	var response protobuf.SC_24010
	decodeResponse(t, client, &response)
	if response.GetScore() != 50 {
		t.Fatalf("expected settle score 50")
	}

	states, err := orm.ListChallengeModeStates(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatalf("load states failed: %v", err)
	}
	if len(states) != 1 || states[0].CurrentScore != 60 {
		t.Fatalf("expected current score to be updated")
	}
}

func seedChallengeConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, "ShareCfg/activity_template.json", "1", `{"id":1,"type":37,"config_id":1}`)
	seedConfigEntry(t, "ShareCfg/activity_event_challenge.json", "1", `{"id":1,"buff":[9],"infinite_stage":[[[10001,10002,10003,10004,10005]]]}`)
}

func seedChallengeShips(t *testing.T, client *connection.Client) {
	t.Helper()
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES (1001, 'Ship A', 'Ship A', 2, 1, 1, 1, 1) ON CONFLICT (template_id) DO NOTHING")
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES (1002, 'Ship B', 'Ship B', 2, 1, 1, 1, 1) ON CONFLICT (template_id) DO NOTHING")
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES (1003, 'Ship C', 'Ship C', 2, 1, 1, 1, 1) ON CONFLICT (template_id) DO NOTHING")
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id, level, max_level, energy, create_time, change_name_timestamp) VALUES ($1, 1001, 101, 1, 100, 150, NOW(), NOW()) ON CONFLICT DO NOTHING", int64(client.Commander.CommanderID))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id, level, max_level, energy, create_time, change_name_timestamp) VALUES ($1, 1002, 102, 1, 100, 150, NOW(), NOW()) ON CONFLICT DO NOTHING", int64(client.Commander.CommanderID))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id, level, max_level, energy, create_time, change_name_timestamp) VALUES ($1, 1003, 201, 1, 100, 150, NOW(), NOW()) ON CONFLICT DO NOTHING", int64(client.Commander.CommanderID))
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("load commander failed: %v", err)
	}
}

func seedChallengeCommanders(t *testing.T, client *connection.Client, commanderIDs []uint32) {
	t.Helper()
	for _, commanderID := range commanderIDs {
		execAnswerTestSQLT(t, "INSERT INTO commander_meows (id, commander_id, template_id, level, exp, is_locked, used_pt) VALUES ($1, $2, 1, 1, 0, 0, 0) ON CONFLICT DO NOTHING", int64(commanderID), int64(client.Commander.CommanderID))
	}
}
