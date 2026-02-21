package answer_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedGuildCoreConfig(t *testing.T) {
	t.Helper()
	execAnswerExternalTestSQLT(t, "INSERT INTO resources (id, item_id, name) VALUES (4, 0, 'gem') ON CONFLICT (id) DO UPDATE SET item_id = EXCLUDED.item_id, name = EXCLUDED.name")
	if err := orm.UpsertConfigEntry("ShareCfg/gameset.json", "create_guild_cost", json.RawMessage(`{"key_value":300}`)); err != nil {
		t.Fatalf("seed create_guild_cost: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/gameset.json", "modify_guild_cost", json.RawMessage(`{"key_value":100}`)); err != nil {
		t.Fatalf("seed modify_guild_cost: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/guildset.json", "base_capital", json.RawMessage(`{"key_value":20000}`)); err != nil {
		t.Fatalf("seed base_capital: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/guildset.json", "guild_tech_default", json.RawMessage(`{"key_value":1000}`)); err != nil {
		t.Fatalf("seed guild_tech_default: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/guild_data_level.json", "1", json.RawMessage(`{"assistant_commander":1,"member_num":30}`)); err != nil {
		t.Fatalf("seed guild_data_level: %v", err)
	}
}

func createGuildCommander(t *testing.T, commanderID uint32) *orm.Commander {
	t.Helper()
	name := fmt.Sprintf("GuildCore-%d", commanderID)
	if err := orm.CreateCommanderRoot(commanderID, commanderID, name, 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	execAnswerExternalTestSQLT(t, "INSERT INTO owned_resources (commander_id, resource_id, amount) VALUES ($1, 4, 1000) ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = EXCLUDED.amount", int64(commanderID))
	commander := &orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return commander
}

func cleanupGuildCoreData(t *testing.T, commanderIDs ...uint32) {
	t.Helper()
	for _, commanderID := range commanderIDs {
		execAnswerExternalTestSQLT(t, "DELETE FROM guild_join_requests WHERE applicant_commander_id = $1", int64(commanderID))
		execAnswerExternalTestSQLT(t, "DELETE FROM commander_guild_states WHERE commander_id = $1", int64(commanderID))
		execAnswerExternalTestSQLT(t, "DELETE FROM guild_user_infos WHERE commander_id = $1", int64(commanderID))
		execAnswerExternalTestSQLT(t, "DELETE FROM owned_resources WHERE commander_id = $1", int64(commanderID))
		execAnswerExternalTestSQLT(t, "DELETE FROM commanders WHERE commander_id = $1", int64(commanderID))
	}
}

func TestGuildCreateAndHydrate(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	commanderID := uint32(86001)
	otherID := uint32(86002)
	cleanupGuildCoreData(t, commanderID, otherID)
	defer cleanupGuildCoreData(t, commanderID, otherID)

	client := &connection.Client{Commander: createGuildCommander(t, commanderID)}
	guildName := fmt.Sprintf("NSTAR-%d", commanderID)
	payload := &protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(2),
		Name:      proto.String(guildName),
		Manifesto: proto.String("For science"),
	}
	buf, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := answer.CreateGuild(&buf, client); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	createResponse := &protobuf.SC_60002{}
	decodeTestPacket(t, client, 60002, createResponse)
	if createResponse.GetResult() != 0 {
		t.Fatalf("expected create result 0, got %d", createResponse.GetResult())
	}
	if createResponse.GetId() == 0 {
		t.Fatalf("expected created guild id")
	}
	if client.Commander.GetResourceCount(4) != 700 {
		t.Fatalf("expected gems reduced to 700, got %d", client.Commander.GetResourceCount(4))
	}

	empty := []byte{}
	if _, _, err := answer.CommanderGuildData(&empty, client); err != nil {
		t.Fatalf("CommanderGuildData failed: %v", err)
	}
	guildData := &protobuf.SC_60000{}
	decodeTestPacket(t, client, 60000, guildData)
	if guildData.GetGuild().GetBase().GetId() != createResponse.GetId() {
		t.Fatalf("expected guild data id %d, got %d", createResponse.GetId(), guildData.GetGuild().GetBase().GetId())
	}
	if guildData.GetGuild().GetBase().GetName() != guildName {
		t.Fatalf("unexpected guild name %q", guildData.GetGuild().GetBase().GetName())
	}

	if _, _, err := answer.GuildGetUserInfoCommand(&empty, client); err != nil {
		t.Fatalf("GuildGetUserInfoCommand failed: %v", err)
	}
	userInfo := &protobuf.SC_60103{}
	decodeTestPacket(t, client, 60103, userInfo)
	if userInfo.GetUserInfo().GetDonateCount() != 0 {
		t.Fatalf("expected default donate_count=0")
	}

	otherClient := &connection.Client{Commander: createGuildCommander(t, otherID)}
	duplicateBuf, _ := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(strings.ToLower(guildName)),
		Manifesto: proto.String("Duplicate"),
	})
	if _, _, err := answer.CreateGuild(&duplicateBuf, otherClient); err != nil {
		t.Fatalf("CreateGuild duplicate failed: %v", err)
	}
	duplicate := &protobuf.SC_60002{}
	decodeTestPacket(t, otherClient, 60002, duplicate)
	if duplicate.GetResult() != 2015 {
		t.Fatalf("expected duplicate result 2015, got %d", duplicate.GetResult())
	}
}

func TestGuildAdminOperations(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86101)
	deputyID := uint32(86102)
	memberID := uint32(86103)
	cleanupGuildCoreData(t, leaderID, deputyID, memberID)
	defer cleanupGuildCoreData(t, leaderID, deputyID, memberID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID), Hash: 11}
	deputyClient := &connection.Client{Commander: createGuildCommander(t, deputyID), Hash: 12}
	memberClient := &connection.Client{Commander: createGuildCommander(t, memberID), Hash: 13}
	guildName := fmt.Sprintf("SKY-%d", leaderID)

	createBuf, _ := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(guildName),
		Manifesto: proto.String("Skyline manifest"),
	})
	if _, _, err := answer.CreateGuild(&createBuf, leaderClient); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	created := &protobuf.SC_60002{}
	decodeTestPacket(t, leaderClient, 60002, created)
	if created.GetResult() != 0 {
		t.Fatalf("expected create success, got %d", created.GetResult())
	}
	guildID := created.GetId()

	nowUnix := uint32(time.Now().Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 4, 0, $3, $3)", int64(guildID), int64(deputyID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 5, 0, $3, $3)", int64(guildID), int64(memberID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(deputyID), int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(memberID), int64(guildID))
	execAnswerExternalTestSQLT(t, "UPDATE guilds SET member_count = 3 WHERE id = $1", int64(guildID))

	server := connection.NewServer("127.0.0.1", 0, func(pkt *[]byte, c *connection.Client, size int) {})
	server.AddClient(leaderClient)
	server.AddClient(deputyClient)
	server.AddClient(memberClient)

	setDutyBuf, _ := proto.Marshal(&protobuf.CS_60012{PlayerId: proto.Uint32(deputyID), DutyId: proto.Uint32(2)})
	if _, _, err := answer.SetGuildDuty(&setDutyBuf, leaderClient); err != nil {
		t.Fatalf("SetGuildDuty failed: %v", err)
	}
	setDuty := &protobuf.SC_60013{}
	decodeTestPacket(t, leaderClient, 60013, setDuty)
	if setDuty.GetResult() != 0 {
		t.Fatalf("expected set duty success, got %d", setDuty.GetResult())
	}

	modifyBuf, _ := proto.Marshal(&protobuf.CS_60026{Type: proto.Uint32(5), Str: proto.String("new notice")})
	if _, _, err := answer.ModifyGuildInfo(&modifyBuf, leaderClient); err != nil {
		t.Fatalf("ModifyGuildInfo failed: %v", err)
	}
	modify := &protobuf.SC_60027{}
	decodeTestPacket(t, leaderClient, 60027, modify)
	if modify.GetResult() != 0 {
		t.Fatalf("expected modify success, got %d", modify.GetResult())
	}
	push := &protobuf.SC_60030{}
	decodeTestPacket(t, deputyClient, 60030, push)
	if push.GetGuild().GetAnnounce() != "new notice" {
		t.Fatalf("expected announce push, got %q", push.GetGuild().GetAnnounce())
	}

	fireBuf, _ := proto.Marshal(&protobuf.CS_60014{PlayerId: proto.Uint32(memberID)})
	if _, _, err := answer.GuildFire(&fireBuf, leaderClient); err != nil {
		t.Fatalf("GuildFire failed: %v", err)
	}
	fireResp := &protobuf.SC_60015{}
	decodeTestPacket(t, leaderClient, 60015, fireResp)
	if fireResp.GetResult() != 0 {
		t.Fatalf("expected fire success, got %d", fireResp.GetResult())
	}

	quitBuf, _ := proto.Marshal(&protobuf.CS_60018{Id: proto.Uint32(guildID)})
	if _, _, err := answer.GuildQuit(&quitBuf, deputyClient); err != nil {
		t.Fatalf("GuildQuit failed: %v", err)
	}
	quitResp := &protobuf.SC_60019{}
	decodeTestPacket(t, deputyClient, 60019, quitResp)
	if quitResp.GetResult() != 0 {
		t.Fatalf("expected quit success, got %d", quitResp.GetResult())
	}
	waitTime, err := orm.GetCommanderGuildWaitTime(deputyID)
	if err != nil {
		t.Fatalf("GetCommanderGuildWaitTime failed: %v", err)
	}
	if waitTime <= uint32(time.Now().Unix()) {
		t.Fatalf("expected wait time in future")
	}

	dissolveBuf, _ := proto.Marshal(&protobuf.CS_60010{Id: proto.Uint32(guildID)})
	if _, _, err := answer.GuildDissolve(&dissolveBuf, leaderClient); err != nil {
		t.Fatalf("GuildDissolve failed: %v", err)
	}
	dissolveResp := &protobuf.SC_60011{}
	decodeTestPacket(t, leaderClient, 60011, dissolveResp)
	if dissolveResp.GetResult() != 0 {
		t.Fatalf("expected dissolve success, got %d", dissolveResp.GetResult())
	}
	if _, err := orm.GetGuildByID(guildID); err == nil {
		t.Fatalf("expected dissolved guild to be hidden")
	}
}

