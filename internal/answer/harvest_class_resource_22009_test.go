package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestHarvestClassResourceSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})

	seedClassHarvestConfig(t)
	seedCommanderResource(t, client, classFieldResourceID, 9000)
	seedCommanderItem(t, client, 16501, 10)

	payload := protobuf.CS_22009{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := HarvestClassResource(&buffer, client); err != nil {
		t.Fatalf("harvest class resource failed: %v", err)
	}

	var response protobuf.SC_22010
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if response.GetExpInWell() != 0 {
		t.Fatalf("expected exp_in_well 0, got %d", response.GetExpInWell())
	}

	itemCount := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16501))
	if itemCount != 13 {
		t.Fatalf("expected item count 13, got %d", itemCount)
	}
	resourceCount := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(classFieldResourceID))
	if resourceCount != 0 {
		t.Fatalf("expected class field resource 0, got %d", resourceCount)
	}
}

func TestHarvestClassResourceCapacityLimitedSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})

	seedClassHarvestConfig(t)
	seedCommanderResource(t, client, classFieldResourceID, 9000)
	seedCommanderItem(t, client, 16501, 2999)

	payload := protobuf.CS_22009{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := HarvestClassResource(&buffer, client); err != nil {
		t.Fatalf("harvest class resource failed: %v", err)
	}

	var response protobuf.SC_22010
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if response.GetExpInWell() != 6000 {
		t.Fatalf("expected exp_in_well 6000, got %d", response.GetExpInWell())
	}

	itemCount := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16501))
	if itemCount != 3000 {
		t.Fatalf("expected item count 3000, got %d", itemCount)
	}
	resourceCount := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(classFieldResourceID))
	if resourceCount != 6000 {
		t.Fatalf("expected class field resource 6000, got %d", resourceCount)
	}
}

func TestHarvestClassResourceFailsWithoutClaimableCount(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})

	seedClassHarvestConfig(t)
	seedCommanderResource(t, client, classFieldResourceID, 2000)
	seedCommanderItem(t, client, 16501, 100)

	payload := protobuf.CS_22009{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := HarvestClassResource(&buffer, client); err != nil {
		t.Fatalf("harvest class resource failed: %v", err)
	}

	var response protobuf.SC_22010
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
	if response.GetExpInWell() != 2000 {
		t.Fatalf("expected exp_in_well unchanged, got %d", response.GetExpInWell())
	}
}

func TestHarvestClassResourceFailsWhenBagFull(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})

	seedClassHarvestConfig(t)
	seedCommanderResource(t, client, classFieldResourceID, 9000)
	seedCommanderItem(t, client, 16501, 3000)

	payload := protobuf.CS_22009{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := HarvestClassResource(&buffer, client); err != nil {
		t.Fatalf("harvest class resource failed: %v", err)
	}

	var response protobuf.SC_22010
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
	if response.GetExpInWell() != 9000 {
		t.Fatalf("expected exp_in_well unchanged, got %d", response.GetExpInWell())
	}
}

func TestHarvestClassResourceFailsForInvalidType(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})

	seedClassHarvestConfig(t)
	seedCommanderResource(t, client, classFieldResourceID, 9000)
	seedCommanderItem(t, client, 16501, 10)

	payload := protobuf.CS_22009{Type: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := HarvestClassResource(&buffer, client); err != nil {
		t.Fatalf("harvest class resource failed: %v", err)
	}

	var response protobuf.SC_22010
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
	if response.GetExpInWell() != 9000 {
		t.Fatalf("expected exp_in_well unchanged, got %d", response.GetExpInWell())
	}
}

func seedClassHarvestConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, classUpgradeTemplateConfig, "1", `{"id":1,"level":1,"item_id":16501}`)
	seedConfigEntry(t, itemConfigCategoryPrimary, "16501", `{"id":16501,"usage_arg":"3000","max_num":3000}`)
}
