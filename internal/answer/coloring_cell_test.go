package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestColoringCellValidBatchPersistsCells(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26004{
		ActId: proto.Uint32(4890),
		Id:    proto.Uint32(92),
		CellList: []*protobuf.CELLSINFO{
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(1)},
			{Row: proto.Uint32(1), Column: proto.Uint32(2), Color: proto.Uint32(2)},
		},
	})
	if _, _, err := ColoringCell(&payload, client); err != nil {
		t.Fatalf("ColoringCell failed: %v", err)
	}
	response := &protobuf.SC_26005{}
	decodeLoveLetterPacketMessage(t, client, 26005, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success, got %d", response.GetResult())
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(state.Cells) != 2 {
		t.Fatalf("expected 2 persisted cells, got %+v", state.Cells)
	}
}

func TestColoringCellInsufficientPaintFailsWithoutWrites(t *testing.T) {
	client := setupColoringTestClient(t)
	if err := client.Commander.SetItem(3001, 0); err != nil {
		t.Fatalf("set paint count: %v", err)
	}
	payload := marshalPacketRequest(t, &protobuf.CS_26004{
		ActId: proto.Uint32(4890),
		Id:    proto.Uint32(92),
		CellList: []*protobuf.CELLSINFO{
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(1)},
		},
	})
	if _, _, err := ColoringCell(&payload, client); err != nil {
		t.Fatalf("ColoringCell insufficient paint returned error: %v", err)
	}
	response := &protobuf.SC_26005{}
	decodeLoveLetterPacketMessage(t, client, 26005, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected insufficient paint failure")
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(state.Cells) != 0 {
		t.Fatalf("expected no cells persisted on failure, got %+v", state.Cells)
	}
}

func TestColoringCellRejectsInvalidCoordinatesOrColor(t *testing.T) {
	client := setupColoringTestClient(t)
	invalidCoordinate := marshalPacketRequest(t, &protobuf.CS_26004{
		ActId: proto.Uint32(4890),
		Id:    proto.Uint32(92),
		CellList: []*protobuf.CELLSINFO{
			{Row: proto.Uint32(99), Column: proto.Uint32(99), Color: proto.Uint32(1)},
		},
	})
	if _, _, err := ColoringCell(&invalidCoordinate, client); err != nil {
		t.Fatalf("invalid coordinate returned error: %v", err)
	}
	response := &protobuf.SC_26005{}
	decodeLoveLetterPacketMessage(t, client, 26005, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid coordinate failure")
	}

	invalidColor := marshalPacketRequest(t, &protobuf.CS_26004{
		ActId: proto.Uint32(4890),
		Id:    proto.Uint32(92),
		CellList: []*protobuf.CELLSINFO{
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(9)},
		},
	})
	if _, _, err := ColoringCell(&invalidColor, client); err != nil {
		t.Fatalf("invalid color returned error: %v", err)
	}
	response = &protobuf.SC_26005{}
	decodeLoveLetterPacketMessage(t, client, 26005, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid color failure")
	}
}

func TestColoringCellDuplicateEntriesUseLastWrite(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26004{
		ActId: proto.Uint32(4890),
		Id:    proto.Uint32(93),
		CellList: []*protobuf.CELLSINFO{
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(1)},
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(2)},
		},
	})
	if _, _, err := ColoringCell(&payload, client); err != nil {
		t.Fatalf("ColoringCell duplicate write failed: %v", err)
	}
	response := &protobuf.SC_26005{}
	decodeLoveLetterPacketMessage(t, client, 26005, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success for duplicate writes")
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	fills := coloringGetPageFills(state, 93)
	cell := fills[coloringCellKey(1, 1)]
	if cell.Color != 2 {
		t.Fatalf("expected last write color=2, got %+v", cell)
	}
}

func TestColoringCellDuplicateCoordinatesChargeOnce(t *testing.T) {
	client := setupColoringTestClient(t)
	if err := client.Commander.SetItem(3001, 1); err != nil {
		t.Fatalf("set paint count: %v", err)
	}
	payload := marshalPacketRequest(t, &protobuf.CS_26004{
		ActId: proto.Uint32(4890),
		Id:    proto.Uint32(92),
		CellList: []*protobuf.CELLSINFO{
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(1)},
			{Row: proto.Uint32(1), Column: proto.Uint32(1), Color: proto.Uint32(1)},
		},
	})
	if _, _, err := ColoringCell(&payload, client); err != nil {
		t.Fatalf("ColoringCell duplicate non-blank failed: %v", err)
	}
	response := &protobuf.SC_26005{}
	decodeLoveLetterPacketMessage(t, client, 26005, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected duplicate coordinates to consume once and succeed")
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	fills := coloringGetPageFills(state, 92)
	if len(fills) != 1 {
		t.Fatalf("expected one persisted fill entry, got %d", len(fills))
	}
}
