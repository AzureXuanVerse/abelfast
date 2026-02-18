package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestMiniGameOperationCompleteSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	seedMiniGameHubAndGameConfig(t, 8801, 7001, 2, 1, []uint32{2, 90001, 2})

	req := &protobuf.CS_26103{Hubid: proto.Uint32(8801), Cmd: proto.Uint32(miniGameCmdComplete), Args1: []uint32{1, 10, 7001}}
	payload, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := MiniGameOperation(&payload, client); err != nil {
		t.Fatalf("MiniGameOperation failed: %v", err)
	}

	resp := &protobuf.SC_26104{}
	decodePacketAt(t, client, 0, 26104, resp)
	if resp.GetResult() != miniGameOpResultSuccess {
		t.Fatalf("expected success result, got %d", resp.GetResult())
	}
	if resp.GetHub() == nil || resp.GetHub().GetId() != 8801 {
		t.Fatalf("expected hub data for 8801")
	}
	if resp.GetData() == nil || resp.GetData().GetId() != 7001 {
		t.Fatalf("expected minigame data for game 7001")
	}
	if len(resp.GetAwardList()) != 1 {
		t.Fatalf("expected one award, got %d", len(resp.GetAwardList()))
	}
	if got := queryInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(90001)); got != 2 {
		t.Fatalf("expected awarded item count 2, got %d", got)
	}
}

func TestMiniGameOperationInvalidCommandFailsWithoutAwards(t *testing.T) {
	client := setupHandlerCommander(t)
	seedMiniGameHubAndGameConfig(t, 8802, 7002, 2, 2, []uint32{2, 90002, 3})

	req := &protobuf.CS_26103{Hubid: proto.Uint32(8802), Cmd: proto.Uint32(99), Args1: []uint32{7002}}
	payload, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := MiniGameOperation(&payload, client); err != nil {
		t.Fatalf("MiniGameOperation failed: %v", err)
	}

	resp := &protobuf.SC_26104{}
	decodePacketAt(t, client, 0, 26104, resp)
	if resp.GetResult() != miniGameOpResultFailure {
		t.Fatalf("expected failure result, got %d", resp.GetResult())
	}
	if len(resp.GetAwardList()) != 0 {
		t.Fatalf("expected no awards on failure")
	}
	if got := queryInt64(t, "SELECT COUNT(*) FROM commander_items WHERE commander_id = $1", int64(client.Commander.CommanderID)); got != 0 {
		t.Fatalf("expected no item mutation, got %d rows", got)
	}
}

func TestMiniGameOperationBatchSendsPerOperation(t *testing.T) {
	client := setupHandlerCommander(t)
	seedMiniGameHubAndGameConfig(t, 8803, 7003, 3, 3, []uint32{2, 90003, 1})

	req := &protobuf.CS_26105{Combine: []*protobuf.CS_26103{
		{Hubid: proto.Uint32(8803), Cmd: proto.Uint32(miniGameCmdPlay), Args1: []uint32{7003}},
		{Hubid: proto.Uint32(8803), Cmd: proto.Uint32(miniGameCmdHighScore), Args1: []uint32{7003, 321, 9}},
	}}
	payload, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := MiniGameOperationBatch(&payload, client); err != nil {
		t.Fatalf("MiniGameOperationBatch failed: %v", err)
	}

	offset := 0
	resp1 := &protobuf.SC_26104{}
	offset = decodePacketAt(t, client, offset, 26104, resp1)
	if resp1.GetResult() != miniGameOpResultSuccess {
		t.Fatalf("expected first operation success")
	}
	resp2 := &protobuf.SC_26104{}
	offset = decodePacketAt(t, client, offset, 26104, resp2)
	if resp2.GetResult() != miniGameOpResultSuccess {
		t.Fatalf("expected second operation success")
	}
	if len(resp2.GetHub().GetMaxscores()) != 1 {
		t.Fatalf("expected high score in second response")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly two batched responses")
	}
}

