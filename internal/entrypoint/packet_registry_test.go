package entrypoint

import (
	"testing"

	"github.com/ggmolly/belfast/internal/packets"
)

func TestRegisterPacketsIncludes14004(t *testing.T) {
	packets.PacketDecisionFn = make(map[int][]packets.PacketHandler)
	registerPackets()
	if _, ok := packets.PacketDecisionFn[14004]; !ok {
		t.Fatalf("expected handler for CS_14004 to be registered")
	}
}

func TestRegisterPacketsIncludesLoveLetterGetAll(t *testing.T) {
	packets.PacketDecisionFn = make(map[int][]packets.PacketHandler)
	registerPackets()
	if _, ok := packets.PacketDecisionFn[12406]; !ok {
		t.Fatalf("expected handler for CS_12406 to be registered")
	}
}

func TestRegisterPacketsIncludesTaskCluster20009To20017(t *testing.T) {
	packets.PacketDecisionFn = make(map[int][]packets.PacketHandler)
	registerPackets()
	for _, packetID := range []int{20005, 20009, 20011, 20013, 20016} {
		if _, ok := packets.PacketDecisionFn[packetID]; !ok {
			t.Fatalf("expected handler for packet %d to be registered", packetID)
		}
	}
}
