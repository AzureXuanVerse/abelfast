package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestExchangeShipSuccessEmitsSC12042(t *testing.T) {
	originalAdd := exchangeShipAddOwnedShip
	originalCommit := exchangeShipCommitCommander
	t.Cleanup(func() {
		exchangeShipAddOwnedShip = originalAdd
		exchangeShipCommitCommander = originalCommit
	})

	newShip := &orm.OwnedShip{ID: 2001, ShipID: 105171, Level: 1, MaxLevel: 100, Energy: 150}
	exchangeShipAddOwnedShip = func(client *connection.Client, shipID uint32) (*orm.OwnedShip, error) {
		return newShip, nil
	}
	exchangeShipCommitCommander = func(client *connection.Client) error {
		return nil
	}

	client := &connection.Client{Commander: &orm.Commander{ExchangeCount: 400}}
	payload := protobuf.CS_12047{ShipTid: proto.Uint32(105171)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := ExchangeShip(&buffer, client)
	if err != nil {
		t.Fatalf("exchange ship failed: %v", err)
	}
	if packetID != 12048 {
		t.Fatalf("expected packet 12048, got %d", packetID)
	}

	var ack protobuf.SC_12048
	offset := decodePacketAt(t, client, 0, 12048, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", ack.GetResult())
	}

	var push protobuf.SC_12042
	offset = decodePacketAt(t, client, offset, 12042, &push)
	if len(push.GetShipList()) != 1 {
		t.Fatalf("expected one pushed ship, got %d", len(push.GetShipList()))
	}
	if push.GetShipList()[0].GetId() != 2001 {
		t.Fatalf("expected pushed ship id 2001, got %d", push.GetShipList()[0].GetId())
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly ack + push packets")
	}
}

func TestExchangeShipInvalidTemplateNoSC12042(t *testing.T) {
	client := &connection.Client{Commander: &orm.Commander{ExchangeCount: 400}}
	payload := protobuf.CS_12047{ShipTid: proto.Uint32(1001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := ExchangeShip(&buffer, client)
	if err != nil {
		t.Fatalf("exchange ship failed: %v", err)
	}
	if packetID != 12048 {
		t.Fatalf("expected packet 12048, got %d", packetID)
	}

	var ack protobuf.SC_12048
	offset := decodePacketAt(t, client, 0, 12048, &ack)
	if ack.GetResult() == 0 {
		t.Fatalf("expected failure result")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected no sc_12042 push on failure")
	}
}
