package answer_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedGuildEventChunk4Config(t *testing.T, chapterID uint32) {
	t.Helper()
	if err := orm.UpsertConfigEntry("ShareCfg/guildset.json", "operation_duration_time", json.RawMessage(`{"key_value":3600}`)); err != nil {
		t.Fatalf("seed operation_duration_time: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/guildset.json", "efficiency_param_times", json.RawMessage(`{"key_value":2}`)); err != nil {
		t.Fatalf("seed efficiency_param_times: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/guildset.json", "operation_event_guild_active", json.RawMessage(`{"key_value":1}`)); err != nil {
		t.Fatalf("seed operation_event_guild_active: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/guild_operation_template.json", fmt.Sprintf("%d", chapterID), json.RawMessage(`{"consume":100,"unlock_guild_level":1}`)); err != nil {
		t.Fatalf("seed guild_operation_template: %v", err)
	}
}

func createGuildEventClient(t *testing.T, commanderID uint32) (*connection.Client, uint32) {
	t.Helper()
	client := &connection.Client{Commander: createGuildCommander(t, commanderID)}
	createPayload := &protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(fmt.Sprintf("GE-%d", commanderID)),
		Manifesto: proto.String("guild event"),
	}
	buf, err := proto.Marshal(createPayload)
	if err != nil {
		t.Fatalf("marshal create guild: %v", err)
	}
	if _, _, err := answer.CreateGuild(&buf, client); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	resp := &protobuf.SC_60002{}
	decodeTestPacket(t, client, 60002, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected create guild success, got %d", resp.GetResult())
	}
	return client, resp.GetId()
}

func cleanupGuildEventChunk4Data(t *testing.T, guildID uint32, commanderID uint32) {
	t.Helper()
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_report_ranks WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_report_nodes WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_reports WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_operation_perfs WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_operation_participants WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_operation_events WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_operation_states WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guild_members WHERE guild_id = $1", int64(guildID))
	execAnswerExternalTestSQLT(t, "DELETE FROM guilds WHERE id = $1", int64(guildID))
	cleanupGuildCoreData(t, commanderID)
}

func activateGuildEventChapter(t *testing.T, client *connection.Client, chapterID uint32) {
	t.Helper()
	buf, err := proto.Marshal(&protobuf.CS_61001{ChapterId: proto.Uint32(chapterID)})
	if err != nil {
		t.Fatalf("marshal 61001: %v", err)
	}
	if _, _, err := answer.GuildActiveEventCommandResponse(&buf, client); err != nil {
		t.Fatalf("GuildActiveEventCommandResponse failed: %v", err)
	}
	resp := &protobuf.SC_61002{}
	decodeTestPacket(t, client, 61002, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected activation success, got %d", resp.GetResult())
	}
}

func TestGuildEventChunk4ActivateAndSnapshot(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterID := uint32(7001)
	seedGuildEventChunk4Config(t, chapterID)
	client, guildID := createGuildEventClient(t, 87101)
	defer cleanupGuildEventChunk4Data(t, guildID, 87101)

	activateGuildEventChapter(t, client, chapterID)

	guild, err := orm.GetGuildByID(guildID)
	if err != nil {
		t.Fatalf("GetGuildByID failed: %v", err)
	}
	if guild.Capital != 19900 {
		t.Fatalf("expected capital 19900 after consume, got %d", guild.Capital)
	}

	buf, _ := proto.Marshal(&protobuf.CS_61005{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildGetActivationEventCommandResponse(&buf, client); err != nil {
		t.Fatalf("GuildGetActivationEventCommandResponse failed: %v", err)
	}
	resp := &protobuf.SC_61006{}
	decodeTestPacket(t, client, 61006, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected snapshot success, got %d", resp.GetResult())
	}
	if resp.GetOperation() == nil || resp.GetOperation().GetOperationId() != chapterID {
		t.Fatalf("expected operation id %d", chapterID)
	}
	if len(resp.GetOperation().GetBaseEvents()) != 1 {
		t.Fatalf("expected one base event, got %d", len(resp.GetOperation().GetBaseEvents()))
	}
}

func TestGuildEventChunk4MissionRefreshAndPerf(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterID := uint32(7002)
	seedGuildEventChunk4Config(t, chapterID)
	client, guildID := createGuildEventClient(t, 87111)
	defer cleanupGuildEventChunk4Data(t, guildID, 87111)
	activateGuildEventChapter(t, client, chapterID)

	execAnswerExternalTestSQLT(t, "INSERT INTO owned_ships (id, owner_id, ship_id, level, max_level, create_time, change_name_timestamp) VALUES ($1, $2, $3, 1, 125, NOW(), NOW())", int64(900001), int64(client.Commander.CommanderID), int64(202124))
	execAnswerExternalTestSQLT(t, "INSERT INTO owned_ships (id, owner_id, ship_id, level, max_level, create_time, change_name_timestamp) VALUES ($1, $2, $3, 1, 125, NOW(), NOW())", int64(900002), int64(client.Commander.CommanderID), int64(202124))

	joinBuf, _ := proto.Marshal(&protobuf.CS_61007{EventTid: proto.Uint32(chapterID), ShipIds: []uint32{900001, 900002}})
	if _, _, err := answer.GuildJoinMissionCommandResponse(&joinBuf, client); err != nil {
		t.Fatalf("GuildJoinMissionCommandResponse failed: %v", err)
	}
	joinResp := &protobuf.SC_61008{}
	decodeTestPacket(t, client, 61008, joinResp)
	if joinResp.GetResult() != 0 {
		t.Fatalf("expected mission join success, got %d", joinResp.GetResult())
	}

	refreshBuf, _ := proto.Marshal(&protobuf.CS_61023{EventTid: proto.Uint32(chapterID)})
	if _, _, err := answer.GuildRefreshMissionCommandResponse(&refreshBuf, client); err != nil {
		t.Fatalf("GuildRefreshMissionCommandResponse failed: %v", err)
	}
	refreshResp := &protobuf.SC_61024{}
	decodeTestPacket(t, client, 61024, refreshResp)
	if refreshResp.GetResult() != 0 {
		t.Fatalf("expected refresh success, got %d", refreshResp.GetResult())
	}
	if refreshResp.GetEventInfo() == nil || len(refreshResp.GetEventInfo().GetPersonship()) != 1 {
		t.Fatalf("expected event info personship to be populated")
	}

	perfBuf, _ := proto.Marshal(&protobuf.CS_61025{Perf: []*protobuf.EVENT_PERFORMANCE{{EventId: proto.Uint32(chapterID), Index: proto.Uint32(3)}}})
	if _, _, err := answer.GuildUpdateNodeAnimFlagCommandResponse(&perfBuf, client); err != nil {
		t.Fatalf("GuildUpdateNodeAnimFlagCommandResponse failed: %v", err)
	}
	perfResp := &protobuf.SC_61026{}
	decodeTestPacket(t, client, 61026, perfResp)
	if perfResp.GetResult() != 0 {
		t.Fatalf("expected perf update success, got %d", perfResp.GetResult())
	}

	snapshotBuf, _ := proto.Marshal(&protobuf.CS_61005{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildGetActivationEventCommandResponse(&snapshotBuf, client); err != nil {
		t.Fatalf("snapshot after perf failed: %v", err)
	}
	snapshotResp := &protobuf.SC_61006{}
	decodeTestPacket(t, client, 61006, snapshotResp)
	if len(snapshotResp.GetOperation().GetPerfs()) != 1 {
		t.Fatalf("expected one perf entry, got %d", len(snapshotResp.GetOperation().GetPerfs()))
	}
}

func TestGuildEventChunk4ReportsAndRank(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterID := uint32(7003)
	seedGuildEventChunk4Config(t, chapterID)
	client, guildID := createGuildEventClient(t, 87121)
	defer cleanupGuildEventChunk4Data(t, guildID, 87121)

	execAnswerExternalTestSQLT(t, "INSERT INTO owned_resources (commander_id, resource_id, amount) VALUES ($1, $2, $3) ON CONFLICT (commander_id, resource_id) DO UPDATE SET amount = EXCLUDED.amount", int64(client.Commander.CommanderID), int64(1), int64(0))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_reports (guild_id, id, event_id, event_type, score, status, claimed, drop_type, drop_id, drop_count) VALUES ($1, 1001, 5001, 2, 300, 1, false, 1, 1, 50)", int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_reports (guild_id, id, event_id, event_type, score, status, claimed, drop_type, drop_id, drop_count) VALUES ($1, 1002, 5002, 2, 400, 1, false, 1, 1, 25)", int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_report_nodes (guild_id, report_id, node_id, status) VALUES ($1, 1001, 1, 2)", int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_report_ranks (guild_id, report_id, user_id, damage) VALUES ($1, 1001, 10, 900)", int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_report_ranks (guild_id, report_id, user_id, damage) VALUES ($1, 1001, 11, 1200)", int64(guildID))

	getReportsBuf, _ := proto.Marshal(&protobuf.CS_61017{Index: proto.Uint32(1000)})
	if _, _, err := answer.GuildGetReportsCommandResponse(&getReportsBuf, client); err != nil {
		t.Fatalf("GuildGetReportsCommandResponse failed: %v", err)
	}
	reportsResp := &protobuf.SC_61018{}
	decodeTestPacket(t, client, 61018, reportsResp)
	if len(reportsResp.GetReports()) != 2 {
		t.Fatalf("expected two reports, got %d", len(reportsResp.GetReports()))
	}

	submitBuf, _ := proto.Marshal(&protobuf.CS_61019{Ids: []uint32{1001, 1001}})
	if _, _, err := answer.SubmitGuildReportCommandResponse(&submitBuf, client); err != nil {
		t.Fatalf("SubmitGuildReportCommandResponse failed: %v", err)
	}
	submitResp := &protobuf.SC_61020{}
	decodeTestPacket(t, client, 61020, submitResp)
	if submitResp.GetResult() != 0 {
		t.Fatalf("expected submit success, got %d", submitResp.GetResult())
	}
	if len(submitResp.GetDropList()) == 0 {
		t.Fatalf("expected non-empty drop list")
	}

	retryBuf, _ := proto.Marshal(&protobuf.CS_61019{Ids: []uint32{1001}})
	if _, _, err := answer.SubmitGuildReportCommandResponse(&retryBuf, client); err != nil {
		t.Fatalf("SubmitGuildReportCommandResponse retry failed: %v", err)
	}
	retryResp := &protobuf.SC_61020{}
	decodeTestPacket(t, client, 61020, retryResp)
	if retryResp.GetResult() == 0 {
		t.Fatalf("expected retry submit failure")
	}

	rankBuf, _ := proto.Marshal(&protobuf.CS_61037{Id: proto.Uint32(1001)})
	if _, _, err := answer.GuildGetReportRankCommandResponse(&rankBuf, client); err != nil {
		t.Fatalf("GuildGetReportRankCommandResponse failed: %v", err)
	}
	rankResp := &protobuf.SC_61038{}
	decodeTestPacket(t, client, 61038, rankResp)
	if len(rankResp.GetList()) != 2 {
		t.Fatalf("expected two rank entries, got %d", len(rankResp.GetList()))
	}
	if rankResp.GetList()[0].GetUserId() != 11 {
		t.Fatalf("expected highest damage entry first")
	}
}

func TestGuildEventChunk4JoinEvent(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterID := uint32(7004)
	seedGuildEventChunk4Config(t, chapterID)
	client, guildID := createGuildEventClient(t, 87131)
	defer cleanupGuildEventChunk4Data(t, guildID, 87131)
	activateGuildEventChapter(t, client, chapterID)

	joinBuf, _ := proto.Marshal(&protobuf.CS_61031{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildJoinEventCommandResponse(&joinBuf, client); err != nil {
		t.Fatalf("GuildJoinEventCommandResponse failed: %v", err)
	}
	joinResp := &protobuf.SC_61032{}
	decodeTestPacket(t, client, 61032, joinResp)
	if joinResp.GetResult() != 0 {
		t.Fatalf("expected join event success, got %d", joinResp.GetResult())
	}

	snapshotBuf, _ := proto.Marshal(&protobuf.CS_61005{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildGetActivationEventCommandResponse(&snapshotBuf, client); err != nil {
		t.Fatalf("snapshot after join failed: %v", err)
	}
	snapshotResp := &protobuf.SC_61006{}
	decodeTestPacket(t, client, 61006, snapshotResp)
	if snapshotResp.GetOperation().GetJoinTimes() != 1 || snapshotResp.GetOperation().GetIsParticipant() != 1 {
		t.Fatalf("expected join_times/is_participant to be updated")
	}

	var liveness int64
	if err := db.DefaultStore.Pool.QueryRow(t.Context(), "SELECT liveness FROM guild_members WHERE guild_id = $1 AND commander_id = $2", int64(guildID), int64(client.Commander.CommanderID)).Scan(&liveness); err != nil {
		t.Fatalf("query liveness: %v", err)
	}
	if liveness != 1 {
		t.Fatalf("expected liveness incremented, got %d", liveness)
	}
}

func TestGuildEventChunk4ActivateInsufficientCapital(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterID := uint32(7010)
	seedGuildEventChunk4Config(t, chapterID)
	client, guildID := createGuildEventClient(t, 87141)
	defer cleanupGuildEventChunk4Data(t, guildID, 87141)

	execAnswerExternalTestSQLT(t, "UPDATE guilds SET capital = 99 WHERE id = $1", int64(guildID))

	buf, _ := proto.Marshal(&protobuf.CS_61001{ChapterId: proto.Uint32(chapterID)})
	if _, _, err := answer.GuildActiveEventCommandResponse(&buf, client); err != nil {
		t.Fatalf("GuildActiveEventCommandResponse failed: %v", err)
	}
	resp := &protobuf.SC_61002{}
	decodeTestPacket(t, client, 61002, resp)
	if resp.GetResult() != 2 {
		t.Fatalf("expected insufficient-capital result 2, got %d", resp.GetResult())
	}
}

func TestGuildEventChunk4ReactivationResetsParticipantsAndScopesEvents(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterA := uint32(7011)
	chapterB := uint32(7012)
	seedGuildEventChunk4Config(t, chapterA)
	seedGuildEventChunk4Config(t, chapterB)
	client, guildID := createGuildEventClient(t, 87151)
	defer cleanupGuildEventChunk4Data(t, guildID, 87151)

	activateGuildEventChapter(t, client, chapterA)

	joinBuf, _ := proto.Marshal(&protobuf.CS_61031{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildJoinEventCommandResponse(&joinBuf, client); err != nil {
		t.Fatalf("GuildJoinEventCommandResponse failed: %v", err)
	}
	joinResp := &protobuf.SC_61032{}
	decodeTestPacket(t, client, 61032, joinResp)
	if joinResp.GetResult() != 0 {
		t.Fatalf("expected first join success, got %d", joinResp.GetResult())
	}

	execAnswerExternalTestSQLT(t, "UPDATE guild_operation_states SET end_time = 1 WHERE guild_id = $1", int64(guildID))

	activateGuildEventChapter(t, client, chapterB)

	snapshotBuf, _ := proto.Marshal(&protobuf.CS_61005{Type: proto.Uint32(0)})
	if _, _, err := answer.GuildGetActivationEventCommandResponse(&snapshotBuf, client); err != nil {
		t.Fatalf("GuildGetActivationEventCommandResponse failed: %v", err)
	}
	snapshotResp := &protobuf.SC_61006{}
	decodeTestPacket(t, client, 61006, snapshotResp)
	if snapshotResp.GetOperation().GetOperationId() != chapterB {
		t.Fatalf("expected operation id %d, got %d", chapterB, snapshotResp.GetOperation().GetOperationId())
	}
	if len(snapshotResp.GetOperation().GetBaseEvents()) != 1 || snapshotResp.GetOperation().GetBaseEvents()[0].GetEventId() != chapterB {
		t.Fatalf("expected snapshot scoped to chapter %d", chapterB)
	}
	if snapshotResp.GetOperation().GetJoinTimes() != 0 {
		t.Fatalf("expected join times reset to 0 on new operation, got %d", snapshotResp.GetOperation().GetJoinTimes())
	}
}

func TestGuildEventChunk4SubmitReportRollbackOnDropFailure(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	chapterID := uint32(7013)
	seedGuildEventChunk4Config(t, chapterID)
	client, guildID := createGuildEventClient(t, 87161)
	defer cleanupGuildEventChunk4Data(t, guildID, 87161)

	execAnswerExternalTestSQLT(t, "INSERT INTO guild_reports (guild_id, id, event_id, event_type, score, status, claimed, drop_type, drop_id, drop_count) VALUES ($1, 1101, 5101, 2, 100, 1, false, 999, 1, 1)", int64(guildID))

	submitBuf, _ := proto.Marshal(&protobuf.CS_61019{Ids: []uint32{1101}})
	if _, _, err := answer.SubmitGuildReportCommandResponse(&submitBuf, client); err != nil {
		t.Fatalf("SubmitGuildReportCommandResponse failed: %v", err)
	}
	submitResp := &protobuf.SC_61020{}
	decodeTestPacket(t, client, 61020, submitResp)
	if submitResp.GetResult() == 0 {
		t.Fatalf("expected submit failure for unsupported drop type")
	}

	var claimed bool
	if err := db.DefaultStore.Pool.QueryRow(t.Context(), "SELECT claimed FROM guild_reports WHERE guild_id = $1 AND id = 1101", int64(guildID)).Scan(&claimed); err != nil {
		t.Fatalf("query claimed status: %v", err)
	}
	if claimed {
		t.Fatalf("expected report claim rollback on drop failure")
	}
}
