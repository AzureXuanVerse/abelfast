package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestColoringClearRemovesPageCells(t *testing.T) {
	client := setupColoringTestClient(t)
	state, err := orm.GetOrCreateCommanderColoringState(client.Commander.CommanderID, 4890, 1700000001)
	if err != nil {
		t.Fatalf("seed state: %v", err)
	}
	state.Cells = []orm.ColoringCellState{{PageID: 93, Row: 1, Column: 1, Color: 1}, {PageID: 93, Row: 1, Column: 2, Color: 2}}
	if err := orm.SaveCommanderColoringState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26006{ActId: proto.Uint32(4890), Id: proto.Uint32(93)})
	if _, _, err := ColoringClear(&payload, client); err != nil {
		t.Fatalf("ColoringClear failed: %v", err)
	}
	response := &protobuf.SC_26007{}
	decodeLoveLetterPacketMessage(t, client, 26007, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected clear success, got %d", response.GetResult())
	}
	state, err = orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(coloringGetPageFills(state, 93)) != 0 {
		t.Fatalf("expected page fills to be removed")
	}
}

func TestColoringClearAlreadyEmptyIsIdempotent(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26006{ActId: proto.Uint32(4890), Id: proto.Uint32(93)})
	if _, _, err := ColoringClear(&payload, client); err != nil {
		t.Fatalf("ColoringClear failed: %v", err)
	}
	response := &protobuf.SC_26007{}
	decodeLoveLetterPacketMessage(t, client, 26007, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected idempotent clear success")
	}
}

func TestColoringClearInvalidActivityFails(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26006{ActId: proto.Uint32(9999), Id: proto.Uint32(93)})
	if _, _, err := ColoringClear(&payload, client); err != nil {
		t.Fatalf("ColoringClear invalid activity returned error: %v", err)
	}
	response := &protobuf.SC_26007{}
	decodeLoveLetterPacketMessage(t, client, 26007, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid activity failure")
	}
}

func TestColoringClearRejectsNonCustomPage(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26006{ActId: proto.Uint32(4890), Id: proto.Uint32(92)})
	if _, _, err := ColoringClear(&payload, client); err != nil {
		t.Fatalf("ColoringClear non-custom returned error: %v", err)
	}
	response := &protobuf.SC_26007{}
	decodeLoveLetterPacketMessage(t, client, 26007, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-custom page clear failure")
	}
}
