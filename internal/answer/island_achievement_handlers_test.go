package answer

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
)

func seedIslandAchievementState(t *testing.T, commanderID uint32, entries []orm.IslandAchievementProgressEntry, finishList []uint32) {
	t.Helper()
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandAchievementStateForUpdateTx(context.Background(), tx, commanderID)
		if err != nil {
			return err
		}
		state.ProgressEntries = append([]orm.IslandAchievementProgressEntry(nil), entries...)
		state.FinishList = append([]uint32(nil), finishList...)
		return orm.SaveIslandAchievementStateTx(context.Background(), tx, state)
	})
	if err != nil {
		t.Fatalf("seed achievement state: %v", err)
	}
}

func seedIslandAchievementConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, islandAchievementCategory, "101", `{"id":101,"target_type":1,"target_value1":1001,"target_num":5,"award_display":[[2,20001,2]]}`)
	seedConfigEntry(t, islandAchievementCategory, "102", `{"id":102,"target_type":2,"target_value1":2002,"target_num":3,"award_display":[[2,20001,3]]}`)
	seedConfigEntry(t, islandAchievementCategory, "103", `{"id":103,"target_type":3,"target_value1":3003,"target_num":8,"award":[2,20001,1]}`)
}

func TestIslandSyncAchievementProgressPersistsAndHydrates(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandAchievementState{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandTechnologyState{})
	clearTable(t, &orm.IslandCommanderDressState{})
	clearTable(t, &orm.IslandShopState{})

	updatePayload := &protobuf.CS_21052{EventList: []*protobuf.PB_ISLAND_ACHIEVENT{
		{EventType: proto.Uint32(5), EventArg: proto.Uint32(2), Value: proto.Uint32(3)},
		{EventType: proto.Uint32(0), EventArg: proto.Uint32(8), Value: proto.Uint32(1)},
		{EventType: proto.Uint32(5), EventArg: proto.Uint32(2), Value: proto.Uint32(8)},
		{EventType: proto.Uint32(1), EventArg: proto.Uint32(9), Value: proto.Uint32(4)},
	}}
	updateBuffer, err := proto.Marshal(updatePayload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandSyncAchievementProgress(&updateBuffer, client); err != nil {
		t.Fatalf("sync achievement progress: %v", err)
	}

	updateResponse := protobuf.SC_21053{}
	decodePacketAt(t, client, 0, 21053, &updateResponse)
	if len(updateResponse.GetEventList()) != 2 {
		t.Fatalf("expected two normalized events, got %+v", updateResponse.GetEventList())
	}
	if updateResponse.GetEventList()[0].GetEventType() != 1 || updateResponse.GetEventList()[0].GetEventArg() != 9 || updateResponse.GetEventList()[0].GetValue() != 4 {
		t.Fatalf("unexpected first normalized event: %+v", updateResponse.GetEventList()[0])
	}
	if updateResponse.GetEventList()[1].GetEventType() != 5 || updateResponse.GetEventList()[1].GetEventArg() != 2 || updateResponse.GetEventList()[1].GetValue() != 8 {
		t.Fatalf("unexpected second normalized event: %+v", updateResponse.GetEventList()[1])
	}

	stored, err := orm.GetIslandAchievementState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load stored achievement state: %v", err)
	}
	if len(stored.ProgressEntries) != 2 {
		t.Fatalf("expected two stored progress entries, got %+v", stored.ProgressEntries)
	}

	getDataPayload := &protobuf.CS_21200{IslandId: proto.Uint32(client.Commander.CommanderID)}
	getDataBuffer, err := proto.Marshal(getDataPayload)
	if err != nil {
		t.Fatalf("marshal get data payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandGetData(&getDataBuffer, client); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}

	getDataResponse := protobuf.SC_21201{}
	decodePacketAt(t, client, 0, 21201, &getDataResponse)
	achievementSys := getDataResponse.GetIsland().GetPrivateData().GetAchievementSys()
	if achievementSys == nil || len(achievementSys.GetAchieveList()) != 2 {
		t.Fatalf("expected hydrated achievement records, got %+v", achievementSys)
	}
	if len(achievementSys.GetFinishList()) != 0 {
		t.Fatalf("expected no finished IDs after sync-only update")
	}
}

func TestIslandClaimAchievementAwardSuccessAndIdempotent(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandAchievementState{})
	seedIslandAchievementConfig(t)
	seedIslandAchievementState(t, client.Commander.CommanderID, []orm.IslandAchievementProgressEntry{
		{EventType: 1, EventArg: 1001, Value: 5},
		{EventType: 2, EventArg: 2002, Value: 6},
		{EventType: 3, EventArg: 3003, Value: 8},
	}, nil)

	claimPayload := &protobuf.CS_21050{IdList: []uint32{101, 102, 101, 103}}
	claimBuffer, err := proto.Marshal(claimPayload)
	if err != nil {
		t.Fatalf("marshal claim payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandClaimAchievementAward(&claimBuffer, client); err != nil {
		t.Fatalf("claim achievement failed: %v", err)
	}

	claimResponse := protobuf.SC_21051{}
	decodePacketAt(t, client, 0, 21051, &claimResponse)
	if claimResponse.GetResult() != islandAchievementClaimOK {
		t.Fatalf("expected success result, got %d", claimResponse.GetResult())
	}
	if len(claimResponse.GetDropList()) != 1 || claimResponse.GetDropList()[0].GetType() != 2 || claimResponse.GetDropList()[0].GetId() != 20001 || claimResponse.GetDropList()[0].GetNumber() != 6 {
		t.Fatalf("expected merged item drop [2,20001,6], got %+v", claimResponse.GetDropList())
	}

	stored, err := orm.GetIslandAchievementState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load achievement state: %v", err)
	}
	if len(stored.FinishList) != 3 || stored.FinishList[0] != 101 || stored.FinishList[1] != 102 || stored.FinishList[2] != 103 {
		t.Fatalf("expected claimed IDs persisted, got %+v", stored.FinishList)
	}

	client.Buffer.Reset()
	replayPayload := &protobuf.CS_21050{IdList: []uint32{101}}
	replayBuffer, err := proto.Marshal(replayPayload)
	if err != nil {
		t.Fatalf("marshal replay payload: %v", err)
	}
	if _, _, err := IslandClaimAchievementAward(&replayBuffer, client); err != nil {
		t.Fatalf("replay claim failed: %v", err)
	}

	replayResponse := protobuf.SC_21051{}
	decodePacketAt(t, client, 0, 21051, &replayResponse)
	if replayResponse.GetResult() == islandAchievementClaimOK {
		t.Fatalf("expected non-success replay result")
	}
	if len(replayResponse.GetDropList()) != 0 {
		t.Fatalf("expected no drops on replay, got %+v", replayResponse.GetDropList())
	}
}

func TestIslandClaimAchievementAwardRejectsUnknownID(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandAchievementState{})
	seedIslandAchievementConfig(t)
	seedIslandAchievementState(t, client.Commander.CommanderID, []orm.IslandAchievementProgressEntry{{EventType: 1, EventArg: 1001, Value: 9}}, []uint32{101})

	payload := &protobuf.CS_21050{IdList: []uint32{99999}}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandClaimAchievementAward(&buffer, client); err != nil {
		t.Fatalf("claim with unknown id failed unexpectedly: %v", err)
	}

	response := protobuf.SC_21051{}
	decodePacketAt(t, client, 0, 21051, &response)
	if response.GetResult() == islandAchievementClaimOK {
		t.Fatalf("expected unknown id to fail")
	}

	stored, err := orm.GetIslandAchievementState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load achievement state: %v", err)
	}
	if len(stored.FinishList) != 1 || stored.FinishList[0] != 101 {
		t.Fatalf("expected finish list unchanged, got %+v", stored.FinishList)
	}
}
