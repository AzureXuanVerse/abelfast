package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestTechnologyPacketHandlersAreRegistered(t *testing.T) {
	original := packets.PacketDecisionFn
	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	t.Cleanup(func() {
		packets.PacketDecisionFn = original
	})

	registerPackets()
	packetIDs := []int{63001, 63003, 63005, 63007, 63009, 63011, 63013, 63015}
	for _, packetID := range packetIDs {
		handlers := packets.PacketDecisionFn[packetID]
		if len(handlers) == 0 {
			t.Fatalf("packet %d has no registered handlers", packetID)
		}
	}
}
