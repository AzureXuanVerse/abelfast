package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/ggmolly/belfast/internal/game"
)

// registerSocial registers the API of a commander's combat, backyard and
// friends.
func (server *Server) registerSocial(api apiFunc) {
	api("GET /api/commanders/{id}/combat", server.handleCombat)
	api("POST /api/commanders/{id}/exercise/score", server.handleExerciseScore)
	api("POST /api/commanders/{id}/exercise/refill", server.commanderAction(server.Game.RefillExercise))
	api("POST /api/commanders/{id}/exercise/rivals", server.commanderAction(server.Game.NewExerciseRivals))
	api("POST /api/commanders/{id}/world/reset", server.handleWorldReset)
	api("POST /api/commanders/{id}/world/stamina", server.handleWorldStamina)
	api("POST /api/commanders/{id}/exercise/defense", server.handleDefenseFleet)
	api("POST /api/commanders/{id}/meta/pt", server.handleMetaPt)

	api("GET /api/commanders/{id}/backyard", server.handleBackyard)
	api("POST /api/commanders/{id}/backyard/level", server.handleBackyardLevel)
	api("POST /api/commanders/{id}/backyard/food", server.handleBackyardFood)
	api("POST /api/commanders/{id}/backyard/fill", server.commanderAction(server.Game.FillDormFood))
	api("DELETE /api/commanders/{id}/backyard/ships/{other}", server.otherEdit("other", server.Game.RemoveDormShip))
	api("POST /api/commanders/{id}/backyard/floors/{other}/clear", server.otherEdit("other", server.Game.ClearDormFloor))

	api("GET /api/commanders/{id}/friends", server.handleFriends)
	api("DELETE /api/commanders/{id}/friends/{other}", server.otherEdit("other", server.Game.RemoveFriend))
	api("POST /api/commanders/{id}/requests/{other}/accept", server.otherEdit("other", server.Game.AcceptFriendRequest))
	api("DELETE /api/commanders/{id}/requests/{other}", server.otherEdit("other", server.Game.RejectFriendRequest))
	api("DELETE /api/commanders/{id}/blacklist/{other}", server.otherEdit("other", server.Game.Unblock))
}

// socialShip is a ship with its name, rarity and icon.
type socialShip struct {
	ID         uint32 `json:"id"`
	TemplateID uint32 `json:"template_id"`
	Name       string `json:"name"`
	Rarity     uint32 `json:"rarity"`
	Icon       string `json:"icon"`
	Level      uint32 `json:"level"`
}

func (server *Server) socialShip(ship game.AdminShip) socialShip {
	template := server.names[dropShip][ship.TemplateID]
	name := strings.TrimSpace(ship.Name)
	if name == "" {
		name = strings.TrimSpace(template.Name)
	}
	return socialShip{ID: ship.ID, TemplateID: ship.TemplateID, Name: name, Rarity: template.Rarity, Icon: server.names.shipIcon(ship.TemplateID, ship.SkinID), Level: ship.Level}
}

func (server *Server) socialShips(ships []game.AdminShip) []socialShip {
	list := make([]socialShip, len(ships))
	for i, ship := range ships {
		list[i] = server.socialShip(ship)
	}
	return list
}

// Combat

type metaGroup struct {
	game.MetaGroupView
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Rarity uint32 `json:"rarity"`
}

// exerciseRival is an opponent with the portrait of its flagship.
type exerciseRival struct {
	game.ExerciseRival
	Icon   string `json:"icon"`
	Rarity uint32 `json:"rarity"`
}

type combatView struct {
	Exercise struct {
		game.ExerciseView
		Vanguard []socialShip    `json:"vanguard"`
		Main     []socialShip    `json:"main"`
		Rivals   []exerciseRival `json:"rivals"`
	} `json:"exercise"`
	World game.WorldView `json:"world"`
	Meta  struct {
		game.MetaView
		CurrentName string      `json:"current_name"`
		BossName    string      `json:"boss_name,omitempty"`
		Groups      []metaGroup `json:"groups"`
	} `json:"meta"`
}

