package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's production, technology and wild resources: see island_core.go.

func (game *Game) registerIslandProduction() {
}

func loadIslandProductionData() error {
	return nil
}

// islandMapGathers adds to an EnterMapCommand reply (SC_21214) the wild
// resources and collection fragments of scene mapID on island state.
func islandMapGathers(tx *Tx, state *islandState, mapID uint32, reply *protobuf.SC_21214) error {
	return nil
}

// islandNewBuilding is a production site (island_production_place) as it is
// when unlocked.
func islandNewBuilding(placeID uint32) *protobuf.PB_ISLAND_BUILD {
	return &protobuf.PB_ISLAND_BUILD{Id: proto.Uint32(placeID)}
}

// islandAbilityGained runs when the island gains an ability
// (island_ability_template), from any reward: what the server does besides
// listing it (a production site unlocked: SC_21519...).
func (tx *Tx) islandAbilityGained(state *islandState, id uint32) error {
	return nil
}
