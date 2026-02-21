package answer_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedGuildOfficeChunkConfig(t *testing.T) {
	t.Helper()
	seedGuildCoreConfig(t)
	execAnswerExternalTestSQLT(t, "INSERT INTO resources (id, item_id, name) VALUES (1, 0, 'gold') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name")
	execAnswerExternalTestSQLT(t, "INSERT INTO resources (id, item_id, name) VALUES (8, 0, 'guild_coin') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name")

	mustUpsertConfigJSON(t, "ShareCfg/guildset.json", "guild_award_consume", map[string]any{"key_value": 3000})
	mustUpsertConfigJSON(t, "ShareCfg/guildset.json", "guild_award_duration", map[string]any{"key_value": 86400})
	mustUpsertConfigJSON(t, "ShareCfg/guildset.json", "guild_award_drop", map[string]any{"key_value": 8})
	mustUpsertConfigJSON(t, "ShareCfg/guildset.json", "guild_tech_default", map[string]any{"key_value": 1000})

	mustUpsertConfigJSON(t, "ShareCfg/guild_contribution_template.json", "1", map[string]any{
		"id":                 1,
		"consume":            []uint32{1, 1, 10},
		"award_contribution": 50,
		"award_capital":      200,
		"award_tech_exp":     5,
		"guild_active":       3,
	})
	mustUpsertConfigJSON(t, "ShareCfg/guild_mission_template.json", "7001", map[string]any{"id": 7001, "max_num": 10})
	mustUpsertConfigJSON(t, "ShareCfg/guild_technology_template.json", "1000", map[string]any{
		"id":                    1000,
		"group":                 1,
		"next_tech":             1001,
		"gold_consume":          1,
		"contribution_consume":  1,
		"contribution_multiple": 1,
	})
	mustUpsertConfigJSON(t, "ShareCfg/guild_technology_template.json", "1001", map[string]any{
		"id":        1001,
		"group":     1,
		"next_tech": 0,
	})
}

func mustUpsertConfigJSON(t *testing.T, category string, key string, payload map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal config %s/%s: %v", category, key, err)
	}
	if err := orm.UpsertConfigEntry(category, key, encoded); err != nil {
		t.Fatalf("upsert config %s/%s: %v", category, key, err)
	}
}