func TestGuildImpeach(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86201)
	deputyID := uint32(86202)
	cleanupGuildCoreData(t, leaderID, deputyID)
	defer cleanupGuildCoreData(t, leaderID, deputyID)

	leader := createGuildCommander(t, leaderID)
	deputy := createGuildCommander(t, deputyID)
	leaderClient := &connection.Client{Commander: leader}
	deputyClient := &connection.Client{Commander: deputy}
	guildName := fmt.Sprintf("OFF-%d", leaderID)

	createBuf, _ := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(guildName),
		Manifesto: proto.String("test"),
	})
	if _, _, err := answer.CreateGuild(&createBuf, leaderClient); err != nil {
		t.Fatalf("create guild: %v", err)
	}
	createResp := &protobuf.SC_60002{}
	decodeTestPacket(t, leaderClient, 60002, createResp)
	if createResp.GetResult() != 0 {
		t.Fatalf("expected create success, got %d", createResp.GetResult())
	}
	guildID := createResp.GetId()

	nowUnix := uint32(time.Now().Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 2, 0, $3, $3)", int64(guildID), int64(deputyID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(deputyID), int64(guildID))
	execAnswerExternalTestSQLT(t, "UPDATE commanders SET last_login = NOW() - INTERVAL '11 days' WHERE commander_id = $1", int64(leaderID))

	impeachBuf, _ := proto.Marshal(&protobuf.CS_60016{PlayerId: proto.Uint32(leaderID)})
	if _, _, err := answer.GuildImpeach(&impeachBuf, deputyClient); err != nil {
		t.Fatalf("GuildImpeach failed: %v", err)
	}
	impeachResp := &protobuf.SC_60017{}
	decodeTestPacket(t, deputyClient, 60017, impeachResp)
	if impeachResp.GetResult() != 0 {
		t.Fatalf("expected impeach success, got %d", impeachResp.GetResult())
	}
	var cooldown int64
	if err := db.DefaultStore.Pool.QueryRow(t.Context(), "SELECT kick_leader_cd FROM guilds WHERE id = $1", int64(guildID)).Scan(&cooldown); err != nil {
		t.Fatalf("query kick_leader_cd: %v", err)
	}
	if cooldown <= time.Now().Unix() {
		t.Fatalf("expected kick_leader_cd in future")
	}
}

