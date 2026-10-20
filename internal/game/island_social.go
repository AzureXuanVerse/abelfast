package game

import "github.com/ggmolly/belfast/internal/connection"

// The island's visits, plaza, sign-in, card, chat and trade: see island_core.go.

func (game *Game) registerIslandSocial() {
}

func loadIslandSocialData() error {
	return nil
}

// islandVisitAllowed is the EnterIslandCommand result (CS_21202) for client
// visiting island owner: 0 lets it in.
func (game *Game) islandVisitAllowed(tx *Tx, client *connection.Client, owner *islandState) (uint32, error) {
	return resultFailed, nil
}
