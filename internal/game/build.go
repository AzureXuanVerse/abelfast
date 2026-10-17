package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// buildWorkCount is how many builds can run at once, the client's
// MAX_BUILD_WORK_COUNT.
const buildWorkCount = 10

// buildDocks is SC_12024, BuildShipProxy's state: nothing building, no
// draws or medal exchanges yet.
func buildDocks() *protobuf.SC_12024 {
	return &protobuf.SC_12024{
		WorklistCount: proto.Uint32(buildWorkCount),
		DrawCount_1:   proto.Uint32(0),
		DrawCount_10:  proto.Uint32(0),
		ExchangeCount: proto.Uint32(0),
	}
}
