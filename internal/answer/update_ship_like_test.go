package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestUpdateShipLikeSuccessSendsCollectionGroupUpdate(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.Like{})

	const groupID uint32 = 45678
	shipTemplateID := uint32(groupID*10 + 1)
	seedShipTemplate(t, shipTemplateID, 1, 2, 1, "Like Ship", 4)
	seedOwnedShip(t, client, shipTemplateID)

	payload := protobuf.CS_17107{ShipGroupId: proto.Uint32(groupID)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := UpdateShipLike(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var result protobuf.SC_17108
	offset := decodePacketAt(t, client, 0, 17108, &result)
	if result.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", result.GetResult())
	}

	var update protobuf.SC_17004
	offset = decodePacketAt(t, client, offset, 17004, &update)
	if update.GetShipInfo().GetId() != groupID {
		t.Fatalf("expected ship_info.id %d, got %d", groupID, update.GetShipInfo().GetId())
	}
	if update.GetShipInfo().GetHeartFlag() != 1 {
		t.Fatalf("expected heart_flag 1, got %d", update.GetShipInfo().GetHeartFlag())
	}
	if update.GetShipInfo().GetHeartCount() != 1 {
		t.Fatalf("expected heart_count 1, got %d", update.GetShipInfo().GetHeartCount())
	}

	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly two packets")
	}
}

func TestUpdateShipLikeWithoutCollectionDataDoesNotSendCollectionGroupUpdate(t *testing.T) {
	client := setupHandlerCommander(t)

	payload := protobuf.CS_17107{ShipGroupId: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := UpdateShipLike(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var result protobuf.SC_17108
	offset := decodePacketAt(t, client, 0, 17108, &result)
	if result.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", result.GetResult())
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected only SC_17108 packet when no collection row exists")
	}
}
