package admin

import (
	"context"
	"net/http"

	"github.com/ggmolly/belfast/internal/game"
)

// registerMeta adds the back-office's META Showdown routes.
func (server *Server) registerMeta(api apiFunc) {
	api("GET /api/commanders/{id}/meta", server.handleMeta)
	api("POST /api/commanders/{id}/meta/award", server.handleMetaAward)
	api("POST /api/commanders/{id}/meta/summon-pt", server.handleMetaSummonPt)
	api("POST /api/commanders/{id}/meta/boss", server.handleMetaSummon)
	api("POST /api/commanders/{id}/meta/boss/hp", server.handleMetaBossHP)
	api("POST /api/commanders/{id}/meta/boss/fights/reset", server.commanderAction(server.Game.ResetMetaBossFights))
	api("POST /api/commanders/{id}/meta/boss/timer", server.handleMetaBossTimer)
	api("POST /api/commanders/{id}/meta/boss/support", server.handleMetaBossSupport)
}

// metaShip is a META ship with its name and picture.
type metaShip struct {
	game.MetaShipView
	Name   string     `json:"name"`
	Icon   string     `json:"icon"`
	Rarity uint32     `json:"rarity"`
	Steps  []metaStep `json:"steps"`
}

// metaStep is a Sync Point reward step with its award named.
type metaStep struct {
	Target  uint32 `json:"target"`
	Claimed bool   `json:"claimed"`
	Award   reward `json:"award"`
}

// metaBossChoice is a summonable boss with its META ship's name and picture.
type metaBossChoice struct {
	game.MetaBossChoice
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Rarity uint32 `json:"rarity"`
}

type metaView struct {
	game.MetaAdminView
	BossName string           `json:"boss_name,omitempty"`
	Bosses   []metaBossChoice `json:"bosses"`
	Ships    []metaShip       `json:"ships"`
}

// handleMeta is the commander's META Showdown: coordinates, boss, every META
// ship's Sync Points and rewards.
func (server *Server) handleMeta(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	data, err := server.Game.MetaAdmin(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	view := metaView{MetaAdminView: data, Bosses: []metaBossChoice{}, Ships: []metaShip{}}
	if boss := data.Boss; boss != nil {
		view.BossName = server.metaName(boss.MetaShip)
	}
	for _, choice := range data.Bosses {
		named := server.names[dropShip][choice.Group*10+1]
		view.Bosses = append(view.Bosses, metaBossChoice{MetaBossChoice: choice, Name: server.metaName(choice.Group), Icon: named.Icon, Rarity: named.Rarity})
	}
	for _, ship := range data.Ships {
		named := server.names[dropShip][ship.Group*10+1]
		row := metaShip{MetaShipView: ship, Name: server.metaName(ship.Group), Icon: named.Icon, Rarity: named.Rarity, Steps: make([]metaStep, len(ship.Steps))}
		for i, step := range ship.Steps {
			row.Steps[i] = metaStep{Target: step.Target, Claimed: step.Claimed, Award: server.namedDrops([][3]uint32{step.Award})[0]}
		}
		view.Ships = append(view.Ships, row)
	}
	return view, nil
}

// handleMetaAward marks a Sync Point reward taken or not: {group, target,
// claimed}.
func (server *Server) handleMetaAward(r *http.Request) (any, error) {
	type body struct {
		Group   uint32 `json:"group"`
		Target  uint32 `json:"target"`
		Claimed bool   `json:"claimed"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetMetaAward(ctx, id, b.Group, b.Target, b.Claimed)
	})
}

// handleMetaSummonPt sets the META Coordinates or, with past, the Dossier
// Coordinate Data: {value, past}.
func (server *Server) handleMetaSummonPt(r *http.Request) (any, error) {
	type body struct {
		Value uint32 `json:"value"`
		Past  bool   `json:"past"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetMetaSummonPt(ctx, id, b.Value, b.Past)
	})
}

// handleMetaSummon summons a boss: {template, level}.
func (server *Server) handleMetaSummon(r *http.Request) (any, error) {
	type body struct {
		Template uint32 `json:"template"`
		Level    uint32 `json:"level"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SummonMetaBoss(ctx, id, b.Template, b.Level)
	})
}

// handleMetaBossHP sets the boss's hp, 0 to kill it: {hp}.
func (server *Server) handleMetaBossHP(r *http.Request) (any, error) {
	type body struct {
		HP uint32 `json:"hp"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetMetaBossHP(ctx, id, b.HP)
	})
}

// handleMetaBossTimer restarts the boss's timer, or extends it: {extend}.
func (server *Server) handleMetaBossTimer(r *http.Request) (any, error) {
	type body struct {
		Extend bool `json:"extend"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetMetaBossTimer(ctx, id, b.Extend)
	})
}

// handleMetaBossSupport turns a support request on or off: {type, on}
// (1 friends, 2 guild, 3 world).
func (server *Server) handleMetaBossSupport(r *http.Request) (any, error) {
	type body struct {
		Type uint32 `json:"type"`
		On   bool   `json:"on"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetMetaBossSupport(ctx, id, b.Type, b.On)
	})
}
