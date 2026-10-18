package game

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// resourceField is an oilfield_template / tradingport_template row: every
// hour_time-th of an hour the field produces production, up to store
// (BaseResourceField:getHourProduct).
type resourceField struct {
	HourTime   uint32 `json:"hour_time"`
	Production uint32 `json:"production"`
	Store      uint32 `json:"store"`
}

var oilFields, goldFields map[uint32]resourceField

func loadResourceFields() error {
	err := gamedata.LoadAll(
		gamedata.Load("oilfield_template", &oilFields),
		gamedata.Load("tradingport_template", &goldFields),
	)
	if err != nil {
		return err
	}
	for name, table := range map[string]map[uint32]resourceField{"oilfield_template": oilFields, "tradingport_template": goldFields} {
		for level, row := range table {
			if row.HourTime == 0 {
				return gamedata.Missing(name, "hour_time of level %d", level)
			}
		}
	}
	return nil
}

// tick is how often the field produces: 20 minutes at hour_time 3, on a
// clock from registration that a harvest doesn't restart, up to store and
// no further while full. Captured: a new account's oil field went from 5 to
// 25 and its merchant from 5 to 65 at register_time + 20 min (SC_11004);
// 25 hours later, never harvested, they held 300 and 900; the merchant
// harvested at :33:14 held 60 at the :38:54 tick.
func (field resourceField) tick() int64 {
	return int64(3600 / field.HourTime)
}

// commanderField is one of the commander's two resource fields: the
// player_resource it fills, its config at the commander's level and when
// it was last filled (0 never).
type commanderField struct {
	resource uint32
	config   resourceField
	filledAt int64
	setAt    func(tx *Tx, at int64) error
}

func commanderFields(commander db.Commander) []commanderField {
	return []commanderField{
		{ResourceOilField, oilFields[commander.OilFieldLevel], commander.OilFieldAt, func(tx *Tx, at int64) error {
			return tx.SetOilFieldAt(tx.ctx, db.SetOilFieldAtParams{OilFieldAt: at, ID: tx.commanderID})
		}},
		{ResourceGoldField, goldFields[commander.GoldFieldLevel], commander.GoldFieldAt, func(tx *Tx, at int64) error {
			return tx.SetGoldFieldAt(tx.ctx, db.SetGoldFieldAtParams{GoldFieldAt: at, ID: tx.commanderID})
		}},
	}
}

// settleResourceFields adds what the oil field and the merchant produced
// since they were last filled, each on its own clock from registration, up
// to their store, and moves their fill time on by the ticks counted.
func (tx *Tx) settleResourceFields(commander db.Commander, now time.Time) error {
	resources, err := tx.ListResources(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	owned := map[uint32]uint32{}
	for _, row := range resources {
		owned[row.ResourceID] = row.Amount
	}
	for _, field := range commanderFields(commander) {
		if field.config.HourTime == 0 {
			return gamedata.Missing("resource field", "level of %d", field.resource)
		}
		since := field.filledAt
		if since == 0 {
			since = commander.RegisteredAt
		}
		tick := field.config.tick()
		ticks := (now.Unix() - since) / tick
		if ticks <= 0 {
			continue
		}
		room := field.config.Store - min(owned[field.resource], field.config.Store)
		if add := min(uint64(ticks)*uint64(field.config.Production), uint64(room)); add > 0 {
			if err := tx.AddResource(field.resource, uint32(add)); err != nil {
				return err
			}
		}
		if err := field.setAt(tx, since+ticks*tick); err != nil {
			return err
		}
	}
	return nil
}

// fillResourceFields settles the commander's fields before SC_11003 reads
// them.
func (game *Game) fillResourceFields(ctx context.Context, commander db.Commander, now time.Time) error {
	return game.DB.InTx(ctx, func(queries *db.Queries) error {
		return newTx(ctx, queries, commander.ID).settleResourceFields(commander, now)
	})
}

// handleHarvestResource answers HarvestResourceCommand
// (controller/command/player/harvestresourcecommand.lua): type 1 collects
// the merchant's gold, 2 the oil field's oil, up to the level's max_gold /
// max_oil. On result 0 the client moves its own count of the field over,
// as much as fits, from the player it read before asking; the server
// settles the field first and moves as much. The official server replies
// only that (captured): it pushes SC_11004 at every tick, so the client's
// count is current. Belfast doesn't push the ticks yet, so it corrects the
// client with SC_11004 after the reply (one sent before would be
// overwritten).
func (game *Game) handleHarvestResource(client *connection.Client, req *protobuf.CS_11013) error {
	var resources []*protobuf.RESOURCE
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		var resource, field, limit uint32
		switch req.GetType() {
		case ResourceGold:
			resource, field, limit = ResourceGold, ResourceGoldField, commanderLevels[commander.Level].MaxGold
		case ResourceOil:
			resource, field, limit = ResourceOil, ResourceOilField, commanderLevels[commander.Level].MaxOil
		default:
			return errRefused
		}
		if err := tx.settleResourceFields(commander, time.Now()); err != nil {
			return err
		}
		rows, err := tx.ListResources(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		owned := map[uint32]uint32{}
		for _, row := range rows {
			owned[row.ResourceID] = row.Amount
		}
		if owned[resource] >= limit {
			return errRefused
		}
		take := min(owned[field], limit-owned[resource])
		if err := tx.SpendResource(field, take); err != nil {
			return err
		}
		if err := tx.AddResource(resource, take); err != nil {
			return err
		}
		owned[field] -= take
		owned[resource] += take
		for _, id := range []uint32{ResourceGold, ResourceOil, ResourceOilField, ResourceGoldField} {
			resources = append(resources, &protobuf.RESOURCE{Type: proto.Uint32(id), Num: proto.Uint32(owned[id])})
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err := client.Send(&protobuf.SC_11014{Result: result(err)}); err != nil || resources == nil {
		return err
	}
	return client.PushAfterReply(&protobuf.SC_11004{ResourceList: resources})
}
