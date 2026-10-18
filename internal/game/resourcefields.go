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

// resourceFieldLevel is the oil field's and the merchant's level, the
// SC_22001 oil_well_level / gold_well_level belfast sends: they can't be
// upgraded yet.
const resourceFieldLevel = 1

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
		if table[resourceFieldLevel].HourTime == 0 {
			return gamedata.Missing(name, "level %d", resourceFieldLevel)
		}
	}
	return nil
}

// resourceFieldTick is how often the fields produce: 20 minutes at
// hour_time 3, as captured (a new account's oil field went from 5 to 25 and
// its merchant from 5 to 65 in the first 20 minutes, SC_11004).
func resourceFieldTick() int64 {
	return int64(3600 / oilFields[resourceFieldLevel].HourTime)
}

// settleResourceFields adds what the oil field and the merchant produced
// since they were last filled, up to their store, and moves the fill time
// on by the ticks counted.
func (tx *Tx) settleResourceFields(commander db.Commander, now time.Time) error {
	since := commander.ResourceFieldAt
	if since == 0 {
		since = commander.RegisteredAt
	}
	tick := resourceFieldTick()
	ticks := (now.Unix() - since) / tick
	if ticks <= 0 {
		return nil
	}
	resources, err := tx.ListResources(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	owned := map[uint32]uint32{}
	for _, row := range resources {
		owned[row.ResourceID] = row.Amount
	}
	for id, field := range map[uint32]resourceField{ResourceOilField: oilFields[resourceFieldLevel], ResourceGoldField: goldFields[resourceFieldLevel]} {
		room := field.Store - min(owned[id], field.Store)
		if add := min(uint64(ticks)*uint64(field.Production), uint64(room)); add > 0 {
			if err := tx.AddResource(id, uint32(add)); err != nil {
				return err
			}
		}
	}
	return tx.SetResourceFieldAt(tx.ctx, db.SetResourceFieldAtParams{ResourceFieldAt: since + ticks*tick, ID: tx.commanderID})
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
// settles the field first, moves as much and corrects the client with
// SC_11004 after the reply (one sent before would be overwritten). No
// official capture has a harvest: the reply is built from the Lua alone.
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
