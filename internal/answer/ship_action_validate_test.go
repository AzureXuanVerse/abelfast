package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestShipAction12020Success(t *testing.T) {
	commander := &orm.Commander{
		OwnedShipsMap: map[uint32]*orm.OwnedShip{
			10: {ID: 10, Intimacy: 7300},
		},
	}
	client := &connection.Client{Commander: commander}
	payload := protobuf.CS_12020{ShipId: proto.Uint32(10)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := HandleShipActionValidate(&buffer, client)
	if err != nil {
		t.Fatalf("ship action failed: %v", err)
	}
	if packetID != 12021 {
		t.Fatalf("expected packet 12021, got %d", packetID)
	}

	var response protobuf.SC_12021
	offset := decodePacketAt(t, client, 0, 12021, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	var push protobuf.SC_12019
	offset = decodePacketAt(t, client, offset, 12019, &push)
	if push.GetIntimacy() != 7300 {
		t.Fatalf("expected intimacy 7300, got %d", push.GetIntimacy())
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly two packets in buffer")
	}
}

func TestShipAction12020MissingShip(t *testing.T) {
	commander := &orm.Commander{OwnedShipsMap: map[uint32]*orm.OwnedShip{}}
	client := &connection.Client{Commander: commander}
	payload := protobuf.CS_12020{ShipId: proto.Uint32(99)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := HandleShipActionValidate(&buffer, client)
	if err != nil {
		t.Fatalf("ship action failed: %v", err)
	}
	if packetID != 12021 {
		t.Fatalf("expected packet 12021, got %d", packetID)
	}

	var response protobuf.SC_12021
	offset := decodePacketAt(t, client, 0, 12021, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected result 1, got %d", response.GetResult())
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected no push packet for missing ship")
	}
}

func TestShipAction12020BadPayload(t *testing.T) {
	commander := &orm.Commander{OwnedShipsMap: map[uint32]*orm.OwnedShip{}}
	client := &connection.Client{Commander: commander}
	buffer := []byte{0xff}

	_, packetID, err := HandleShipActionValidate(&buffer, client)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if packetID != 12021 {
		t.Fatalf("expected packet 12021, got %d", packetID)
	}
	if len(client.Buffer.Bytes()) != 0 {
		t.Fatalf("expected no response packets on bad payload")
	}
}
