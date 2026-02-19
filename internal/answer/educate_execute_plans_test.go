package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestEducateExecutePlansSuccess(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27002{Type: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateExecutePlans(&buffer, client); err != nil {
		t.Fatalf("EducateExecutePlans failed: %v", err)
	}

	var response protobuf.SC_27003
	decodePacketAt(t, client, 0, 27003, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetPlanResults()) != 0 {
		t.Fatalf("expected empty plan results")
	}
	if len(response.GetEvents()) != 0 {
		t.Fatalf("expected empty events")
	}
}

func TestEducateExecutePlansUnsupportedType(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27002{Type: proto.Uint32(2)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateExecutePlans(&buffer, client); err != nil {
		t.Fatalf("EducateExecutePlans failed: %v", err)
	}

	var response protobuf.SC_27003
	decodePacketAt(t, client, 0, 27003, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for unsupported type")
	}
}

func TestEducateExecutePlansDecodeFailure(t *testing.T) {
	client := &connection.Client{}
	buffer := []byte{0xff, 0x00}

	_, outID, err := EducateExecutePlans(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 27003 {
		t.Fatalf("expected outgoing packet id 27003, got %d", outID)
	}
}