func TestActivityItemListReturnsScopedBalances(t *testing.T) {
	client := setupHandlerCommander(t)
	seedActivityTemplate(t, 9801, []any{93001, map[string]any{"nested": []any{93002, map[string]any{"deep": "93003"}}}})
	seedCommanderItemCount(t, client.Commander.CommanderID, 93001, 4)
	seedCommanderMiscItemCount(t, client.Commander.CommanderID, 93002, 6)
	seedCommanderItemCount(t, client.Commander.CommanderID, 93003, 2)

	req := &protobuf.CS_26106{ActId: proto.Uint32(9801)}
	payload, _ := proto.Marshal(req)
	if _, _, err := ActivityItemList(&payload, client); err != nil {
		t.Fatalf("ActivityItemList failed: %v", err)
	}
	resp := &protobuf.SC_26107{}
	decodePacketAt(t, client, 0, 26107, resp)
	if resp.GetRet() != activityItemResultSuccess {
		t.Fatalf("expected success ret, got %d", resp.GetRet())
	}
	if len(resp.GetItemList()) != 3 {
		t.Fatalf("expected three scoped items, got %d", len(resp.GetItemList()))
	}
	client.Buffer.Reset()

	invalidPayload, _ := proto.Marshal(&protobuf.CS_26106{ActId: proto.Uint32(999999)})
	if _, _, err := ActivityItemList(&invalidPayload, client); err != nil {
		t.Fatalf("ActivityItemList invalid request failed: %v", err)
	}
	invalidResp := &protobuf.SC_26107{}
	decodePacketAt(t, client, 0, 26107, invalidResp)
	if invalidResp.GetRet() != activityItemResultFailure {
		t.Fatalf("expected failure ret for unknown activity")
	}
}

func TestIslandRequestNodeListReturnsSavedState(t *testing.T) {
	client := setupHandlerCommander(t)
	seedActivityTemplate(t, 9901, []any{})
	if err := orm.SaveIslandNodeState(client.Commander.CommanderID, 9901, []orm.IslandNodeState{{ID: 3, EventID: 77, IsNew: 1}}); err != nil {
		t.Fatalf("save island nodes: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_26108{ActId: proto.Uint32(9901)})
	if _, _, err := IslandRequestNodeList(&payload, client); err != nil {
		t.Fatalf("IslandRequestNodeList failed: %v", err)
	}
	resp := &protobuf.SC_26109{}
	decodePacketAt(t, client, 0, 26109, resp)
	if resp.GetRet() != islandNodeResultSuccess {
		t.Fatalf("expected success ret, got %d", resp.GetRet())
	}
	if len(resp.GetNodeList()) != 1 || resp.GetNodeList()[0].GetId() != 3 {
		t.Fatalf("unexpected island node list: %+v", resp.GetNodeList())
	}
}

func TestMiniGameTimeSubmitPersistsValidTelemetry(t *testing.T) {
	client := setupHandlerCommander(t)
	seedMiniGameHubAndGameConfig(t, 8804, 7004, 1, 1, []uint32{})

	payload, _ := proto.Marshal(&protobuf.CS_26110{Gameid: proto.Uint32(7004), Time: proto.Uint32(120)})
	if _, _, err := MiniGameTimeSubmit(&payload, client); err != nil {
		t.Fatalf("MiniGameTimeSubmit failed: %v", err)
	}
	telemetry, err := orm.GetOrCreateMiniGameTelemetryState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load telemetry: %v", err)
	}
	if telemetry.GameTimes[7004] != 120 {
		t.Fatalf("expected telemetry 120, got %d", telemetry.GameTimes[7004])
	}

	invalidPayload, _ := proto.Marshal(&protobuf.CS_26110{Gameid: proto.Uint32(7004), Time: proto.Uint32(0)})
	if _, _, err := MiniGameTimeSubmit(&invalidPayload, client); err != nil {
		t.Fatalf("MiniGameTimeSubmit invalid failed: %v", err)
	}
	telemetry, err = orm.GetOrCreateMiniGameTelemetryState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("reload telemetry: %v", err)
	}
	if telemetry.GameTimes[7004] != 120 {
		t.Fatalf("expected telemetry unchanged at 120, got %d", telemetry.GameTimes[7004])
	}
}

func TestMiniGameFriendRankReturnsSortedScores(t *testing.T) {
	client := setupHandlerCommander(t)
	seedMiniGameHubAndGameConfig(t, 8805, 7005, 2, 1, []uint32{})

	secondCommanderID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(secondCommanderID, secondCommanderID, fmt.Sprintf("Rank Commander %d", secondCommanderID), 0, 0); err != nil {
		t.Fatalf("create second commander: %v", err)
	}

	hubConfig, err := orm.GetMiniGameHubConfig(8805)
	if err != nil {
		t.Fatalf("load hub config: %v", err)
	}
	firstState, err := orm.GetOrCreateMiniGameHubState(client.Commander.CommanderID, hubConfig)
	if err != nil {
		t.Fatalf("load first state: %v", err)
	}
	firstState.MaxScores[7005] = orm.MiniGameScoreEntry{Score: 150, Extra: 20}
	if err := orm.SaveMiniGameHubState(firstState); err != nil {
		t.Fatalf("save first state: %v", err)
	}
	secondState, err := orm.GetOrCreateMiniGameHubState(secondCommanderID, hubConfig)
	if err != nil {
		t.Fatalf("load second state: %v", err)
	}
	secondState.MaxScores[7005] = orm.MiniGameScoreEntry{Score: 220, Extra: 10}
	if err := orm.SaveMiniGameHubState(secondState); err != nil {
		t.Fatalf("save second state: %v", err)
	}

	payload, _ := proto.Marshal(&protobuf.CS_26111{Gameid: proto.Uint32(7005)})
	if _, _, err := MiniGameFriendRank(&payload, client); err != nil {
		t.Fatalf("MiniGameFriendRank failed: %v", err)
	}
	resp := &protobuf.SC_26112{}
	decodePacketAt(t, client, 0, 26112, resp)
	if len(resp.GetRanks()) < 2 {
		t.Fatalf("expected at least two ranks, got %d", len(resp.GetRanks()))
	}
	if resp.GetRanks()[0].GetScore() < resp.GetRanks()[1].GetScore() {
		t.Fatalf("expected ranks sorted descending")
	}
}