// handleCombat is the commander's exercises, Operation Siren and META
// Showdown.
func (server *Server) handleCombat(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	var view combatView
	if view.Exercise.ExerciseView, err = server.Game.Exercise(ctx, id); err != nil {
		return nil, err
	}
	view.Exercise.Vanguard = server.socialShips(view.Exercise.ExerciseView.Vanguard)
	view.Exercise.Main = server.socialShips(view.Exercise.ExerciseView.Main)
	view.Exercise.Rivals = []exerciseRival{}
	for _, rival := range view.Exercise.ExerciseView.Rivals {
		row, err := server.friendRow(r, rival.ID, rival.Name, rival.Level)
		if err != nil {
			return nil, err
		}
		view.Exercise.Rivals = append(view.Exercise.Rivals, exerciseRival{ExerciseRival: rival, Icon: row.Icon, Rarity: row.Rarity})
	}
	if view.World, err = server.Game.World(ctx, id); err != nil {
		return nil, err
	}
	if view.Meta.MetaView, err = server.Game.Meta(ctx, id); err != nil {
		return nil, err
	}
	view.Meta.CurrentName = server.metaName(view.Meta.CurrentShip)
	if boss := view.Meta.Boss; boss != nil {
		view.Meta.BossName = server.metaName(boss.MetaShip)
	}
	view.Meta.Groups = []metaGroup{}
	for _, group := range view.Meta.MetaView.Groups {
		named := server.names[dropShip][group.Group*10+1]
		view.Meta.Groups = append(view.Meta.Groups, metaGroup{MetaGroupView: group, Name: server.metaName(group.Group), Icon: named.Icon, Rarity: named.Rarity})
	}
	return view, nil
}

// metaName is a META ship group's name (ship_strengthen_meta ids are ship
// groups: its first template is group*10+1).
func (server *Server) metaName(group uint32) string {
	if group == 0 {
		return ""
	}
	return strings.TrimSpace(server.names.name(dropShip, group*10+1))
}

func (server *Server) handleExerciseScore(r *http.Request) (any, error) {
	type body struct {
		Score uint32 `json:"score"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetExerciseScore(ctx, id, b.Score)
	})
}

// handleDefenseFleet sets the exercise defense fleet: {ships}, vanguard and
// main mixed (meowfficers, which it has none of, ignored).
func (server *Server) handleDefenseFleet(r *http.Request) (any, error) {
	type body struct {
		Ships       []uint32 `json:"ships"`
		Meowfficers []uint32 `json:"meowfficers"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetDefenseFleet(ctx, id, b.Ships)
	})
}

// handleWorldStamina sets the world's action points: {stamina}.
func (server *Server) handleWorldStamina(r *http.Request) (any, error) {
	type body struct {
		Stamina uint32 `json:"stamina"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetWorldStamina(ctx, id, b.Stamina)
	})
}

// handleMetaPt sets a META ship group's Sync Points: {group, pt}.
func (server *Server) handleMetaPt(r *http.Request) (any, error) {
	type body struct {
		Group uint32 `json:"group"`
		Pt    uint32 `json:"pt"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetMetaPt(ctx, id, b.Group, b.Pt)
	})
}