func TestGuildCreateBlockedByWaitCooldown(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	commanderID := uint32(86301)
	cleanupGuildCoreData(t, commanderID)
	defer cleanupGuildCoreData(t, commanderID)

	client := &connection.Client{Commander: createGuildCommander(t, commanderID)}
	if err := orm.SetCommanderGuildWaitTime(commanderID, uint32(time.Now().Unix())+3600); err != nil {
		t.Fatalf("SetCommanderGuildWaitTime failed: %v", err)
	}

	payload, err := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(fmt.Sprintf("WAIT-%d", commanderID)),
		Manifesto: proto.String("cooldown check"),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := answer.CreateGuild(&payload, client); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	resp := &protobuf.SC_60002{}
	decodeTestPacket(t, client, 60002, resp)
	if resp.GetResult() != 1 {
		t.Fatalf("expected create blocked by cooldown, got %d", resp.GetResult())
	}
}

func TestGuildCreateValidationResultCodes(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	commanderID := uint32(86321)
	cleanupGuildCoreData(t, commanderID)
	defer cleanupGuildCoreData(t, commanderID)

	client := &connection.Client{Commander: createGuildCommander(t, commanderID)}

	testCases := []struct {
		name     string
		payload  *protobuf.CS_60001
		expected uint32
	}{
		{
			name: "invalid faction uses generic failure",
			payload: &protobuf.CS_60001{
				Faction:   proto.Uint32(3),
				Policy:    proto.Uint32(1),
				Name:      proto.String("VALIDNAME"),
				Manifesto: proto.String("manifesto"),
			},
			expected: 1,
		},
		{
			name: "invalid policy uses generic failure",
			payload: &protobuf.CS_60001{
				Faction:   proto.Uint32(1),
				Policy:    proto.Uint32(3),
				Name:      proto.String("VALIDNAME"),
				Manifesto: proto.String("manifesto"),
			},
			expected: 1,
		},
		{
			name: "empty manifesto uses generic failure",
			payload: &protobuf.CS_60001{
				Faction:   proto.Uint32(1),
				Policy:    proto.Uint32(1),
				Name:      proto.String("VALIDNAME"),
				Manifesto: proto.String("  \t"),
			},
			expected: 1,
		},
		{
			name: "invalid name uses 2015",
			payload: &protobuf.CS_60001{
				Faction:   proto.Uint32(1),
				Policy:    proto.Uint32(1),
				Name:      proto.String(""),
				Manifesto: proto.String("manifesto"),
			},
			expected: 2015,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := proto.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			if _, _, err := answer.CreateGuild(&buf, client); err != nil {
				t.Fatalf("CreateGuild failed: %v", err)
			}
			resp := &protobuf.SC_60002{}
			decodeTestPacket(t, client, 60002, resp)
			if resp.GetResult() != tc.expected {
				t.Fatalf("expected result %d, got %d", tc.expected, resp.GetResult())
			}
		})
	}
}

