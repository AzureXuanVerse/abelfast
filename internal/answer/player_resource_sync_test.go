package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestSendPlayerResourceSyncBuildsSC11004Snapshot(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	if err := client.Commander.SetResource(1, 321); err != nil {
		t.Fatalf("seed gold: %v", err)
	}
	if err := client.Commander.SetResource(2, 654); err != nil {
		t.Fatalf("seed oil: %v", err)
	}

	if _, _, err := SendPlayerResourceSync(client); err != nil {
		t.Fatalf("send player resource sync: %v", err)
	}

	var response protobuf.SC_11004
	decodePacketAt(t, client, 0, 11004, &response)
	if len(response.GetResourceList()) != len(client.Commander.OwnedResources) {
		t.Fatalf("expected full snapshot length %d, got %d", len(client.Commander.OwnedResources), len(response.GetResourceList()))
	}
	if findResourceAmount(response.GetResourceList(), 1) != 321 {
		t.Fatalf("expected gold amount 321")
	}
	if findResourceAmount(response.GetResourceList(), 2) != 654 {
		t.Fatalf("expected oil amount 654")
	}
}

func TestSendPlayerResourceSyncMissingCommander(t *testing.T) {
	client := &connection.Client{Commander: nil}
	_, _, err := SendPlayerResourceSync(client)
	if err == nil {
		t.Fatalf("expected missing commander error")
	}
	if client.Buffer.Len() != 0 {
		t.Fatalf("expected no packet output")
	}
}

func TestBuildResourceSnapshot(t *testing.T) {
	resources := []orm.OwnedResource{{ResourceID: 4, Amount: 100}, {ResourceID: 2, Amount: 55}}
	snapshot := buildResourceSnapshot(resources)
	if len(snapshot) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(snapshot))
	}
	if snapshot[0].GetType() != 4 || snapshot[0].GetNum() != 100 {
		t.Fatalf("unexpected first snapshot entry")
	}
	if snapshot[1].GetType() != 2 || snapshot[1].GetNum() != 55 {
		t.Fatalf("unexpected second snapshot entry")
	}
}

func TestPlayerInfoKeepsResourceListOnLogin(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	pos := uint32(0)
	if err := client.Commander.SetResource(1, 777); err != nil {
		t.Fatalf("seed gold: %v", err)
	}
	client.Commander.Ships = []orm.OwnedShip{{
		OwnerID:           client.Commander.CommanderID,
		ShipID:            202124,
		IsSecretary:       true,
		SecretaryPosition: &pos,
	}}

	buffer := []byte{}
	if _, _, err := PlayerInfo(&buffer, client); err != nil {
		t.Fatalf("player info: %v", err)
	}
	var response protobuf.SC_11003
	decodePacketAt(t, client, 0, 11003, &response)
	if findResourceAmount(response.GetResourceList(), 1) != 777 {
		t.Fatalf("expected login resource list to include gold=777")
	}
}
