package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludesIslandTradeInvitationHandler(t *testing.T) {
	previous := packets.PacketDecisionFn
	t.Cleanup(func() {
		packets.PacketDecisionFn = previous
	})

	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	registerPackets()

	handlers, ok := packets.PacketDecisionFn[21245]
	if !ok {
		t.Fatalf("expected packet 21245 to be registered")
	}
	if len(handlers) == 0 {
		t.Fatalf("expected packet 21245 to have handlers")
	}
}
