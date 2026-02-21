package guild_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func createGuildForTest(t *testing.T, leaderClient *connection.Client, name string) uint32 {
	t.Helper()
	payload, err := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(name),
		Manifesto: proto.String("guild manifesto"),
	})
	if err != nil {
		t.Fatalf("marshal create guild payload: %v", err)
	}
	if _, _, err := answer.CreateGuild(&payload, leaderClient); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	resp := &protobuf.SC_60002{}
	decodeTestPacket(t, leaderClient, 60002, resp)
	if resp.GetResult() != 0 || resp.GetId() == 0 {
		t.Fatalf("expected create guild success, got result=%d id=%d", resp.GetResult(), resp.GetId())
	}
	return resp.GetId()
}

func TestGuildApplyRequestListAndRejectFlow(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86401)
	applicantID := uint32(86402)
	memberID := uint32(86403)
	cleanupGuildCoreData(t, leaderID, applicantID, memberID)
	defer cleanupGuildCoreData(t, leaderID, applicantID, memberID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID)}
	applicantClient := &connection.Client{Commander: createGuildCommander(t, applicantID)}
	memberClient := &connection.Client{Commander: createGuildCommander(t, memberID)}
	guildID := createGuildForTest(t, leaderClient, fmt.Sprintf("REQ-%d", leaderID))

	nowUnix := uint32(time.Now().Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 4, 0, $3, $3)", int64(guildID), int64(memberID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(memberID), int64(guildID))

	applyPayload, _ := proto.Marshal(&protobuf.CS_60005{Id: proto.Uint32(guildID), Content: proto.String("  hello guild  ")})
	if _, _, err := answer.GuildApply(&applyPayload, applicantClient); err != nil {
		t.Fatalf("GuildApply failed: %v", err)
	}
	applyResponse := &protobuf.SC_60006{}
	decodeTestPacket(t, applicantClient, 60006, applyResponse)
	if applyResponse.GetResult() != 0 {
		t.Fatalf("expected apply success, got %d", applyResponse.GetResult())
	}

	listPayload, _ := proto.Marshal(&protobuf.CS_60003{Id: proto.Uint32(guildID)})
	if _, _, err := answer.GetGuildRequestsCommandResponse(&listPayload, leaderClient); err != nil {
		t.Fatalf("GetGuildRequestsCommandResponse failed: %v", err)
	}
	listResponse := &protobuf.SC_60004{}
	decodeTestPacket(t, leaderClient, 60004, listResponse)
	if len(listResponse.GetRequestList()) != 1 {
		t.Fatalf("expected one guild request, got %d", len(listResponse.GetRequestList()))
	}
	request := listResponse.GetRequestList()[0]
	if request.GetPlayer().GetId() != applicantID {
		t.Fatalf("expected applicant id %d, got %d", applicantID, request.GetPlayer().GetId())
	}
	if request.GetContent() != "hello guild" {
		t.Fatalf("expected trimmed content, got %q", request.GetContent())
	}

	rejectByMemberPayload, _ := proto.Marshal(&protobuf.CS_60022{PlayerId: proto.Uint32(applicantID)})
	if _, _, err := answer.RejectGuildJoinRequest(&rejectByMemberPayload, memberClient); err != nil {
		t.Fatalf("RejectGuildJoinRequest by member failed: %v", err)
	}
	rejectByMemberResponse := &protobuf.SC_60023{}
	decodeTestPacket(t, memberClient, 60023, rejectByMemberResponse)
	if rejectByMemberResponse.GetResult() == 0 {
		t.Fatalf("expected reject failure for ordinary member")
	}

	if _, _, err := answer.GetGuildRequestsCommandResponse(&listPayload, leaderClient); err != nil {
		t.Fatalf("GetGuildRequestsCommandResponse after member reject failed: %v", err)
	}
	listAfterMemberReject := &protobuf.SC_60004{}
	decodeTestPacket(t, leaderClient, 60004, listAfterMemberReject)
	if len(listAfterMemberReject.GetRequestList()) != 1 {
		t.Fatalf("expected guild request to remain after member reject attempt, got %d", len(listAfterMemberReject.GetRequestList()))
	}

	rejectPayload, _ := proto.Marshal(&protobuf.CS_60022{PlayerId: proto.Uint32(applicantID)})
	if _, _, err := answer.RejectGuildJoinRequest(&rejectPayload, leaderClient); err != nil {
		t.Fatalf("RejectGuildJoinRequest failed: %v", err)
	}
	rejectResponse := &protobuf.SC_60023{}
	decodeTestPacket(t, leaderClient, 60023, rejectResponse)
	if rejectResponse.GetResult() != 0 {
		t.Fatalf("expected reject success, got %d", rejectResponse.GetResult())
	}

	if _, _, err := answer.GetGuildRequestsCommandResponse(&listPayload, leaderClient); err != nil {
		t.Fatalf("GetGuildRequestsCommandResponse after reject failed: %v", err)
	}
	listAfterReject := &protobuf.SC_60004{}
	decodeTestPacket(t, leaderClient, 60004, listAfterReject)
	if len(listAfterReject.GetRequestList()) != 0 {
		t.Fatalf("expected no guild requests after reject, got %d", len(listAfterReject.GetRequestList()))
	}
}

