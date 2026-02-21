package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestSC12042RoundTripMultipleShips(t *testing.T) {
	ships := []orm.OwnedShip{
		{ID: 11, ShipID: 1001, Level: 20, MaxLevel: 100, Energy: 150},
		{ID: 12, ShipID: 1002, Level: 30, MaxLevel: 100, Energy: 150},
	}
	original := &protobuf.SC_12042{
		ShipList: orm.ToProtoOwnedShipList(ships, nil, nil),
	}

	encoded, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("marshal sc_12042: %v", err)
	}

	var decoded protobuf.SC_12042
	if err := proto.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal sc_12042: %v", err)
	}

	if len(decoded.GetShipList()) != 2 {
		t.Fatalf("expected 2 ships, got %d", len(decoded.GetShipList()))
	}
	if decoded.GetShipList()[0].GetId() != 11 || decoded.GetShipList()[1].GetId() != 12 {
		t.Fatalf("unexpected ship ids in decoded payload")
	}
}

func TestPushNewShipsEmitsSC12042(t *testing.T) {
	client := &connection.Client{}
	ships := []*orm.OwnedShip{
		{ID: 101, ShipID: 1001, Level: 1, MaxLevel: 100, Energy: 150},
		{ID: 102, ShipID: 1002, Level: 1, MaxLevel: 100, Energy: 150},
	}

	pushed, err := pushNewShips(client, ships)
	if err != nil {
		t.Fatalf("push new ships: %v", err)
	}
	if !pushed {
		t.Fatalf("expected push to be sent")
	}

	var response protobuf.SC_12042
	offset := decodePacketAt(t, client, 0, 12042, &response)
	if len(response.GetShipList()) != 2 {
		t.Fatalf("expected 2 ships in push, got %d", len(response.GetShipList()))
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly one packet")
	}
}
