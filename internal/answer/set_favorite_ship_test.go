package answer

import (
	"errors"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestSetFavoriteShipSuccessNoSC12042(t *testing.T) {
	original := setFavoriteShipPreference
	t.Cleanup(func() {
		setFavoriteShipPreference = original
	})
	setFavoriteShipPreference = func(ship *orm.OwnedShip, flag uint32) error {
		ship.CommonFlag = flag != 0
		return nil
	}

	ship := &orm.OwnedShip{ID: 10}
	client := &connection.Client{Commander: &orm.Commander{OwnedShipsMap: map[uint32]*orm.OwnedShip{10: ship}}}
	payload := protobuf.CS_12040{ShipId: proto.Uint32(10), Flag: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := SetFavoriteShip(&buffer, client)
	if err != nil {
		t.Fatalf("set favorite ship: %v", err)
	}
	if packetID != 12041 {
		t.Fatalf("expected packet 12041, got %d", packetID)
	}

	var response protobuf.SC_12041
	offset := decodePacketAt(t, client, 0, 12041, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected no sc_12042 push for 12040 flow")
	}
}

func TestSetFavoriteShipFailure(t *testing.T) {
	original := setFavoriteShipPreference
	t.Cleanup(func() {
		setFavoriteShipPreference = original
	})
	setFavoriteShipPreference = func(_ *orm.OwnedShip, _ uint32) error {
		return errors.New("write failed")
	}

	ship := &orm.OwnedShip{ID: 10}
	client := &connection.Client{Commander: &orm.Commander{OwnedShipsMap: map[uint32]*orm.OwnedShip{10: ship}}}
	payload := protobuf.CS_12040{ShipId: proto.Uint32(10), Flag: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := SetFavoriteShip(&buffer, client)
	if err != nil {
		t.Fatalf("set favorite ship: %v", err)
	}
	if packetID != 12041 {
		t.Fatalf("expected packet 12041, got %d", packetID)
	}

	var response protobuf.SC_12041
	decodePacketAt(t, client, 0, 12041, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure result")
	}
}