func (server *Server) handleWorldReset(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	drops, was, err := server.Game.ResetWorld(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	granted := []drop{}
	for _, d := range drops {
		granted = append(granted, drop{Type: d.GetType(), ID: d.GetId(), Count: d.GetNumber(), Name: server.names.shown(d.GetType(), d.GetId()).Name})
	}
	return map[string]any{"kicked": was, "drops": granted}, nil
}

// Backyard

type backyardShip struct {
	socialShip
	Floor       uint32 `json:"floor"`
	PopIcon     uint32 `json:"pop_icon"`
	PopIntimacy uint32 `json:"pop_intimacy"`
}

type backyardView struct {
	game.DormView
	Ships  []backyardShip   `json:"ships"`
	Levels []game.DormLevel `json:"levels"`
}

func (server *Server) handleBackyard(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	dorm, err := server.Game.Dorm(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	view := backyardView{DormView: dorm, Ships: []backyardShip{}, Levels: game.DormLevels()}
	for _, ship := range dorm.Ships {
		view.Ships = append(view.Ships, backyardShip{socialShip: server.socialShip(ship.AdminShip), Floor: ship.Floor, PopIcon: ship.PopIcon, PopIntimacy: ship.PopIntimacy})
	}
	return view, nil
}

func (server *Server) handleBackyardLevel(r *http.Request) (any, error) {
	type body struct {
		Level uint32 `json:"level"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetDormLevel(ctx, id, b.Level)
	})
}

func (server *Server) handleBackyardFood(r *http.Request) (any, error) {
	type body struct {
		Food uint32 `json:"food"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.SetDormFood(ctx, id, b.Food)
	})
}

// Friends

type friendRow struct {
	ID         uint32 `json:"id"`
	Name       string `json:"name"`
	Level      uint32 `json:"level"`
	Icon       string `json:"icon"`
	Rarity     uint32 `json:"rarity"`
	Online     bool   `json:"online"`
	LastOnline int64  `json:"last_online,omitempty"`
	Since      int64  `json:"since,omitempty"`
	Content    string `json:"content,omitempty"`
	SentAt     int64  `json:"sent_at,omitempty"`
	BlockedAt  int64  `json:"blocked_at,omitempty"`
}

type friendsView struct {
	Friends   []friendRow `json:"friends"`
	Requests  []friendRow `json:"requests"`
	Blacklist []friendRow `json:"blacklist"`
	Max       int         `json:"max"`
}

// friendRow is a commander with the portrait its flagship gives it
// (displayInfo), the starter ship without one.
func (server *Server) friendRow(r *http.Request, id uint32, name string, level uint32) (friendRow, error) {
	row := friendRow{ID: id, Name: name, Level: level, Online: server.Game.IsOnline(id)}
	var template, skin uint32
	flagship, err := server.Store.GetFlagshipDisplay(r.Context(), id)
	switch {
	case err == nil:
		template, skin = flagship.TemplateID, flagship.SkinID
	case errors.Is(err, sql.ErrNoRows):
		commander, err := server.Store.GetCommander(r.Context(), id)
		if err != nil {
			return row, err
		}
		template = commander.StarterShip
	default:
		return row, err
	}
	row.Icon, row.Rarity = server.names.shipIcon(template, skin), server.names[dropShip][template].Rarity
	return row, nil
}

func (server *Server) handleFriends(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	view := friendsView{Friends: []friendRow{}, Requests: []friendRow{}, Blacklist: []friendRow{}, Max: game.MaxFriends}
	friends, err := server.Store.AdminSocialFriends(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, f := range friends {
		row, err := server.friendRow(r, f.ID, f.Name, f.Level)
		if err != nil {
			return nil, err
		}
		row.LastOnline, row.Since = f.LastOnline, f.Since
		view.Friends = append(view.Friends, row)
	}
	requests, err := server.Store.ListFriendRequests(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, q := range requests {
		row, err := server.friendRow(r, q.Commander.ID, q.Commander.Name, q.Commander.Level)
		if err != nil {
			return nil, err
		}
		row.Content, row.SentAt = q.Content, q.SentAt
		view.Requests = append(view.Requests, row)
	}
	blocked, err := server.Store.AdminSocialBlacklist(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, b := range blocked {
		row, err := server.friendRow(r, b.ID, b.Name, b.Level)
		if err != nil {
			return nil, err
		}
		row.BlockedAt = b.BlockedAt
		view.Blacklist = append(view.Blacklist, row)
	}
	return view, nil
}
