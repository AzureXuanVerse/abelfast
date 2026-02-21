package guild

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestGuildFetchBossCommandResponse(t *testing.T) {
	client := setupConfigTest(t)

	client.Buffer.Reset()
	successPayload, _ := proto.Marshal(&protobuf.CS_61015{Type: proto.Uint32(0)})
	if _, _, err := GuildFetchBossCommandResponse(&successPayload, client); err != nil {
		t.Fatalf("61015 success request failed: %v", err)
	}
	var successResp protobuf.SC_61016
	decodeResponse(t, client, &successResp)
	if successResp.GetResult() != 0 {
		t.Fatalf("expected success result 0, got %d", successResp.GetResult())
	}

	client.Buffer.Reset()
	failurePayload, _ := proto.Marshal(&protobuf.CS_61015{Type: proto.Uint32(1)})
	if _, _, err := GuildFetchBossCommandResponse(&failurePayload, client); err != nil {
		t.Fatalf("61015 invalid type request failed: %v", err)
	}
	var failureResp protobuf.SC_61016
	decodeResponse(t, client, &failureResp)
	if failureResp.GetResult() == 0 {
		t.Fatalf("expected non-zero result for unsupported type")
	}
}

func TestGuildFetchBossCommandResponseDecodeFailure(t *testing.T) {
	client := setupConfigTest(t)
	invalid := []byte{0xFF, 0x01}
	_, packetID, err := GuildFetchBossCommandResponse(&invalid, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if packetID != 61016 {
		t.Fatalf("expected response packet id 61016, got %d", packetID)
	}
}

func TestGuildGetBossInfoCommandResponse(t *testing.T) {
	client := setupConfigTest(t)
	const guildID uint32 = 9201
	const operationID uint32 = 7301
	seedGuildAssaultTestContext(t, client.Commander.CommanderID, guildID, operationID)
	if err := orm.UpsertGuildOperationBossState(orm.GuildOperationBossState{
		GuildID:     guildID,
		OperationID: operationID,
		BossID:      88,
		Damage:      1200,
		HP:          45000,
	}); err != nil {
		t.Fatalf("seed boss state failed: %v", err)
	}

	client.Buffer.Reset()
	successPayload, _ := proto.Marshal(&protobuf.CS_61027{Type: proto.Uint32(0)})
	if _, _, err := GuildGetBossInfoCommandResponse(&successPayload, client); err != nil {
		t.Fatalf("61027 success request failed: %v", err)
	}
	var successResp protobuf.SC_61028
	decodeResponse(t, client, &successResp)
	if successResp.GetResult() != 0 {
		t.Fatalf("expected success result 0, got %d", successResp.GetResult())
	}
	if successResp.GetBossEvent() == nil {
		t.Fatalf("expected non-nil boss event")
	}
	if successResp.GetBossEvent().GetBossId() != 88 || successResp.GetBossEvent().GetDamage() != 1200 || successResp.GetBossEvent().GetHp() != 45000 {
		t.Fatalf("unexpected boss event payload: %+v", successResp.GetBossEvent())
	}

	client.Buffer.Reset()
	invalidTypePayload, _ := proto.Marshal(&protobuf.CS_61027{Type: proto.Uint32(2)})
	if _, _, err := GuildGetBossInfoCommandResponse(&invalidTypePayload, client); err != nil {
		t.Fatalf("61027 invalid type request failed: %v", err)
	}
	var invalidTypeResp protobuf.SC_61028
	decodeResponse(t, client, &invalidTypeResp)
	if invalidTypeResp.GetResult() == 0 {
		t.Fatalf("expected non-zero result for unsupported type")
	}
	if invalidTypeResp.GetBossEvent() == nil {
		t.Fatalf("expected non-nil boss event on invalid type response")
	}
}

func TestGuildGetBossInfoCommandResponseNoBossStateReturnsDefaultBossEvent(t *testing.T) {
	client := setupConfigTest(t)
	seedGuildAssaultTestContext(t, client.Commander.CommanderID, 9202, 7302)

	client.Buffer.Reset()
	payload, _ := proto.Marshal(&protobuf.CS_61027{Type: proto.Uint32(0)})
	if _, _, err := GuildGetBossInfoCommandResponse(&payload, client); err != nil {
		t.Fatalf("61027 request failed: %v", err)
	}
	var resp protobuf.SC_61028
	decodeResponse(t, client, &resp)
	if resp.GetResult() != guildEventResultSuccess {
		t.Fatalf("expected result %d, got %d", guildEventResultSuccess, resp.GetResult())
	}
	if resp.GetBossEvent() == nil {
		t.Fatalf("expected non-nil boss event")
	}
	if resp.GetBossEvent().GetBossId() != 0 || resp.GetBossEvent().GetDamage() != 0 || resp.GetBossEvent().GetHp() != 0 {
		t.Fatalf("expected default boss event payload, got %+v", resp.GetBossEvent())
	}
}

func TestGuildGetBossInfoCommandResponseDecodeFailure(t *testing.T) {
	client := setupConfigTest(t)
	invalid := []byte{0xFF, 0x01}
	_, packetID, err := GuildGetBossInfoCommandResponse(&invalid, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if packetID != 61028 {
		t.Fatalf("expected response packet id 61028, got %d", packetID)
	}
}

func TestGuildGetBossRankCommandResponse(t *testing.T) {
	client := setupConfigTest(t)
	const guildID uint32 = 9203
	const operationID uint32 = 7303
	const bossID uint32 = 89
	seedGuildAssaultTestContext(t, client.Commander.CommanderID, guildID, operationID)
	if err := orm.UpsertGuildOperationBossState(orm.GuildOperationBossState{
		GuildID:     guildID,
		OperationID: operationID,
		BossID:      bossID,
		Damage:      200,
		HP:          5000,
	}); err != nil {
		t.Fatalf("seed boss state failed: %v", err)
	}
	if err := orm.ReplaceGuildOperationBossRanks(guildID, operationID, bossID, []orm.GuildOperationBossRank{{UserID: 8, Damage: 1200}, {UserID: 7, Damage: 1200}, {UserID: 9, Damage: 500}}); err != nil {
		t.Fatalf("seed boss ranks failed: %v", err)
	}

	client.Buffer.Reset()
	successPayload, _ := proto.Marshal(&protobuf.CS_61029{Type: proto.Uint32(0)})
	if _, _, err := GuildGetBossRankCommandResponse(&successPayload, client); err != nil {
		t.Fatalf("61029 success request failed: %v", err)
	}
	var successResp protobuf.SC_61030
	decodeResponse(t, client, &successResp)
	if len(successResp.GetList()) != 3 {
		t.Fatalf("expected 3 rank entries, got %d", len(successResp.GetList()))
	}
	if successResp.GetList()[0].GetUserId() != 7 || successResp.GetList()[1].GetUserId() != 8 || successResp.GetList()[2].GetUserId() != 9 {
		t.Fatalf("expected deterministic rank ordering by damage and user id")
	}

	client.Buffer.Reset()
	invalidTypePayload, _ := proto.Marshal(&protobuf.CS_61029{Type: proto.Uint32(3)})
	if _, _, err := GuildGetBossRankCommandResponse(&invalidTypePayload, client); err != nil {
		t.Fatalf("61029 invalid type request failed: %v", err)
	}
	var invalidTypeResp protobuf.SC_61030
	decodeResponse(t, client, &invalidTypeResp)
	if len(invalidTypeResp.GetList()) != 0 {
		t.Fatalf("expected empty list for unsupported type")
	}
}

func TestGuildGetBossRankCommandResponseNoContextReturnsEmpty(t *testing.T) {
	client := setupConfigTest(t)
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_boss_ranks")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_boss_states")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_events")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_states")
	execAnswerTestSQLT(t, "DELETE FROM guild_members")
	execAnswerTestSQLT(t, "DELETE FROM guilds")

	client.Buffer.Reset()
	payload, _ := proto.Marshal(&protobuf.CS_61029{Type: proto.Uint32(0)})
	if _, _, err := GuildGetBossRankCommandResponse(&payload, client); err != nil {
		t.Fatalf("61029 request failed: %v", err)
	}
	var resp protobuf.SC_61030
	decodeResponse(t, client, &resp)
	if len(resp.GetList()) != 0 {
		t.Fatalf("expected empty list when no active context")
	}
}

func TestGuildGetBossRankCommandResponseDecodeFailure(t *testing.T) {
	client := setupConfigTest(t)
	invalid := []byte{0xFF, 0x01}
	_, packetID, err := GuildGetBossRankCommandResponse(&invalid, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if packetID != 61030 {
		t.Fatalf("expected response packet id 61030, got %d", packetID)
	}
}
