package game

import (
	"context"
	"maps"
	"slices"

	"github.com/ggmolly/belfast/internal/db"
)

// fleetTechShip is a fleet_tech_ship_template row: the Fleet Technology
// points a ship group gives once owned (pt_get), at its max limit break
// (pt_upgrage, star >= max_star) and at level 120 (pt_level).
type fleetTechShip struct {
	MaxStar   uint32 `json:"max_star"`
	PtGet     uint32 `json:"pt_get"`
	PtLevel   uint32 `json:"pt_level"`
	PtUpgrade uint32 `json:"pt_upgrage"`
}

// fleet_tech_ship_template, by ship group
var fleetTechShips map[uint32]fleetTechShip

// TechnologyConst.SHIP_LEVEL_FOR_BUFF, the level that gives pt_level
const techShipLevel = 120

// techPoints are the commander's Fleet Technology points by nation, as
// TechnologyNationProxy.nationPointFilter counts them from CollectionProxy's
// ship groups: for each group in fleet_tech_ship_template, pt_get, plus
// pt_level when its maxLV reached 120, plus pt_upgrage when its star reached
// max_star, under the nationality of its base template. A group's star and
// maxLV are its SC_17001 star / lv_max, raised by every ship of the group the
// client sees (CollectionProxy.flushCollection), hence the owned ships here.
func techPoints(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32]uint32, error) {
	groups, err := queries.ListShipGroups(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	ships, err := queries.ListShipsByCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	type best struct{ star, level uint32 }
	owned := make(map[uint32]best, len(groups))
	for _, group := range groups {
		owned[group.GroupID] = best{star: shipTemplates[group.GroupID*10+1].Star, level: group.LvMax}
	}
	for _, ship := range ships {
		group, ok := owned[ship.TemplateID/10]
		if !ok {
			continue
		}
		if template, ok := shipTemplates[ship.TemplateID]; ok {
			group.star = max(group.star, template.Star)
		}
		group.level = max(group.level, ship.Level)
		owned[ship.TemplateID/10] = group
	}
	points := map[uint32]uint32{}
	for id, group := range owned {
		tech, ok := fleetTechShips[id]
		if !ok {
			continue
		}
		value := tech.PtGet
		if group.level >= techShipLevel {
			value += tech.PtLevel
		}
		if group.star >= tech.MaxStar {
			value += tech.PtUpgrade
		}
		points[shipTemplates[id*10+1].Nationality] += value
	}
	return points, nil
}

// techPointEvents are the TaskSubTechPoints events of the commander's
// points: one per nation (target_id a nation), then the total (target_id 0,
// TechnologyNationProxy.getNationPoint(0)) last, the value a task of any
// target takes. Points only grow (the client's ship groups keep their best
// star and level), so a stored value higher than the count (a retired ship
// that had reached 120) is kept.
func techPointEvents(ctx context.Context, queries *db.Queries, commanderID uint32) ([]TaskEvent, error) {
	points, err := techPoints(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	counters, err := listTaskCounters(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	var events []TaskEvent
	var total uint32
	for _, nation := range slices.Sorted(maps.Keys(points)) {
		value := max(points[nation], counters[taskCounterKey{TaskSubTechPoints, nation}])
		total += value
		events = append(events, TaskEvent{SubType: TaskSubTechPoints, Target: nation, Count: value})
	}
	total = max(total, counters[taskCounterKey{TaskSubTechPoints, 0}])
	return append(events, TaskEvent{SubType: TaskSubTechPoints, Count: total}), nil
}

// techPointsChanged tells tx the commander's ship groups changed (a new
// group, a ship reaching level 120 or its max limit break): their Fleet
// Technology points are counted again before its task events are applied.
func (tx *Tx) techPointsChanged() {
	tx.techPoints = true
}

// recordTechPoints records the commander's Fleet Technology points on tx
// when techPointsChanged was called.
func (tx *Tx) recordTechPoints() error {
	if !tx.techPoints {
		return nil
	}
	events, err := techPointEvents(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	tx.event(events...)
	return nil
}

// syncTechPoints brings the commander's TaskSubTechPoints counters and tasks
// to their current points at login, before SC_20001 lists them: the client
// computes that progress itself (Task.getProgress) and claims the task as
// soon as it is reached, whatever the server last sent.
func (game *Game) syncTechPoints(ctx context.Context, commanderID uint32) error {
	return game.DB.InTx(ctx, func(queries *db.Queries) error {
		events, err := techPointEvents(ctx, queries, commanderID)
		if err != nil {
			return err
		}
		// the login burst carries the progress, no push
		_, err = game.applyTaskEvents(ctx, queries, commanderID, events)
		return err
	})
}
