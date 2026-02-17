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

func TestRegisterPacketsIncludesIslandDelegationAwardClaim(t *testing.T) {
	packets.PacketDecisionFn = make(map[int][]packets.PacketHandler)
	registerPackets()
	if _, ok := packets.PacketDecisionFn[21505]; !ok {
		t.Fatalf("expected handler for CS_21505 to be registered")
	}
}

func TestRegisterPacketsIncludesIslandAgoraThemeDelete(t *testing.T) {
	packets.PacketDecisionFn = make(map[int][]packets.PacketHandler)
	registerPackets()
	if _, ok := packets.PacketDecisionFn[21319]; !ok {
		t.Fatalf("expected handler for CS_21319 to be registered")
	}
}

func TestRegisterPacketsIncludesIslandOpsBatch(t *testing.T) {
	packets.PacketDecisionFn = make(map[int][]packets.PacketHandler)
	registerPackets()
	required := []int{21006, 21416, 21423, 21425, 21427, 21501, 21503}
	for _, packetID := range required {
		if _, ok := packets.PacketDecisionFn[packetID]; !ok {
			t.Fatalf("expected handler for packet %d to be registered", packetID)
		}
	}
}
