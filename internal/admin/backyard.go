package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/ggmolly/belfast/internal/game"
)

// registerBackyard adds the back-office's Backyard routes.
func (server *Server) registerBackyard(api apiFunc) {
	api("GET /api/commanders/{id}/backyard/detail", server.handleBackyardDetail)
	api("GET /api/catalog/furniture", server.handleFurnitureCatalog)
	api("POST /api/commanders/{id}/backyard/name", server.handleBackyardName)
	api("POST /api/commanders/{id}/backyard/floors", server.backyardNumber("floors", server.Game.SetDormFloors))
	api("POST /api/commanders/{id}/backyard/slots", server.backyardNumber("slots", server.Game.SetDormSlots))
	api("POST /api/commanders/{id}/backyard/food-extensions", server.backyardNumber("count", server.Game.SetDormFoodExtensions))
	api("POST /api/commanders/{id}/backyard/settle", server.commanderAction(server.Game.SettleDorm))
	api("POST /api/commanders/{id}/backyard/collect", server.commanderAction(func(ctx context.Context, id uint32) (bool, error) { return server.Game.CollectDorm(ctx, id, 0) }))
	api("POST /api/commanders/{id}/backyard/ships", server.handleBackyardAddShip)
	api("POST /api/commanders/{id}/backyard/ships/{other}/floor", server.handleBackyardMoveShip)
	api("POST /api/commanders/{id}/backyard/ships/{other}/pop", server.handleBackyardShipPop)
	api("POST /api/commanders/{id}/backyard/ships/{other}/collect", server.otherEdit("other", server.Game.CollectDorm))
	api("POST /api/commanders/{id}/backyard/ships/{other}/exp", server.handleBackyardShipExp)
	api("PUT /api/commanders/{id}/backyard/furniture/{other}", server.handleBackyardFurniture)
	api("POST /api/commanders/{id}/backyard/floors/{other}/starter", server.otherEdit("other", server.Game.ResetDormFloor))
	api("POST /api/commanders/{id}/backyard/themes/{other}/share", server.handleBackyardThemeShare)
	api("DELETE /api/commanders/{id}/backyard/themes/{other}", server.otherEdit("other", server.Game.DeleteDormTheme))
}

type backyardDetailShip struct {
	game.BackyardShip
	socialShip
}

type backyardCandidate struct {
	socialShip
	Busy string `json:"busy,omitempty"`
}

type backyardFurniture struct {
	game.BackyardFurniture
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity"`
}

type backyardDetail struct {
	game.BackyardAdmin
	Ships      []backyardDetailShip `json:"ships"`
	Candidates []backyardCandidate  `json:"candidates"`
	Furniture  []backyardFurniture  `json:"furniture_list"`
	Levels     []game.DormLevel     `json:"levels"`
}

// handleBackyardDetail is the commander's backyard with the bounds of its
// edits, the ships named, the furniture owned and placed.
func (server *Server) handleBackyardDetail(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	data, err := server.Game.BackyardAdmin(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	view := backyardDetail{BackyardAdmin: data, Ships: []backyardDetailShip{}, Candidates: []backyardCandidate{}, Furniture: []backyardFurniture{}, Levels: game.DormLevels()}
	for _, ship := range data.Ships {
		view.Ships = append(view.Ships, backyardDetailShip{BackyardShip: ship, socialShip: server.socialShip(ship.AdminShip)})
	}
	for _, ship := range data.Candidates {
		view.Candidates = append(view.Candidates, backyardCandidate{socialShip: server.socialShip(ship.AdminShip), Busy: ship.Busy})
	}
	for _, row := range data.Furniture {
		named := server.names[dropFurniture][row.ID]
		view.Furniture = append(view.Furniture, backyardFurniture{BackyardFurniture: row, Name: strings.TrimSpace(named.Name), Icon: named.Icon, Rarity: named.Rarity})
	}
	return view, nil
}

type furnitureCatalogEntry struct {
	game.BackyardFurnitureEntry
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity"`
}

// handleFurnitureCatalog is every furniture, with how many one may own.
func (server *Server) handleFurnitureCatalog(r *http.Request) (any, error) {
	list := []furnitureCatalogEntry{}
	for _, entry := range game.BackyardFurnitureCatalog() {
		named := server.names[dropFurniture][entry.ID]
		list = append(list, furnitureCatalogEntry{BackyardFurnitureEntry: entry, Name: strings.TrimSpace(named.Name), Icon: named.Icon, Rarity: named.Rarity})
	}
	return list, nil
}

// backyardNumber is an edit of the {id} commander's backyard taking one
// number, body {field: n}.
func (server *Server) backyardNumber(field string, edit func(ctx context.Context, id, value uint32) (bool, error)) func(*http.Request) (any, error) {
	return func(r *http.Request) (any, error) {
		return commanderEdit(server, r, func(ctx context.Context, id uint32, body map[string]uint32) (bool, error) {
			value, ok := body[field]
			if !ok || len(body) != 1 {
				return false, badRequest("body: {\"" + field + "\": n}")
			}
			return edit(ctx, id, value)
		})
	}
}

func (server *Server) handleBackyardName(r *http.Request) (any, error) {
	type body struct {
		Name string `json:"name"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetDormName(ctx, id, b.Name)
	})
}

func (server *Server) handleBackyardAddShip(r *http.Request) (any, error) {
	type body struct {
		Ship  uint32 `json:"ship"`
		Floor uint32 `json:"floor"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.AddDormShip(ctx, id, b.Ship, b.Floor)
	})
}

func (server *Server) handleBackyardMoveShip(r *http.Request) (any, error) {
	ship, err := pathUint(r, "other")
	if err != nil {
		return nil, err
	}
	type body struct {
		Floor uint32 `json:"floor"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.MoveDormShip(ctx, id, ship, b.Floor)
	})
}

func (server *Server) handleBackyardShipPop(r *http.Request) (any, error) {
	ship, err := pathUint(r, "other")
	if err != nil {
		return nil, err
	}
	type body struct {
		Coins    uint32 `json:"coins"`
		Intimacy uint32 `json:"intimacy"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetDormShipPop(ctx, id, ship, b.Coins, b.Intimacy)
	})
}

// handleBackyardShipExp sets a backyard ship's exp within its level, as the
// dock's ship edit does.
func (server *Server) handleBackyardShipExp(r *http.Request) (any, error) {
	ship, err := pathUint(r, "other")
	if err != nil {
		return nil, err
	}
	type body struct {
		Exp uint32 `json:"exp"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.EditShip(ctx, id, ship, game.ShipEdit{Exp: &b.Exp})
	})
}

func (server *Server) handleBackyardFurniture(r *http.Request) (any, error) {
	furniture, err := pathUint(r, "other")
	if err != nil {
		return nil, err
	}
	type body struct {
		Count uint32 `json:"count"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetDormFurniture(ctx, id, furniture, b.Count)
	})
}

func (server *Server) handleBackyardThemeShare(r *http.Request) (any, error) {
	pos, err := pathUint(r, "other")
	if err != nil {
		return nil, err
	}
	type body struct {
		Shared bool `json:"shared"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.ShareDormTheme(ctx, id, pos, b.Shared)
	})
}
