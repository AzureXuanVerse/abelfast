package answer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const metaPtCategory = "ShareCfg/ship_strengthen_meta.json"

func TestClaimMetaPtAwardSuccessAndBootstrapConsistency(t *testing.T) {
	client := setupMetaPtTestClient(t)
	seedMetaPtConfigEntry(t, 970108, `{"id":970108,"type":1,"target":[100,300],"award_display":[[1,1,100],[1,1,200]]}`)
	if err := orm.SaveCommanderMetaPtProgress(&orm.CommanderMetaPtProgress{
		CommanderID: client.Commander.CommanderID,
		GroupID:     970108,
		Pt:          300,
		FetchList:   []uint32{},
	}); err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	startGold := client.Commander.GetResourceCount(1)
	payload := marshalPacketRequest(t, &protobuf.CS_34003{GroupId: proto.Uint32(970108), TargetPt: proto.Uint32(100)})
	if _, _, err := ClaimMetaPtAward(&payload, client); err != nil {
		t.Fatalf("claim meta pt award failed: %v", err)
	}

	response := &protobuf.SC_34004{}
	decodeLoveLetterPacketMessage(t, client, 34004, response)
	if response.GetResult() != metaPtClaimResultSuccess {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetType() != 1 || response.GetDropList()[0].GetId() != 1 || response.GetDropList()[0].GetNumber() != 100 {
		t.Fatalf("unexpected drop_list: %+v", response.GetDropList())
	}
	if client.Commander.GetResourceCount(1) != startGold+100 {
		t.Fatalf("expected gold increase by 100")
	}

	stored, err := orm.GetCommanderMetaPtProgress(client.Commander.CommanderID, 970108)
	if err != nil {
		t.Fatalf("load stored progress: %v", err)
	}
	if len(stored.FetchList) != 1 || stored.FetchList[0] != 100 {
		t.Fatalf("expected claimed threshold persisted, got %+v", stored.FetchList)
	}

	emptyPayload := []byte{}
	if _, _, err := GetMetaShipsPointsResponse(&emptyPayload, client); err != nil {
		t.Fatalf("get meta ships points failed: %v", err)
	}
	bootstrap := &protobuf.SC_34002{}
	decodeLoveLetterPacketMessage(t, client, 34002, bootstrap)
	if len(bootstrap.GetMetaShipList()) != 1 {
		t.Fatalf("expected one meta ship info, got %+v", bootstrap.GetMetaShipList())
	}
	info := bootstrap.GetMetaShipList()[0]
	if info.GetGroupId() != 970108 || info.GetPt() != 300 || len(info.GetFetchList()) != 1 || info.GetFetchList()[0] != 100 {
		t.Fatalf("unexpected bootstrap info: %+v", info)
	}
}

func TestClaimMetaPtAwardFailures(t *testing.T) {
	t.Run("already claimed", func(t *testing.T) {
		client := setupMetaPtTestClient(t)
		seedMetaPtConfigEntry(t, 970108, `{"id":970108,"type":1,"target":[100],"award_display":[[1,1,100]]}`)
		if err := orm.SaveCommanderMetaPtProgress(&orm.CommanderMetaPtProgress{CommanderID: client.Commander.CommanderID, GroupID: 970108, Pt: 200, FetchList: []uint32{100}}); err != nil {
			t.Fatalf("seed progress: %v", err)
		}
		payload := marshalPacketRequest(t, &protobuf.CS_34003{GroupId: proto.Uint32(970108), TargetPt: proto.Uint32(100)})
		if _, _, err := ClaimMetaPtAward(&payload, client); err != nil {
			t.Fatalf("claim call failed: %v", err)
		}
		response := &protobuf.SC_34004{}
		decodeLoveLetterPacketMessage(t, client, 34004, response)
		if response.GetResult() != metaPtClaimResultClaimed || len(response.GetDropList()) != 0 {
			t.Fatalf("unexpected response: result=%d drops=%+v", response.GetResult(), response.GetDropList())
		}
	})

	t.Run("insufficient pt", func(t *testing.T) {
		client := setupMetaPtTestClient(t)
		seedMetaPtConfigEntry(t, 970108, `{"id":970108,"type":1,"target":[100],"award_display":[[1,1,100]]}`)
		if err := orm.SaveCommanderMetaPtProgress(&orm.CommanderMetaPtProgress{CommanderID: client.Commander.CommanderID, GroupID: 970108, Pt: 50, FetchList: []uint32{}}); err != nil {
			t.Fatalf("seed progress: %v", err)
		}
		payload := marshalPacketRequest(t, &protobuf.CS_34003{GroupId: proto.Uint32(970108), TargetPt: proto.Uint32(100)})
		if _, _, err := ClaimMetaPtAward(&payload, client); err != nil {
			t.Fatalf("claim call failed: %v", err)
		}
		response := &protobuf.SC_34004{}
		decodeLoveLetterPacketMessage(t, client, 34004, response)
		if response.GetResult() != metaPtClaimResultInsufficient || len(response.GetDropList()) != 0 {
			t.Fatalf("unexpected response: result=%d drops=%+v", response.GetResult(), response.GetDropList())
		}
	})

	t.Run("invalid target", func(t *testing.T) {
		client := setupMetaPtTestClient(t)
		seedMetaPtConfigEntry(t, 970108, `{"id":970108,"type":1,"target":[100],"award_display":[[1,1,100]]}`)
		if err := orm.SaveCommanderMetaPtProgress(&orm.CommanderMetaPtProgress{CommanderID: client.Commander.CommanderID, GroupID: 970108, Pt: 200, FetchList: []uint32{}}); err != nil {
			t.Fatalf("seed progress: %v", err)
		}
		payload := marshalPacketRequest(t, &protobuf.CS_34003{GroupId: proto.Uint32(970108), TargetPt: proto.Uint32(500)})
		if _, _, err := ClaimMetaPtAward(&payload, client); err != nil {
			t.Fatalf("claim call failed: %v", err)
		}
		response := &protobuf.SC_34004{}
		decodeLoveLetterPacketMessage(t, client, 34004, response)
		if response.GetResult() != metaPtClaimResultInvalidTier || len(response.GetDropList()) != 0 {
			t.Fatalf("unexpected response: result=%d drops=%+v", response.GetResult(), response.GetDropList())
		}
	})

	t.Run("invalid group", func(t *testing.T) {
		client := setupMetaPtTestClient(t)
		payload := marshalPacketRequest(t, &protobuf.CS_34003{GroupId: proto.Uint32(999999), TargetPt: proto.Uint32(100)})
		if _, _, err := ClaimMetaPtAward(&payload, client); err != nil {
			t.Fatalf("claim call failed: %v", err)
		}
		response := &protobuf.SC_34004{}
		decodeLoveLetterPacketMessage(t, client, 34004, response)
		if response.GetResult() != metaPtClaimResultInvalidGroup || len(response.GetDropList()) != 0 {
			t.Fatalf("unexpected response: result=%d drops=%+v", response.GetResult(), response.GetDropList())
		}
	})
}

func TestClaimMetaPtAwardConcurrentSingleSuccess(t *testing.T) {
	baseClient := setupMetaPtTestClient(t)
	seedMetaPtConfigEntry(t, 970108, `{"id":970108,"type":1,"target":[100],"award_display":[[1,1,100]]}`)
	if err := orm.SaveCommanderMetaPtProgress(&orm.CommanderMetaPtProgress{CommanderID: baseClient.Commander.CommanderID, GroupID: 970108, Pt: 100, FetchList: []uint32{}}); err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	clientA := mustLoadMetaPtCommanderClient(t, baseClient.Commander.CommanderID)
	clientB := mustLoadMetaPtCommanderClient(t, baseClient.Commander.CommanderID)
	payload := marshalPacketRequest(t, &protobuf.CS_34003{GroupId: proto.Uint32(970108), TargetPt: proto.Uint32(100)})
	startingGold := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(baseClient.Commander.CommanderID), int64(1))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, _ = ClaimMetaPtAward(&payload, clientA)
	}()
	go func() {
		defer wg.Done()
		_, _, _ = ClaimMetaPtAward(&payload, clientB)
	}()
	wg.Wait()

	respA := &protobuf.SC_34004{}
	decodeLoveLetterPacketMessage(t, clientA, 34004, respA)
	respB := &protobuf.SC_34004{}
	decodeLoveLetterPacketMessage(t, clientB, 34004, respB)

	successCount := 0
	for _, result := range []uint32{respA.GetResult(), respB.GetResult()} {
		if result == metaPtClaimResultSuccess {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly one success, got respA=%d respB=%d", respA.GetResult(), respB.GetResult())
	}

	stored, err := orm.GetCommanderMetaPtProgress(baseClient.Commander.CommanderID, 970108)
	if err != nil {
		t.Fatalf("load stored progress: %v", err)
	}
	if len(stored.FetchList) != 1 || stored.FetchList[0] != 100 {
		t.Fatalf("expected exactly one claimed milestone, got %+v", stored.FetchList)
	}

	gold := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(baseClient.Commander.CommanderID), int64(1))
	if gold != startingGold+100 {
		t.Fatalf("expected gold to increase once by 100, start=%d got=%d", startingGold, gold)
	}
}

func setupMetaPtTestClient(t *testing.T) *connection.Client {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderMetaPtProgress{})
	return client
}

func seedMetaPtConfigEntry(t *testing.T, groupID uint32, payload string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(metaPtCategory, fmt.Sprintf("%d", groupID), []byte(payload)); err != nil {
		t.Fatalf("seed meta pt config: %v", err)
	}
}

func mustLoadMetaPtCommanderClient(t *testing.T, commanderID uint32) *connection.Client {
	t.Helper()
	commander := &orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander %d: %v", commanderID, err)
	}
	return &connection.Client{Commander: commander}
}