func TestMiniGameFriendRankCapsResponseSize(t *testing.T) {
	client := setupHandlerCommander(t)
	seedMiniGameHubAndGameConfig(t, 8806, 7006, 2, 1, []uint32{})

	hubConfig, err := orm.GetMiniGameHubConfig(8806)
	if err != nil {
		t.Fatalf("load hub config: %v", err)
	}
	for i := 0; i < 105; i++ {
		commanderID := uint32(time.Now().UnixNano()) + uint32(i) + 1
		if err := orm.CreateCommanderRoot(commanderID, commanderID, fmt.Sprintf("Cap Commander %d", commanderID), 0, 0); err != nil {
			t.Fatalf("create commander %d: %v", commanderID, err)
		}
		state, err := orm.GetOrCreateMiniGameHubState(commanderID, hubConfig)
		if err != nil {
			t.Fatalf("load state %d: %v", commanderID, err)
		}
		state.MaxScores[7006] = orm.MiniGameScoreEntry{Score: uint32(1000 - i), Extra: uint32(i)}
		if err := orm.SaveMiniGameHubState(state); err != nil {
			t.Fatalf("save state %d: %v", commanderID, err)
		}
	}

	payload, _ := proto.Marshal(&protobuf.CS_26111{Gameid: proto.Uint32(7006)})
	if _, _, err := MiniGameFriendRank(&payload, client); err != nil {
		t.Fatalf("MiniGameFriendRank failed: %v", err)
	}
	resp := &protobuf.SC_26112{}
	decodePacketAt(t, client, 0, 26112, resp)
	if len(resp.GetRanks()) != 100 {
		t.Fatalf("expected rank list capped at 100, got %d", len(resp.GetRanks()))
	}
}

func seedMiniGameHubAndGameConfig(t *testing.T, hubID uint32, gameID uint32, reborn uint32, rewardNeed uint32, rewardDisplay []uint32) {
	t.Helper()
	hubPayload, err := json.Marshal(map[string]any{
		"id":             hubID,
		"reborn_times":   reborn,
		"reward_need":    rewardNeed,
		"reward_display": rewardDisplay,
	})
	if err != nil {
		t.Fatalf("marshal mini game hub config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/mini_game_hub.json", fmt.Sprintf("%d", hubID), hubPayload); err != nil {
		t.Fatalf("upsert mini game hub config: %v", err)
	}
	gamePayload, err := json.Marshal(map[string]any{"id": gameID, "hub_id": hubID})
	if err != nil {
		t.Fatalf("marshal mini game config: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/mini_game.json", fmt.Sprintf("%d", gameID), gamePayload); err != nil {
		t.Fatalf("upsert mini game config: %v", err)
	}
}

func seedActivityTemplate(t *testing.T, actID uint32, configData any) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"id": actID, "config_data": configData, "config_client": map[string]any{}})
	if err != nil {
		t.Fatalf("marshal activity template: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/activity_template.json", fmt.Sprintf("%d", actID), payload); err != nil {
		t.Fatalf("upsert activity template: %v", err)
	}
}

func seedCommanderItemCount(t *testing.T, commanderID uint32, itemID uint32, count uint32) {
	t.Helper()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commander_items (commander_id, item_id, count) VALUES ($1, $2, $3) ON CONFLICT (commander_id, item_id) DO UPDATE SET count = EXCLUDED.count`, int64(commanderID), int64(itemID), int64(count)); err != nil {
		t.Fatalf("seed commander item: %v", err)
	}
}

func seedCommanderMiscItemCount(t *testing.T, commanderID uint32, itemID uint32, count uint32) {
	t.Helper()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `DELETE FROM commander_misc_items WHERE commander_id = $1 AND item_id = $2`, int64(commanderID), int64(itemID)); err != nil {
		t.Fatalf("clear commander misc item: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commander_misc_items (commander_id, item_id, data) VALUES ($1, $2, $3)`, int64(commanderID), int64(itemID), int64(count)); err != nil {
		t.Fatalf("seed commander misc item: %v", err)
	}
}

func queryInt64(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var value int64
	if err := db.DefaultStore.Pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	return value
}
