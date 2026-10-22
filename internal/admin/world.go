package admin

import (
	"context"
	"net/http"

	"github.com/ggmolly/belfast/internal/game"
)

// registerWorld adds the back-office's Operation Siren routes.
func (server *Server) registerWorld(api apiFunc) {
	api("GET /api/commanders/{id}/world", server.handleWorldDetail)
	api("PUT /api/commanders/{id}/world/items/{item}", server.handleWorldItem)
	api("POST /api/commanders/{id}/world/counters/{counter}", server.handleWorldCounter)
	api("POST /api/commanders/{id}/world/counters/{counter}/reset", server.handleWorldCounterReset)
	api("POST /api/commanders/{id}/world/targets/{target}/achieve", server.handleWorldTarget(true))
	api("DELETE /api/commanders/{id}/world/targets/{target}", server.handleWorldTarget(false))
	api("POST /api/commanders/{id}/world/entrances/{entrance}/stars/{star}", server.handleWorldStar(true))
	api("DELETE /api/commanders/{id}/world/entrances/{entrance}/stars/{star}", server.handleWorldStar(false))
	api("POST /api/commanders/{id}/world/ports/{port}/restock", server.otherEdit("port", server.Game.RestockWorldPort))
	api("POST /api/commanders/{id}/world/ports/{port}/refresh", server.otherEdit("port", server.Game.RefreshWorldPort))
	api("POST /api/commanders/{id}/world/activate", server.handleWorldActivate)
}

// worldDetail is game.WorldDetail with every drop named.
type worldDetail struct {
	game.WorldDetail
	Activation *worldActivation `json:"activation,omitempty"`
}

type worldActivation struct {
	game.WorldActivationView
	Ships []worldActivationShip `json:"ships"`
}

type worldActivationShip struct {
	socialShip
	Team string `json:"team"`
}

// namedDrop fills a drop's name and icon from the config tables (world items
// come named).
func (server *Server) namedDrop(drop *game.WorldDropView) {
	if drop.Name != "" {
		return
	}
	row := server.names.shown(drop.Type, drop.ID)
	drop.Name, drop.Icon, drop.Rarity = row.Name, row.Icon, row.Rarity
}

func (server *Server) handleWorldDetail(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	detail, err := server.Game.WorldDetail(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	for i := range detail.Regions {
		for j := range detail.Regions[i].Entrances {
			for k := range detail.Regions[i].Entrances[j].Stars {
				server.namedDrop(&detail.Regions[i].Entrances[j].Stars[k].Drop)
			}
		}
	}
	for i := range detail.Ports {
		for j := range detail.Ports[i].Goods {
			server.namedDrop(&detail.Ports[i].Goods[j].Item)
			server.namedDrop(&detail.Ports[i].Goods[j].Price)
		}
	}
	view := worldDetail{WorldDetail: detail}
	if activation := detail.Activation; activation != nil {
		view.Activation = &worldActivation{WorldActivationView: *activation, Ships: []worldActivationShip{}}
		for _, ship := range activation.Ships {
			view.Activation.Ships = append(view.Activation.Ships, worldActivationShip{socialShip: server.socialShip(ship.AdminShip), Team: ship.Team})
		}
	}
	return view, nil
}

// handleWorldItem sets a world item's count: {count}, 0 removing it.
func (server *Server) handleWorldItem(r *http.Request) (any, error) {
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	type body struct {
		Count uint32 `json:"count"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetWorldItem(ctx, id, item, b.Count)
	})
}

// handleWorldCounter sets a counter: {value}.
func (server *Server) handleWorldCounter(r *http.Request) (any, error) {
	type body struct {
		Value uint32 `json:"value"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetWorldCounter(ctx, id, r.PathValue("counter"), b.Value)
	})
}

func (server *Server) handleWorldCounterReset(r *http.Request) (any, error) {
	return commanderEdit(server, r, func(ctx context.Context, id uint32, _ struct{}) (bool, error) {
		return server.Game.ResetWorldCounter(ctx, id, r.PathValue("counter"))
	})
}

// handleWorldStar marks a star of an entrance claimed or not.
func (server *Server) handleWorldStar(claimed bool) func(*http.Request) (any, error) {
	return func(r *http.Request) (any, error) {
		entrance, err := pathUint(r, "entrance")
		if err != nil {
			return nil, err
		}
		star, err := pathUint(r, "star")
		if err != nil {
			return nil, err
		}
		return commanderEdit(server, r, func(ctx context.Context, id uint32, _ struct{}) (bool, error) {
			return server.Game.SetWorldStar(ctx, id, entrance, star, claimed)
		})
	}
}

// handleWorldTarget marks a target achieved or not.
func (server *Server) handleWorldTarget(achieved bool) func(*http.Request) (any, error) {
	return server.otherEdit("target", func(ctx context.Context, id, target uint32) (bool, error) {
		return server.Game.SetWorldTarget(ctx, id, target, achieved)
	})
}

// handleWorldActivate activates the world: {fleets: [{ships, commanders}],
// submarine: {ships, commanders}}.
func (server *Server) handleWorldActivate(r *http.Request) (any, error) {
	type body struct {
		Fleets    []game.WorldFleetPick `json:"fleets"`
		Submarine *game.WorldFleetPick  `json:"submarine"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.ActivateWorld(ctx, id, b.Fleets, b.Submarine)
	})
}
