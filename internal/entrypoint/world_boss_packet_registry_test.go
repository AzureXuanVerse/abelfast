package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludesWorldBoss34xxxHandlers(t *testing.T) {
	previous := packets.PacketDecisionFn
	t.Cleanup(func() {
		packets.PacketDecisionFn = previous
	})

	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	registerPackets()

	expected := []int{34501, 34503, 34505, 34509, 34511, 34513, 34515, 34517, 34519, 34521, 34523, 34525, 34527}
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