func TestGuildOfficeChunk5Handlers(t *testing.T) {
	orm.InitDatabase()
	seedGuildOfficeChunkConfig(t)

	leaderID := uint32(87101)
	memberID := uint32(87102)
	applicantID := uint32(87103)
	cleanupGuildCoreData(t, leaderID, memberID, applicantID)
	defer cleanupGuildCoreData(t, leaderID, memberID, applicantID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID), Hash: 81}
	memberClient := &connection.Client{Commander: createGuildCommander(t, memberID), Hash: 82}
	applicantClient := &connection.Client{Commander: createGuildCommander(t, applicantID), Hash: 83}
	server := connection.NewServer("127.0.0.1", 0, func(pkt *[]byte, c *connection.Client, size int) {})
	server.AddClient(leaderClient)
	server.AddClient(memberClient)
	server.AddClient(applicantClient)

	guildID := createGuildForTest(t, leaderClient, fmt.Sprintf("OFFICE-%d", leaderID))
	nowUnix := uint32(time.Now().Add(-26 * time.Hour).Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 4, 0, $3, $3)", int64(guildID), int64(memberID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id, donate_count, benefit_time, weekly_task_flag, extra_donate, extra_operation) VALUES ($1, $2, 0, 0, 0, 0, 0) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(memberID), int64(guildID))
	execAnswerExternalTestSQLT(t, "UPDATE guilds SET member_count = 2 WHERE id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO owned_resources (commander_id, resource_id, amount) VALUES ($1, 1, 1000) ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = EXCLUDED.amount", int64(leaderID))
	execAnswerExternalTestSQLT(t, "INSERT INTO owned_resources (commander_id, resource_id, amount) VALUES ($1, 8, 1000) ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = EXCLUDED.amount", int64(leaderID))
	if err := leaderClient.Commander.Load(); err != nil {
		t.Fatalf("reload leader commander: %v", err)
	}

	donatePayload, _ := proto.Marshal(&protobuf.CS_62002{Id: proto.Uint32(1)})
	if _, _, err := answer.GuildCommitDonate(&donatePayload, leaderClient); err != nil {
		t.Fatalf("GuildCommitDonate failed: %v", err)
	}
	donateResp := &protobuf.SC_62003{}
	decodeTestPacket(t, leaderClient, 62003, donateResp)
	if donateResp.GetResult() != 0 {
		t.Fatalf("expected donate success, got %d", donateResp.GetResult())
	}
	decodeTestPacket(t, memberClient, 62019, &protobuf.SC_62019{})

	capitalPayload, _ := proto.Marshal(&protobuf.CS_62024{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildFetchCapitalCommandResponse(&capitalPayload, leaderClient); err != nil {
		t.Fatalf("GuildFetchCapitalCommandResponse failed: %v", err)
	}
	capitalResp := &protobuf.SC_62025{}
	decodeTestPacket(t, leaderClient, 62025, capitalResp)
	if capitalResp.GetResult() != 0 || capitalResp.GetCapital() == 0 {
		t.Fatalf("expected capital success with value, result=%d capital=%d", capitalResp.GetResult(), capitalResp.GetCapital())
	}

	buyPayload, _ := proto.Marshal(&protobuf.CS_62007{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildBuySupply(&buyPayload, leaderClient); err != nil {
		t.Fatalf("GuildBuySupply failed: %v", err)
	}
	buyResp := &protobuf.SC_62008{}
	decodeTestPacket(t, leaderClient, 62008, buyResp)
	if buyResp.GetResult() != 0 {
		t.Fatalf("expected buy supply success, got %d", buyResp.GetResult())
	}
	buyPush := &protobuf.SC_62005{}
	decodeTestPacket(t, memberClient, 62005, buyPush)
	if buyPush.GetBenefitFinishTime() == 0 {
		t.Fatalf("expected supply push finish time")
	}
	execAnswerExternalTestSQLT(t, "UPDATE guilds SET benefit_finish_time = $2 WHERE id = $1", int64(guildID), int64(time.Now().Add(time.Hour).Unix()))
	execAnswerExternalTestSQLT(t, "UPDATE guild_members SET duty = 4, join_time = $3 WHERE guild_id = $1 AND commander_id = $2", int64(guildID), int64(memberID), int64(time.Now().Add(-48*time.Hour).Unix()))
	execAnswerExternalTestSQLT(t, "UPDATE guild_user_infos SET benefit_time = 0 WHERE commander_id = $1", int64(memberID))

	claimPayload, _ := proto.Marshal(&protobuf.CS_62009{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildGetSupplyAwardCommandResponse(&claimPayload, memberClient); err != nil {
		t.Fatalf("GuildGetSupplyAwardCommandResponse failed: %v", err)
	}
	claimResp := &protobuf.SC_62010{}
	decodeTestPacket(t, memberClient, 62010, claimResp)
	if claimResp.GetResult() != 0 || len(claimResp.GetDropList()) == 0 {
		t.Fatalf("expected claim success with drops, result=%d drops=%d", claimResp.GetResult(), len(claimResp.GetDropList()))
	}

	logsPayload, _ := proto.Marshal(&protobuf.CS_62011{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildFetchCapitalLogCommandResponse(&logsPayload, leaderClient); err != nil {
		t.Fatalf("GuildFetchCapitalLogCommandResponse failed: %v", err)
	}
	logsResp := &protobuf.SC_62012{}
	decodeTestPacket(t, leaderClient, 62012, logsResp)
	if logsResp.GetResult() != 0 {
		t.Fatalf("expected logs success, got %d", logsResp.GetResult())
	}
	if len(logsResp.GetInclog())+len(logsResp.GetDeclog())+len(logsResp.GetOtherlog()) == 0 {
		t.Fatalf("expected at least one capital log entry")
	}

	selectPayload, _ := proto.Marshal(&protobuf.CS_62013{Id: proto.Uint32(7001)})
	if _, _, err := answer.GuildSelectWeeklyTask(&selectPayload, leaderClient); err != nil {
		t.Fatalf("GuildSelectWeeklyTask failed: %v", err)
	}
	selectResp := &protobuf.SC_62014{}
	decodeTestPacket(t, leaderClient, 62014, selectResp)
	if selectResp.GetResult() != 0 {
		t.Fatalf("expected weekly task select success, got %d", selectResp.GetResult())
	}
	selectPush := &protobuf.SC_62004{}
	decodeTestPacket(t, memberClient, 62004, selectPush)
	if selectPush.GetThisWeeklyTasks().GetId() != 7001 {
		t.Fatalf("expected weekly task push id 7001")
	}

	progressPayload, _ := proto.Marshal(&protobuf.CS_62022{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildFetchWeeklyTaskProgressCommandResponse(&progressPayload, leaderClient); err != nil {
		t.Fatalf("GuildFetchWeeklyTaskProgressCommandResponse failed: %v", err)
	}
	progressResp := &protobuf.SC_62023{}
	decodeTestPacket(t, leaderClient, 62023, progressResp)
	if progressResp.GetResult() != 0 {
		t.Fatalf("expected progress success, got %d", progressResp.GetResult())
	}

	techStartPayload, _ := proto.Marshal(&protobuf.CS_62020{Id: proto.Uint32(1000)})
	if _, _, err := answer.GuildStartTechGroupCommandResponse(&techStartPayload, leaderClient); err != nil {
		t.Fatalf("GuildStartTechGroupCommandResponse failed: %v", err)
	}
	techStartResp := &protobuf.SC_62021{}
	decodeTestPacket(t, leaderClient, 62021, techStartResp)
	if techStartResp.GetResult() != 0 {
		t.Fatalf("expected tech start success, got %d", techStartResp.GetResult())
	}
	techStartPush := &protobuf.SC_62018{}
	decodeTestPacket(t, memberClient, 62018, techStartPush)

	techUpgradePayload, _ := proto.Marshal(&protobuf.CS_62015{Id: proto.Uint32(1000)})
	if _, _, err := answer.GuildUpgradeTechnologyCommandResponse(&techUpgradePayload, leaderClient); err != nil {
		t.Fatalf("GuildUpgradeTechnologyCommandResponse failed: %v", err)
	}
	techUpgradeResp := &protobuf.SC_62016{}
	decodeTestPacket(t, leaderClient, 62016, techUpgradeResp)
	if techUpgradeResp.GetResult() != 0 {
		t.Fatalf("expected tech upgrade success, got %d", techUpgradeResp.GetResult())
	}

	rankPayload, _ := proto.Marshal(&protobuf.CS_62029{Type: proto.Uint32(1)})
	if _, _, err := answer.GuildGetRankCommandResponse(&rankPayload, leaderClient); err != nil {
		t.Fatalf("GuildGetRankCommandResponse failed: %v", err)
	}
	rankResp := &protobuf.SC_62030{}
	decodeTestPacket(t, leaderClient, 62030, rankResp)
	if len(rankResp.GetList()) != 3 {
		t.Fatalf("expected 3 rank periods, got %d", len(rankResp.GetList()))
	}

	if _, _, err := answer.GuildGetUserInfoCommand(&[]byte{}, leaderClient); err != nil {
		t.Fatalf("GuildGetUserInfoCommand failed: %v", err)
	}
	userInfoResp := &protobuf.SC_60103{}
	decodeTestPacket(t, leaderClient, 60103, userInfoResp)
	if len(userInfoResp.GetUserInfo().GetTechId()) == 0 {
		t.Fatalf("expected user tech ids to be populated")
	}

	applyPayload, _ := proto.Marshal(&protobuf.CS_60005{Id: proto.Uint32(guildID), Content: proto.String("join")})
	if _, _, err := answer.GuildApply(&applyPayload, applicantClient); err != nil {
		t.Fatalf("GuildApply failed: %v", err)
	}
	decodeTestPacket(t, applicantClient, 60006, &protobuf.SC_60006{})
	acceptPayload, _ := proto.Marshal(&protobuf.CS_60020{PlayerId: proto.Uint32(applicantID)})
	if _, _, err := answer.AcceptGuildJoinRequest(&acceptPayload, leaderClient); err != nil {
		t.Fatalf("AcceptGuildJoinRequest failed: %v", err)
	}
	acceptResp := &protobuf.SC_60021{}
	decodeTestPacket(t, leaderClient, 60021, acceptResp)
	if acceptResp.GetResult() != 0 {
		t.Fatalf("expected accept success, got %d", acceptResp.GetResult())
	}

	refreshPayload, _ := proto.Marshal(&protobuf.CS_60024{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildListRefresh(&refreshPayload, leaderClient); err != nil {
		t.Fatalf("GuildListRefresh failed: %v", err)
	}
	refreshResp := &protobuf.SC_60025{}
	decodeTestPacket(t, leaderClient, 60025, refreshResp)
	if len(refreshResp.GetGuildList()) == 0 {
		t.Fatalf("expected guild list entries")
	}
}

func TestGuildCommitDonateRejectsNonMembers(t *testing.T) {
	orm.InitDatabase()
	seedGuildOfficeChunkConfig(t)

	commanderID := uint32(87201)
	cleanupGuildCoreData(t, commanderID)
	defer cleanupGuildCoreData(t, commanderID)

	client := &connection.Client{Commander: createGuildCommander(t, commanderID), Hash: 84}
	execAnswerExternalTestSQLT(t, "INSERT INTO owned_resources (commander_id, resource_id, amount) VALUES ($1, 1, 100) ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = EXCLUDED.amount", int64(commanderID))
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_62002{Id: proto.Uint32(1)})
	if _, _, err := answer.GuildCommitDonate(&payload, client); err != nil {
		t.Fatalf("GuildCommitDonate failed: %v", err)
	}
	resp := &protobuf.SC_62003{}
	decodeTestPacket(t, client, 62003, resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected donate failure for commander outside a guild")
	}
	guildCoin := queryAnswerExternalTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = 8), 0)", int64(commanderID))
	if guildCoin != 0 {
		t.Fatalf("expected no guild coin gain, got %d", guildCoin)
	}
}

func TestGuildStartTechGroupRequiresOfficerDuty(t *testing.T) {
	orm.InitDatabase()
	seedGuildOfficeChunkConfig(t)

	leaderID := uint32(87211)
	memberID := uint32(87212)
	cleanupGuildCoreData(t, leaderID, memberID)
	defer cleanupGuildCoreData(t, leaderID, memberID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID), Hash: 85}
	memberClient := &connection.Client{Commander: createGuildCommander(t, memberID), Hash: 86}
	guildID := createGuildForTest(t, leaderClient, fmt.Sprintf("OFFICER-%d", leaderID))
	nowUnix := uint32(time.Now().Add(-26 * time.Hour).Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 4, 0, $3, $3)", int64(guildID), int64(memberID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id, donate_count, benefit_time, weekly_task_flag, extra_donate, extra_operation) VALUES ($1, $2, 0, 0, 0, 0, 0) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(memberID), int64(guildID))

	payload, _ := proto.Marshal(&protobuf.CS_62020{Id: proto.Uint32(1000)})
	if _, _, err := answer.GuildStartTechGroupCommandResponse(&payload, memberClient); err != nil {
		t.Fatalf("GuildStartTechGroupCommandResponse failed: %v", err)
	}
	resp := &protobuf.SC_62021{}
	decodeTestPacket(t, memberClient, 62021, resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected non-officer to be denied starting guild tech")
	}
}
