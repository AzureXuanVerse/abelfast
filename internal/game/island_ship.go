package game

import "github.com/ggmolly/belfast/internal/protobuf"

// The island's characters, outfits and NPC actions: see island_core.go.

func (game *Game) registerIslandShips() {
}

func loadIslandShipData() error {
	return nil
}

// islandMapNpcs adds to an EnterMapCommand reply (SC_21214) the event
// characters about in scene mapID on island state.
func islandMapNpcs(tx *Tx, state *islandState, mapID uint32, reply *protobuf.SC_21214) error {
	return nil
}
