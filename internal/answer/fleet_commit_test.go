package answer

import (
	"errors"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestFleetCommitSuccessEmitsSC12106(t *testing.T) {
	originalUpdate := fleetCommitUpdateShipList
	t.Cleanup(func() {
		fleetCommitUpdateShipList = originalUpdate
	})
	fleetCommitUpdateShipList = func(fleet *orm.Fleet, _ *orm.Commander, shipList []uint32) error {
		fleet.ShipList = make(orm.Int64List, len(shipList))
		for i, id := range shipList {
			fleet.ShipList[i] = int64(id)
		}
		return nil
	}

	fleet := orm.Fleet{ID: 1, GameID: 1, Name: "Main", ShipList: orm.Int64List{101, 102}}
	commander := &orm.Commander{Fleets: []orm.Fleet{fleet}, FleetsMap: map[uint32]*orm.Fleet{1: &fleet}}
	client := &connection.Client{Commander: commander}
	payload := protobuf.CS_12102{Id: proto.Uint32(1), ShipList: []uint32{201, 202, 203}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := FleetCommit(&buffer, client)
	if err != nil {
		t.Fatalf("fleet commit failed: %v", err)
	}
	if packetID != 12103 {
		t.Fatalf("expected packet 12103, got %d", packetID)
	}

	var ack protobuf.SC_12103
	offset := decodePacketAt(t, client, 0, 12103, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected success ack, got %d", ack.GetResult())
	}

	var push protobuf.SC_12106
	offset = decodePacketAt(t, client, offset, 12106, &push)
	if push.GetGroup().GetId() != 1 {
		t.Fatalf("expected group id 1, got %d", push.GetGroup().GetId())
	}
	if len(push.GetGroup().GetShipList()) != 3 {
		t.Fatalf("expected 3 ships in push, got %d", len(push.GetGroup().GetShipList()))
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly ack + push packets")
	}
}

func TestFleetCommitFailureHasNoSC12106(t *testing.T) {
	originalCreate := fleetCommitCreateFleet
	t.Cleanup(func() {
		fleetCommitCreateFleet = originalCreate
	})
	fleetCommitCreateFleet = func(_ *orm.Commander, _ uint32, _ string, _ []uint32) error {
		return errors.New("create failed")
	}

	client := &connection.Client{Commander: &orm.Commander{FleetsMap: map[uint32]*orm.Fleet{}}}
	payload := protobuf.CS_12102{Id: proto.Uint32(99), ShipList: []uint32{201}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := FleetCommit(&buffer, client)
	if err != nil {
		t.Fatalf("fleet commit failed: %v", err)
	}
	if packetID != 12103 {
		t.Fatalf("expected packet 12103, got %d", packetID)
	}

	var ack protobuf.SC_12103
	offset := decodePacketAt(t, client, 0, 12103, &ack)
	if ack.GetResult() == 0 {
		t.Fatalf("expected non-zero ack result")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected no fleet sync push on failure")
	}
}
