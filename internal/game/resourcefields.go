package game

import (
	"context"
	"encoding/json"
	"math"
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

// classField is a class_upgrade_template row, the lecture hall at a level
// (ClassResourceField): it turns proficiency_cost_per_min proficiency a
// minute into proficency_to_exp_rant percent of it in expField, up to stock
// of its item_id (an EXP Data Pack, usage_arg exp each). Battles give it
// proficency_get_percent of some ships' exp as proficiency, up to store
// (GetExp2ProficiencyRatio, GetMaxProficiency).
type classField struct {
	Item       uint32 `json:"item_id"`
	Stock      uint32 `json:"stock"`
	CostPerMin uint32 `json:"proficency_cost_per_min"`
	ExpRate    uint32 `json:"proficency_to_exp_rant"`
	GetPercent uint32 `json:"proficency_get_percent"`
	Store      uint32 `json:"store"`
}

// classBook is the item a classField makes: exp is its usage_arg
// (ClassResourceField:GetTarget), the expField one pack takes, and maxNum
// how many the bag holds (GetCanGetResCnt).
type classBook struct {
	exp, maxNum uint32
}

var (
	classFields map[uint32]classField
	classBooks  map[uint32]classBook
)

func loadResourceFields() error {
	var items map[uint32]struct {
		UsageArg json.RawMessage `json:"usage_arg"`
		MaxNum   uint32          `json:"max_num"`
	}
	err := gamedata.LoadAll(
		gamedata.Load("oilfield_template", &oilFields),
		gamedata.Load("tradingport_template", &goldFields),
		gamedata.Load("class_upgrade_template", &classFields),
		gamedata.Load("item_data_statistics", &items),
	)
	if err != nil {
		return err
	}
	classBooks = map[uint32]classBook{}
	for level, row := range classFields {
		if row.ExpRate == 0 {
			return gamedata.Missing("class_upgrade_template", "proficency_to_exp_rant of level %d", level)
		}
		exp := gamedata.IDs(items[row.Item].UsageArg)
		if len(exp) == 0 || exp[0] == 0 {
			return gamedata.Missing("item_data_statistics", "usage_arg of the lecture hall's item %d", row.Item)
		}
		classBooks[row.Item] = classBook{exp: exp[0], maxNum: items[row.Item].MaxNum}
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

// classTick is how often the lecture hall turns proficiency into exp: on
// the fields' 20-minute ticks, on its own clock from registration. Captured:
// the official server pushes SC_22013 (proficiency, exp_in_well) with the
// fields' SC_11004 at a tick.
const classTick = 20 * 60

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
	return tx.settleClassField(commander, owned[ResourceClassField], now)
}

// settleClassField turns the lecture hall's proficiency into expField for
// every tick since it was last filled, proficiency_cost_per_min a minute,
// while there is proficiency and up to stock packs of exp, and moves its
// fill time on by the ticks counted.
func (tx *Tx) settleClassField(commander db.Commander, exp uint32, now time.Time) error {
	config, ok := classFields[commander.ClassFieldLevel]
	if !ok {
		return gamedata.Missing("class_upgrade_template", "level %d", commander.ClassFieldLevel)
	}
	since := commander.ClassFieldAt
	if since == 0 {
		since = commander.RegisteredAt
	}
	ticks := (now.Unix() - since) / classTick
	if ticks <= 0 {
		return nil
	}
	capacity := uint64(config.Stock) * uint64(classBooks[config.Item].exp)
	room := capacity - min(uint64(exp), capacity)
	spend := min(uint64(ticks)*classTick/60*uint64(config.CostPerMin), uint64(commander.ClassProficiency))
	gain := spend * uint64(config.ExpRate) / 100
	if gain > room {
		gain = room
		spend = gain * 100 / uint64(config.ExpRate)
	}
	if gain > 0 {
		if err := tx.AddResource(ResourceClassField, uint32(gain)); err != nil {
			return err
		}
	}
	return tx.SetClassField(tx.ctx, db.SetClassFieldParams{
		ClassProficiency: commander.ClassProficiency - uint32(spend),
		ClassFieldAt:     since + ticks*classTick,
		ID:               tx.commanderID,
	})
}

// classExtraRate is AcademyCourse:getExtraRate: battles give the lecture
// hall twice the proficiency on the server week's 7th day (Sunday).
func classExtraRate(now time.Time) uint32 {
	if serverWeekday(now) == 7 {
		return 2
	}
	return 1
}

// addClassProficiency is NavalAcademyProxy:AddCourseProficiency for each
// ship exp of a battle that feeds the lecture hall (addShipsExp): it adds
// floor(exp x proficency_get_percent x extra rate x 0.01) to the hall's
// proficiency, up to the level's store, one exp after the other as the
// client does. The hall is settled first, and SC_22013 sends what it then
// holds ahead of the reply: the client adds the same exps to it itself, so
// both end on the same proficiency although belfast doesn't push the ticks.
func (tx *Tx) addClassProficiency(exps []uint32, now time.Time) error {
	if len(exps) == 0 {
		return nil
	}
	expInWell := func() (uint32, error) {
		resources, err := tx.ListResources(tx.ctx, tx.commanderID)
		if err != nil {
			return 0, err
		}
		for _, row := range resources {
			if row.ResourceID == ResourceClassField {
				return row.Amount, nil
			}
		}
		return 0, nil
	}
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	well, err := expInWell()
	if err != nil {
		return err
	}
	if err := tx.settleClassField(commander, well, now); err != nil {
		return err
	}
	if commander, err = tx.GetCommander(tx.ctx, tx.commanderID); err != nil {
		return err
	}
	if well, err = expInWell(); err != nil {
		return err
	}
	tx.push(&protobuf.SC_22013{
		Proficiency: proto.Uint32(commander.ClassProficiency),
		ExpInWell:   proto.Uint32(well),
	})
	config := classFields[commander.ClassFieldLevel]
	extra := classExtraRate(now)
	proficiency := commander.ClassProficiency
	for _, exp := range exps {
		// the client's float arithmetic, in its order
		gain := math.Floor(float64(exp) * float64(config.GetPercent) * float64(extra) * 0.01)
		proficiency = uint32(min(float64(proficiency)+gain, float64(config.Store)))
	}
	return tx.SetClassField(tx.ctx, db.SetClassFieldParams{
		ClassProficiency: proficiency,
		ClassFieldAt:     commander.ClassFieldAt,
		ID:               tx.commanderID,
	})
}

// fillResourceFields settles the commander's fields before SC_11003 and
// SC_22001 read them.
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

// handleHarvestClassResource answers HarvestClassResourceCommand
// (controller/command/player/harvestclassresourcecommand.lua): type 0 packs
// the lecture hall's expField into EXP Data Packs. The client only asks
// when it counts at least one (ClassResourceField:GetCanGetResCnt: whole
// packs of exp, no more than the bag still holds); on result 0 it adds
// that many packs and takes their exp from its expField itself, and sets
// the hall's proficiency to exp_in_well. Never captured: built from the
// Lua. The server settles the hall first and packs as many as it counts.
// Belfast doesn't push the ticks yet (SC_22013), so it corrects the
// client's proficiency and expField with SC_22013 after the reply; a pack
// made by a tick the client didn't see is in the bag at the next login.
func (game *Game) handleHarvestClassResource(client *connection.Client, req *protobuf.CS_22009) error {
	var update *protobuf.SC_22013
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if err := tx.settleResourceFields(commander, time.Now()); err != nil {
			return err
		}
		if commander, err = tx.GetCommander(tx.ctx, tx.commanderID); err != nil {
			return err
		}
		config := classFields[commander.ClassFieldLevel]
		book := classBooks[config.Item]
		resources, err := tx.ListResources(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		exp := uint32(0)
		for _, row := range resources {
			if row.ResourceID == ResourceClassField {
				exp = row.Amount
			}
		}
		items, err := tx.ListItems(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		owned := uint32(0)
		for _, row := range items {
			if row.ItemID == config.Item {
				owned = row.Count
			}
		}
		count := min(exp/book.exp, book.maxNum-min(owned, book.maxNum))
		if count == 0 {
			return errRefused
		}
		if err := tx.SpendResource(ResourceClassField, count*book.exp); err != nil {
			return err
		}
		if err := tx.AddItem(config.Item, count); err != nil {
			return err
		}
		update = &protobuf.SC_22013{
			Proficiency: proto.Uint32(commander.ClassProficiency),
			ExpInWell:   proto.Uint32(exp - count*book.exp),
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply := &protobuf.SC_22010{Result: result(err)}
	if update != nil {
		reply.ExpInWell = proto.Uint32(update.GetProficiency())
	}
	if err := client.Send(reply); err != nil || update == nil {
		return err
	}
	return client.PushAfterReply(update)
}
