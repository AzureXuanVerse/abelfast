package answer

import (
	"errors"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestFleetRenameSuccessEmitsSC12106(t *testing.T) {
	originalRename := fleetRenameApply
	t.Cleanup(func() {
		fleetRenameApply = originalRename
	})
	fleetRenameApply = func(fleet *orm.Fleet, name string) error {
		fleet.Name = name
		return nil
	}

	fleet := &orm.Fleet{ID: 1, GameID: 1, Name: "Before", ShipList: orm.Int64List{101, 102}}
	client := &connection.Client{Commander: &orm.Commander{FleetsMap: map[uint32]*orm.Fleet{1: fleet}}}
	payload := protobuf.CS_12104{Id: proto.Uint32(1), Name: proto.String("After")}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := FleetRename(&buffer, client)
	if err != nil {
		t.Fatalf("fleet rename failed: %v", err)
	}
	if packetID != 12105 {
		t.Fatalf("expected packet 12105, got %d", packetID)
	}

	var ack protobuf.SC_12105
	offset := decodePacketAt(t, client, 0, 12105, &ack)
	if ack.GetResult() != 0 {
		t.Fatalf("expected success ack, got %d", ack.GetResult())
	}

	var push protobuf.SC_12106
	offset = decodePacketAt(t, client, offset, 12106, &push)
	if push.GetGroup().GetName() != "After" {
		t.Fatalf("expected pushed fleet name After, got %q", push.GetGroup().GetName())
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly ack + push packets")
	}
}

func TestFleetRenameFailureHasNoSC12106(t *testing.T) {
	originalRename := fleetRenameApply
	t.Cleanup(func() {
		fleetRenameApply = originalRename
	})
	fleetRenameApply = func(_ *orm.Fleet, _ string) error {
		return errors.New("rename failed")
	}

	fleet := &orm.Fleet{ID: 1, GameID: 1, Name: "Before", ShipList: orm.Int64List{101, 102}}
	client := &connection.Client{Commander: &orm.Commander{FleetsMap: map[uint32]*orm.Fleet{1: fleet}}}
	payload := protobuf.CS_12104{Id: proto.Uint32(1), Name: proto.String("After")}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, packetID, err := FleetRename(&buffer, client)
	if err != nil {
		t.Fatalf("fleet rename failed: %v", err)
	}
	if packetID != 12105 {
		t.Fatalf("expected packet 12105, got %d", packetID)
	}

	var ack protobuf.SC_12105
	offset := decodePacketAt(t, client, 0, 12105, &ack)
	if ack.GetResult() == 0 {
		t.Fatalf("expected non-zero ack result")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected no fleet sync push on failure")
	}
}

func TestFleetGroupInfoParityBetween12101And12106(t *testing.T) {
	fleet := orm.Fleet{ID: 1, GameID: 2, Name: "Parity", ShipList: orm.Int64List{301, 302}}
	commander := &orm.Commander{Fleets: []orm.Fleet{fleet}, FleetsMap: map[uint32]*orm.Fleet{2: &fleet}}
	client := &connection.Client{Commander: commander}

	empty := []byte{}
	if _, _, err := CommanderFleet(&empty, client); err != nil {
		t.Fatalf("commander fleet failed: %v", err)
	}

	var full protobuf.SC_12101
	offset := decodePacketAt(t, client, 0, 12101, &full)
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected single 12101 packet")
	}
	if len(full.GetGroupList()) != 1 {
		t.Fatalf("expected one fleet group in 12101")
	}

	client.Buffer.Reset()
	if err := pushFleetSync(client, &fleet); err != nil {
		t.Fatalf("push fleet sync failed: %v", err)
	}

	var push protobuf.SC_12106
	offset = decodePacketAt(t, client, 0, 12106, &push)
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected single 12106 packet")
	}

	base := full.GetGroupList()[0]
	group := push.GetGroup()
	if group.GetId() != base.GetId() || group.GetName() != base.GetName() {
		t.Fatalf("expected matching group id/name between 12101 and 12106")
	}
	if len(group.GetShipList()) != len(base.GetShipList()) {
		t.Fatalf("expected matching ship list lengths")
	}
}
