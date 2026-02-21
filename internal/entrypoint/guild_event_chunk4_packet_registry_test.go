package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludesGuildEventChunk4Handlers(t *testing.T) {
	previous := packets.PacketDecisionFn
	t.Cleanup(func() {
		packets.PacketDecisionFn = previous
	})

	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	registerPackets()

	for _, packetID := range []int{61001, 61005, 61007, 61017, 61019, 61023, 61025, 61031, 61037} {
		handlers, ok := packets.PacketDecisionFn[packetID]
		if !ok {
			t.Fatalf("expected packet %d to be registered", packetID)
		}
		if len(handlers) == 0 {
			t.Fatalf("expected packet %d to have handlers", packetID)
		}
	}
}
