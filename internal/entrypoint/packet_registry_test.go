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