func TestGuildImpeachRejectsFutureLastLogin(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86311)
	deputyID := uint32(86312)
	cleanupGuildCoreData(t, leaderID, deputyID)
	defer cleanupGuildCoreData(t, leaderID, deputyID)

	leader := createGuildCommander(t, leaderID)
	deputy := createGuildCommander(t, deputyID)
	leaderClient := &connection.Client{Commander: leader}
	deputyClient := &connection.Client{Commander: deputy}

	createBuf, _ := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(fmt.Sprintf("FUT-%d", leaderID)),
		Manifesto: proto.String("test"),
	})
	if _, _, err := answer.CreateGuild(&createBuf, leaderClient); err != nil {
		t.Fatalf("create guild: %v", err)
	}
	createResp := &protobuf.SC_60002{}
	decodeTestPacket(t, leaderClient, 60002, createResp)
	if createResp.GetResult() != 0 {
		t.Fatalf("expected create success, got %d", createResp.GetResult())
	}
	guildID := createResp.GetId()

	nowUnix := uint32(time.Now().Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 2, 0, $3, $3)", int64(guildID), int64(deputyID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(deputyID), int64(guildID))
	execAnswerExternalTestSQLT(t, "UPDATE commanders SET last_login = NOW() + INTERVAL '1 day' WHERE commander_id = $1", int64(leaderID))

	impeachBuf, _ := proto.Marshal(&protobuf.CS_60016{PlayerId: proto.Uint32(leaderID)})
	if _, _, err := answer.GuildImpeach(&impeachBuf, deputyClient); err != nil {
		t.Fatalf("GuildImpeach failed: %v", err)
	}
	impeachResp := &protobuf.SC_60017{}
	decodeTestPacket(t, deputyClient, 60017, impeachResp)
	if impeachResp.GetResult() != 1 {
		t.Fatalf("expected impeach reject for future login, got %d", impeachResp.GetResult())
	}
}
