package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludesIslandTradeHandlers(t *testing.T) {
	previous := packets.PacketDecisionFn
	t.Cleanup(func() {
		packets.PacketDecisionFn = previous
	})

	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	registerPackets()

	for _, packetID := range []int{21240, 21243} {
		handlers, ok := packets.PacketDecisionFn[packetID]
		if !ok {
			t.Fatalf("expected packet %d to be registered", packetID)
		}
		if len(handlers) == 0 {
			t.Fatalf("expected packet %d to include handlers", packetID)
		}
	}
}
