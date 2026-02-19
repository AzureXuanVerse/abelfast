package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludesFriendBlacklistHandlers(t *testing.T) {
	previous := packets.PacketDecisionFn
	t.Cleanup(func() {
		packets.PacketDecisionFn = previous
	})

	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	registerPackets()

	expected := []int{50016, 50107, 50109}
	for _, packetID := range expected {
		handlers, ok := packets.PacketDecisionFn[packetID]
		if !ok {
			t.Fatalf("expected packet %d to be registered", packetID)
		}
		if len(handlers) == 0 {
			t.Fatalf("expected packet %d to have handlers", packetID)
		}
	}
}
