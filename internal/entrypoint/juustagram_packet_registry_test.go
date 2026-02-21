package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsJuustagramRangeRegisteredWithoutSC11700Inbound(t *testing.T) {
	previous := packets.PacketDecisionFn
	t.Cleanup(func() {
		packets.PacketDecisionFn = previous
	})

	packets.PacketDecisionFn = map[int][]packets.PacketHandler{}
	registerPackets()

	rangeHandlers, ok := packets.PacketDecisionFn[11705]
	if !ok || len(rangeHandlers) == 0 {
		t.Fatalf("expected packet 11705 to be registered")
	}
	if _, ok := packets.PacketDecisionFn[11700]; ok {
		t.Fatalf("did not expect packet 11700 to be registered as inbound")
	}
}