func TestGuildApplyCooldownAndOutstandingLimit(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86411)
	applicantID := uint32(86412)
	cleanupGuildCoreData(t, leaderID, applicantID)
	defer cleanupGuildCoreData(t, leaderID, applicantID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID)}
	applicantClient := &connection.Client{Commander: createGuildCommander(t, applicantID)}
	guildID := createGuildForTest(t, leaderClient, fmt.Sprintf("LIM-%d", leaderID))

	if err := orm.SetCommanderGuildWaitTime(applicantID, uint32(time.Now().Unix())+60); err != nil {
		t.Fatalf("SetCommanderGuildWaitTime failed: %v", err)
	}
	applyPayload, _ := proto.Marshal(&protobuf.CS_60005{Id: proto.Uint32(guildID), Content: proto.String("cooldown")})
	if _, _, err := answer.GuildApply(&applyPayload, applicantClient); err != nil {
		t.Fatalf("GuildApply cooldown failed: %v", err)
	}
	cooldownResponse := &protobuf.SC_60006{}
	decodeTestPacket(t, applicantClient, 60006, cooldownResponse)
	if cooldownResponse.GetResult() != 4 {
		t.Fatalf("expected cooldown result 4, got %d", cooldownResponse.GetResult())
	}

	if err := orm.SetCommanderGuildWaitTime(applicantID, 0); err != nil {
		t.Fatalf("clear cooldown failed: %v", err)
	}
	now := time.Now().Add(-time.Minute)
	for i := 0; i < 10; i++ {
		seedGuildID := uint32(95000 + i)
		execAnswerExternalTestSQLT(t, "INSERT INTO guilds (id, policy, faction, name, level, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id) VALUES ($1, 1, 1, $2, 1, '', 0, 1, 0, 0, 20000, 1000) ON CONFLICT (id) DO NOTHING", int64(seedGuildID), fmt.Sprintf("SEED-%d", i))
		if err := orm.UpsertGuildJoinRequest(seedGuildID, applicantID, "seed", now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("seed join request %d: %v", i, err)
		}
	}

	if _, _, err := answer.GuildApply(&applyPayload, applicantClient); err != nil {
		t.Fatalf("GuildApply outstanding limit failed: %v", err)
	}
	limitResponse := &protobuf.SC_60006{}
	decodeTestPacket(t, applicantClient, 60006, limitResponse)
	if limitResponse.GetResult() != 6 {
		t.Fatalf("expected outstanding limit result 6, got %d", limitResponse.GetResult())
	}
}

func TestGuildSearchByIDAndName(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86421)
	searcherID := uint32(86422)
	cleanupGuildCoreData(t, leaderID, searcherID)
	defer cleanupGuildCoreData(t, leaderID, searcherID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID)}
	searchClient := &connection.Client{Commander: createGuildCommander(t, searcherID)}
	guildName := fmt.Sprintf("FIND-%d", leaderID)
	guildID := createGuildForTest(t, leaderClient, guildName)

	byIDPayload, _ := proto.Marshal(&protobuf.CS_60028{Type: proto.Uint32(0), Keyword: proto.String(fmt.Sprintf("%d", guildID))})
	if _, _, err := answer.GuildSearch(&byIDPayload, searchClient); err != nil {
		t.Fatalf("GuildSearch by id failed: %v", err)
	}
	byIDResponse := &protobuf.SC_60029{}
	decodeTestPacket(t, searchClient, 60029, byIDResponse)
	if byIDResponse.GetResult() != 0 || len(byIDResponse.GetGuild()) != 1 {
		t.Fatalf("expected one guild for id search, got result=%d len=%d", byIDResponse.GetResult(), len(byIDResponse.GetGuild()))
	}
	if byIDResponse.GetGuild()[0].GetBase().GetId() != guildID {
		t.Fatalf("expected guild id %d, got %d", guildID, byIDResponse.GetGuild()[0].GetBase().GetId())
	}

	byNamePayload, _ := proto.Marshal(&protobuf.CS_60028{Type: proto.Uint32(1), Keyword: proto.String(guildName)})
	if _, _, err := answer.GuildSearch(&byNamePayload, searchClient); err != nil {
		t.Fatalf("GuildSearch by name failed: %v", err)
	}
	byNameResponse := &protobuf.SC_60029{}
	decodeTestPacket(t, searchClient, 60029, byNameResponse)
	if byNameResponse.GetResult() != 0 || len(byNameResponse.GetGuild()) != 1 {
		t.Fatalf("expected one guild for name search, got result=%d len=%d", byNameResponse.GetResult(), len(byNameResponse.GetGuild()))
	}
	if byNameResponse.GetGuild()[0].GetLeader().GetId() != leaderID {
		t.Fatalf("expected leader id %d, got %d", leaderID, byNameResponse.GetGuild()[0].GetLeader().GetId())
	}
}

func TestGuildSearchValidation(t *testing.T) {
	orm.InitDatabase()
	commanderID := uint32(86431)
	cleanupGuildCoreData(t, commanderID)
	defer cleanupGuildCoreData(t, commanderID)

	client := &connection.Client{Commander: createGuildCommander(t, commanderID)}

	testCases := []protobuf.CS_60028{
		{Type: proto.Uint32(0), Keyword: proto.String("not-a-number")},
		{Type: proto.Uint32(1), Keyword: proto.String("  ")},
		{Type: proto.Uint32(99), Keyword: proto.String("anything")},
	}

	for _, payload := range testCases {
		buf, err := proto.Marshal(&payload)
		if err != nil {
			t.Fatalf("marshal search payload: %v", err)
		}
		if _, _, err := answer.GuildSearch(&buf, client); err != nil {
			t.Fatalf("GuildSearch validation failed: %v", err)
		}
		response := &protobuf.SC_60029{}
		decodeTestPacket(t, client, 60029, response)
		if response.GetResult() == 0 {
			t.Fatalf("expected failure result for payload %+v", payload)
		}
		if len(response.GetGuild()) != 0 {
			t.Fatalf("expected empty guild list for payload %+v", payload)
		}
	}
}
