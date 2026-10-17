package game

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleChargeList answers GetChargeListCommand: nothing bought, so every
// first-purchase bonus is still available.
func (game *Game) handleChargeList(client *connection.Client, req *protobuf.CS_16104) error {
	return client.Send(&protobuf.SC_16105{})
}
