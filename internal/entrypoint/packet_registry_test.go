package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludesActivityBossHandlers(t *testing.T) {
	original := packets.PacketDecisionFn
	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	t.Cleanup(func() {
		packets.PacketDecisionFn = original
	})

	registerPackets()

	if len(packets.PacketDecisionFn[26031]) == 0 {
		t.Fatalf("expected handler registration for 26031")
	}
	if len(packets.PacketDecisionFn[26081]) == 0 {
		t.Fatalf("expected handler registration for 26081")
	}
	for _, packetID := range []int{27004, 27006, 27008, 27010, 27019, 27023, 27031, 27037, 27039, 27041} {
		if len(packets.PacketDecisionFn[packetID]) == 0 {
			t.Fatalf("expected handler registration for %d", packetID)
		}
	}
	if len(packets.PacketDecisionFn[27002]) == 0 {
		t.Fatalf("expected handler registration for 27002")
	}
	if len(packets.PacketDecisionFn[27012]) == 0 {
		t.Fatalf("expected handler registration for 27012")
	}
	if len(packets.PacketDecisionFn[27029]) == 0 {
		t.Fatalf("expected handler registration for 27029")
	}
	if len(packets.PacketDecisionFn[27045]) == 0 {
		t.Fatalf("expected handler registration for 27045")
	}
	if len(packets.PacketDecisionFn[27047]) == 0 {
		t.Fatalf("expected handler registration for 27047")
	}
	if len(packets.PacketDecisionFn[28001]) == 0 {
		t.Fatalf("expected handler registration for 28001")
	}
	if len(packets.PacketDecisionFn[28007]) == 0 {
		t.Fatalf("expected handler registration for 28007")
	}
	if len(packets.PacketDecisionFn[28017]) == 0 {
		t.Fatalf("expected handler registration for 28017")
	}
	if len(packets.PacketDecisionFn[28019]) == 0 {
		t.Fatalf("expected handler registration for 28019")
	}
}
