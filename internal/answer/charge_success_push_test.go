package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func resetChargeSuccessDedupForTest(t *testing.T) {
	t.Helper()
	clearTable(t, &orm.CommanderChargeSuccessEvent{})
}

func TestHandleChargeStartRemainsDisabled(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	payload := protobuf.CS_11501{ShopId: proto.Uint32(1), Device: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := HandleChargeStart(&buffer, client); err != nil {
		t.Fatalf("charge start: %v", err)
	}
	var response protobuf.SC_11502
	decodePacketAt(t, client, 0, 11502, &response)
	if response.GetResult() != 5002 {
		t.Fatalf("expected disabled charge result 5002, got %d", response.GetResult())
	}
}

func TestApplyChargeSuccessEventPushesSC11503(t *testing.T) {
	resetChargeSuccessDedupForTest(t)
	client := setupPlayerUpdateTest(t)
	event := ChargeSuccessEvent{ShopID: 10, PayID: "pay-001", Gem: 120, GemFree: 30}

	if err := ApplyChargeSuccessEvent(client.Commander, client, event); err != nil {
		t.Fatalf("apply charge success: %v", err)
	}
	if client.Commander.GetResourceCount(4) != 150 {
		t.Fatalf("expected gems 150, got %d", client.Commander.GetResourceCount(4))
	}

	var response protobuf.SC_11503
	decodePacketAt(t, client, 0, 11503, &response)
	if response.GetShopId() != 10 || response.GetPayId() != "pay-001" || response.GetGem() != 120 || response.GetGemFree() != 30 {
		t.Fatalf("unexpected SC_11503 payload: %+v", response)
	}
}

func TestApplyChargeSuccessEventIdempotentByPayID(t *testing.T) {
	resetChargeSuccessDedupForTest(t)
	client := setupPlayerUpdateTest(t)
	event := ChargeSuccessEvent{ShopID: 22, PayID: "pay-dup", Gem: 40, GemFree: 10}

	if err := ApplyChargeSuccessEvent(client.Commander, client, event); err != nil {
		t.Fatalf("first charge success: %v", err)
	}
	if err := ApplyChargeSuccessEvent(client.Commander, client, event); err != nil {
		t.Fatalf("duplicate charge success: %v", err)
	}

	if client.Commander.GetResourceCount(4) != 50 {
		t.Fatalf("expected gems 50 after duplicate event, got %d", client.Commander.GetResourceCount(4))
	}
	packetIDs := decodePacketIDs(t, client.Buffer.Bytes())
	if len(packetIDs) != 1 || packetIDs[0] != 11503 {
		t.Fatalf("expected a single SC_11503 packet, got %v", packetIDs)
	}

	reloadedCommander := orm.Commander{CommanderID: client.Commander.CommanderID}
	if err := reloadedCommander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	reloadedClient := &connection.Client{Commander: &reloadedCommander}
	if err := ApplyChargeSuccessEvent(reloadedClient.Commander, reloadedClient, event); err != nil {
		t.Fatalf("duplicate charge success after reload: %v", err)
	}
	totalGems := queryAnswerTestInt64(t, "SELECT COALESCE(SUM(amount), 0) FROM owned_resources WHERE commander_id = $1 AND resource_id IN (4, 14)", int64(client.Commander.CommanderID))
	if totalGems != 50 {
		t.Fatalf("expected persistent idempotency across reload, got total gems %d", totalGems)
	}
	if reloadedClient.Buffer.Len() != 0 {
		t.Fatalf("expected no push for duplicate event after reload")
	}
}

func TestApplyChargeSuccessEventOfflineCommander(t *testing.T) {
	resetChargeSuccessDedupForTest(t)
	client := setupPlayerUpdateTest(t)
	event := ChargeSuccessEvent{ShopID: 42, PayID: "pay-offline", Gem: 80, GemFree: 20}

	if err := ApplyChargeSuccessEvent(client.Commander, nil, event); err != nil {
		t.Fatalf("offline charge success: %v", err)
	}
	if client.Commander.GetResourceCount(4) != 100 {
		t.Fatalf("expected gems 100, got %d", client.Commander.GetResourceCount(4))
	}
}

func TestApplyChargeSuccessEventRejectsMalformedInput(t *testing.T) {
	resetChargeSuccessDedupForTest(t)
	client := setupPlayerUpdateTest(t)
	if err := ApplyChargeSuccessEvent(client.Commander, client, ChargeSuccessEvent{ShopID: 0, PayID: "", Gem: 50, GemFree: 10}); err == nil {
		t.Fatalf("expected malformed event error")
	}
	if client.Commander.GetResourceCount(4) != 0 {
		t.Fatalf("expected no gem mutation, got %d", client.Commander.GetResourceCount(4))
	}
	if client.Buffer.Len() != 0 {
		t.Fatalf("expected no packets for malformed event")
	}
}
