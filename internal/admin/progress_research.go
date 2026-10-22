package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
)

type progResearchProject struct {
	Pos     uint32 `json:"pos"`
	Pool    uint32 `json:"pool"`
	ID      uint32 `json:"id"` // the queue entry's
	Tech    uint32 `json:"tech"`
	Name    string `json:"name"`
	Sub     string `json:"sub"`
	Version uint32 `json:"version"`
	Rarity  uint32 `json:"rarity"`
	Time    int64  `json:"time"`
	Finish  int64  `json:"finish"`
}

type progCatchupRow struct {
	Version uint32 `json:"version"`
	Group   uint32 `json:"group"` // 0: the priority ships' shared count
	Name    string `json:"name"`
	Number  uint32 `json:"number"`
	Max     uint32 `json:"max"`
}

type progBlueprintShip struct {
	ID        uint32             `json:"id"`
	Name      string             `json:"name"`
	Rarity    uint32             `json:"rarity"`
	Icon      string             `json:"icon,omitempty"`
	Version   uint32             `json:"version"`
	State     string             `json:"state"` // locked, stopped, running, done
	ShipID    uint32             `json:"ship_id"`
	ShipLevel uint32             `json:"ship_level"`
	Start     int64              `json:"start"`
	Dev       *game.BlueprintDev `json:"dev"`
}

type progFleetTechGroupRow struct {
	game.FleetTechGroup
	Name string `json:"name"`
}

type progFleetTechAttrRow struct {
	game.FleetTechAttr
	TypeName string `json:"type_name"`
}

type progResearchView struct {
	Now            int64                   `json:"now"`
	RefreshedAt    int64                   `json:"refreshed_at"`
	Offers         []progResearchProject   `json:"offers"`
	Queue          []progResearchProject   `json:"queue"`
	CatchupVersion uint32                  `json:"catchup_version"`
	CatchupTarget  string                  `json:"catchup_target"`
	Catchups       []progCatchupRow        `json:"catchups"`
	Blueprints     []progBlueprintShip     `json:"blueprints"`
	BlueprintCold  int64                   `json:"blueprint_cold"`
	Pursued        uint32                  `json:"pursued"`
	PursuedUR      uint32                  `json:"pursued_ur"`
	FleetTech      []progFleetTechGroupRow `json:"fleet_tech"`
	FleetTechAttrs []progFleetTechAttrRow  `json:"fleet_tech_attrs"`
}

// progShipGroupName is the name of a ship group (ship_data_group id), its
// first template's.
func (server *Server) progShipGroupName(group uint32) string {
	return server.names[dropShip][group*10+1].Name
}

