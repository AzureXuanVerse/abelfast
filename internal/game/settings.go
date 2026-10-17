package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleSecondaryPassword answers FetchSecondaryPasswordCommand, which keeps
// a loading overlay up until it gets a reply: no secondary password set.
func (game *Game) handleSecondaryPassword(client *connection.Client, req *protobuf.CS_11603) error {
	return client.Send(&protobuf.SC_11604{State: proto.Uint32(0)})
}
