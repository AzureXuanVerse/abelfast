package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestEducateRequestOptionSuccessDeterministic(t *testing.T) {
	client := &connection.Client{}
	payload := protobuf.CS_27045{Type: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if _, _, err := EducateRequestOption(&buffer, client); err != nil {
		t.Fatalf("EducateRequestOption failed: %v", err)
	}
	var first protobuf.SC_27046
	decodePacketAt(t, client, 0, 27046, &first)

	client.Buffer.Reset()
	if _, _, err := EducateRequestOption(&buffer, client); err != nil {
		t.Fatalf("EducateRequestOption second call failed: %v", err)
	}
	var second protobuf.SC_27046
	decodePacketAt(t, client, 0, 27046, &second)

	if first.GetResult() != 0 || second.GetResult() != 0 {
		t.Fatalf("expected success result")
	}

	firstOpts, ok := findSiteOption(first.GetOpts(), 131)
	if !ok {
		t.Fatalf("expected site options in first response")
	}
	secondOpts, ok := findSiteOption(second.GetOpts(), 131)
	if !ok {
		t.Fatalf("expected site options in second response")
	}

	if len(firstOpts) != 2 || firstOpts[0] != 1314 || firstOpts[1] != 13142 {
		t.Fatalf("unexpected first option ids: %v", firstOpts)
	}
	if len(secondOpts) != 2 || secondOpts[0] != 1314 || secondOpts[1] != 13142 {
		t.Fatalf("unexpected second option ids: %v", secondOpts)
	}
}

func TestEducateRequestOptionDecodeFailure(t *testing.T) {
	client := &connection.Client{}
	buffer := []byte{0xff, 0x00}

	_, outID, err := EducateRequestOption(&buffer, client)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if outID != 27046 {
		t.Fatalf("expected outgoing packet id 27046, got %d", outID)
	}
}

func findSiteOption(options []*protobuf.CHILD_SITE_OPTION, siteID uint32) ([]uint32, bool) {
	for _, option := range options {
		if option.GetSiteId() == siteID {
			return option.GetOptionIds(), true
		}
	}
	return nil, false
}