// handleResearch is the commander's Research Academy projects, research
// ships and Fleet Technology.
func (server *Server) handleResearch(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadProgressTables()
	if err != nil {
		return nil, err
	}
	ctx, id, now := r.Context(), commander.ID, time.Now()
	view := progResearchView{Now: now.Unix(), Offers: []progResearchProject{}, Queue: []progResearchProject{}, Catchups: []progCatchupRow{}, Blueprints: []progBlueprintShip{}, FleetTech: []progFleetTechGroupRow{}, FleetTechAttrs: []progFleetTechAttrRow{}}
	project := func(tech uint32) progResearchProject {
		config := tables.projects[tech]
		return progResearchProject{Tech: tech, Name: config.Name, Sub: config.SubName, Version: config.Version, Rarity: config.Rarity, Time: config.Time}
	}

	state, err := server.Store.GetTechnologyState(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	view.RefreshedAt, view.CatchupVersion = state.RefreshedAt, state.CatchupVersion
	if state.CatchupTarget != 0 {
		view.CatchupTarget = server.progShipGroupName(state.CatchupTarget)
	}
	offers, err := server.Store.ListTechnologyOffers(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, offer := range offers {
		row := project(offer.TechID)
		row.Pos, row.Pool, row.Finish = offer.Pos, offer.PoolID, offer.FinishAt
		view.Offers = append(view.Offers, row)
	}
	queue, err := server.Store.ListTechnologyQueue(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, entry := range queue {
		row := project(entry.TechID)
		row.ID, row.Finish = entry.ID, entry.FinishAt
		view.Queue = append(view.Queue, row)
	}
	catchups, err := server.Store.ListTechnologyCatchups(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, catchup := range catchups {
		config := tables.catchups[catchup.Version]
		row := progCatchupRow{Version: catchup.Version, Group: catchup.ShipGroup, Number: catchup.Number, Max: config.ObtainMax}
		if catchup.ShipGroup != 0 {
			row.Name, row.Max = server.progShipGroupName(catchup.ShipGroup), config.PerUR
		}
		view.Catchups = append(view.Catchups, row)
	}

	blueprints, err := server.Store.ListBlueprints(ctx, id)
	if err != nil {
		return nil, err
	}
	rows := make(map[uint32]db.CommanderBlueprint, len(blueprints))
	for _, row := range blueprints {
		rows[row.BlueprintID] = row
	}
	for blueprintID, config := range tables.blueprints {
		template := server.names[dropShip][blueprintID*10+1]
		ship := progBlueprintShip{ID: blueprintID, Name: template.Name, Rarity: template.Rarity, Icon: template.Icon, Version: config.Version, State: "locked"}
		if row, ok := rows[blueprintID]; ok {
			ship.ShipID, ship.Start = row.ShipID, row.StartTime
			switch {
			case row.ShipID != 0:
				ship.State = "done"
			case row.StartTime != 0:
				ship.State = "running"
			case row.Duration != 0:
				ship.State = "stopped"
			}
		}
		if ship.ShipID != 0 {
			built, err := server.Store.GetShip(ctx, db.GetShipParams{ID: ship.ShipID, CommanderID: id})
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			ship.ShipLevel = built.Level
			dev, ok, err := game.BlueprintProgress(ctx, server.Store.Queries, blueprintID, ship.ShipID)
			if err != nil {
				return nil, err
			}
			if ok {
				ship.Dev = &dev
			}
		}
		view.Blueprints = append(view.Blueprints, ship)
	}
	sort.Slice(view.Blueprints, func(i, j int) bool {
		a, b := view.Blueprints[i], view.Blueprints[j]
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		return a.ID < b.ID
	})
	cold, err := server.Store.GetBlueprintState(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	view.BlueprintCold, view.Pursued, view.PursuedUR = cold.ColdTime, cold.Pursued, cold.PursuedUr

	groups, attrs, err := server.Game.FleetTech(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		view.FleetTech = append(view.FleetTech, progFleetTechGroupRow{FleetTechGroup: group, Name: tables.techGroups[group.ID].Name})
	}
	sort.Slice(view.FleetTech, func(i, j int) bool { return view.FleetTech[i].ID < view.FleetTech[j].ID })
	for _, attr := range attrs {
		view.FleetTechAttrs = append(view.FleetTechAttrs, progFleetTechAttrRow{FleetTechAttr: attr, TypeName: tables.shipTypes[attr.ShipType].TypeName})
	}
	sort.Slice(view.FleetTechAttrs, func(i, j int) bool {
		a, b := view.FleetTechAttrs[i], view.FleetTechAttrs[j]
		if a.ShipType != b.ShipType {
			return a.ShipType < b.ShipType
		}
		return a.Attr < b.Attr
	})
	return view, nil
}

// handleDevelopBlueprint develops a built research ship up to a level:
// {blueprint, level}, fate simulation phases counted past the max level.
func (server *Server) handleDevelopBlueprint(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Blueprint uint32 `json:"blueprint"`
		Level     uint32 `json:"level"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	return kicked(server.Game.DevelopBlueprint(r.Context(), commander.ID, body.Blueprint, body.Level))
}

// handleUnlockBlueprint builds a research ship without its research:
// {blueprint}.
func (server *Server) handleUnlockBlueprint(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Blueprint uint32 `json:"blueprint"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	return kicked(server.Game.UnlockBlueprint(r.Context(), commander.ID, body.Blueprint))
}

// handleFleetTechLevel sets a faction's Fleet Technology level: {group, level}.
func (server *Server) handleFleetTechLevel(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Group uint32 `json:"group"`
		Level int    `json:"level"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	return kicked(server.Game.SetFleetTechLevel(r.Context(), commander.ID, body.Group, body.Level))
}

// handleFleetTechReset puts a ship type's Fleet Technology bonuses back to
// their max: {ship_type}.
func (server *Server) handleFleetTechReset(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		ShipType uint32 `json:"ship_type"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	return kicked(server.Game.ResetFleetTechAttrs(r.Context(), commander.ID, body.ShipType))
}
