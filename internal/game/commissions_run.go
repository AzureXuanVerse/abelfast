package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// TaskSubCommission is task_data_template sub_type 80: commissions completed
// ("Complete 1 commissions.").
const TaskSubCommission = 80

// collection_template types of the daily commissions (the wiki's
// Commissions page, "Daily Commissions"): the "Daily Resource Extraction"
// ones (type 4, and from level 50 the "Awakening Tactical Research" ones of
// type 6, listed with them) and the extra ones (type 1) drawn once a day's
// first commissionDailyPerDay were.
const (
	commissionTypeExtra     = 1
	commissionTypeDaily     = 4
	commissionTypeAwakening = 6
)

// commissionDailyTypes are the daily-slot commissions: drawn each server day
// and on completing one, rolled over into the next day's morning.
var commissionDailyTypes = []uint32{commissionTypeExtra, commissionTypeDaily, commissionTypeAwakening}

const (
	// commissionDailyCount is how many daily commissions a day brings: the
	// capture's login on the next day offered 4 new ones (30102, 30106,
	// 30109, 30110, type 4).
	commissionDailyCount = 4
	// commissionDailyPerDay is how many of a day's daily-slot commissions are
	// Daily Resource Extraction ones, the extra ones (type 1) coming after
	// (the wiki's Commissions page: "The first 10 daily commissions
	// generated in a day will be of this type").
	commissionDailyPerDay = 10
	// commissionNightCount is how many night commissions the night window
	// brings (the wiki's Commissions page: "four of these commissions will
	// automatically appear in the Urgent tab").
	commissionNightCount = 4
	// commissionDailyGrace is how long into the new day the previous day's
	// daily commissions stay offered: the capture gave them an over_time of
	// the day's 12:00, server time.
	commissionDailyGrace = 12 * time.Hour
	// commissionMaxShips is the most ships a commission takes, the dock's
	// selectedMax (EventMediator EVENT_OPEN_DOCK).
	commissionMaxShips = 6
	// commissionGreatSuccessPercent is the chance a commission ends in a
	// great success (SC_13006.is_cri, the "perfect" grade of
	// ShipExpLayer), which also gives its special_drop. The rate is
	// server-side, nothing captured shows it and the wiki's Commissions
	// page doesn't say: this is a GUESS.
	commissionGreatSuccessPercent = 10
	// itemShipExpDisplay is the "Ship EXP" placeholder of drop_display
	// (item_virtual_data_statistics 59000): SC_13006.exp, not a drop.
	itemShipExpDisplay = 59000
)

// gameset night_collection_begin / night_collection_end: the server hours
// night commissions are offered between (EventProxy:checkNightEvent).
var nightCommissionBegin, nightCommissionEnd uint32

// player_resource row: the placeholder item the client shows the resource as
type commissionResourceConfig struct {
	ItemID uint32 `json:"itemid"`
}

// commissionAwards are what a commission gives: its drop_display (the
// rewards are server-side; the panel's display is what the client has),
// and the special_drop shown as "Chance to receive", given on a great
// success.
type commissionAwards struct {
	awards  []dailyAward
	special *dailyAward
}

var commissionRewards map[uint32]commissionAwards

// readCommissionAwards reads the commissions' drop_display: a count, a
// range ("100~170") or a minimum ("3000+", given as is). The Coins, Oil,
// Gems and Decor Tokens placeholders (59001...) are the player_resource
// whose itemid they are; "Mystery" placeholders are drawn when given
// (resolveContent); what can't be stored (event points of past
// collaborations...) is left out when rolled.
func readCommissionAwards(resources map[uint32]commissionResourceConfig) error {
	byItem := map[uint32]uint32{}
	for id, row := range resources {
		if current, ok := byItem[row.ItemID]; !ok || id < current {
			byItem[row.ItemID] = id
		}
	}
	award := func(kind, id uint32, nums string) (dailyAward, bool) {
		if resource, ok := byItem[id]; ok && kind == dropTypeItem {
			kind, id = dropTypeResource, resource
		}
		return parseDailyAward([]json.RawMessage{
			json.RawMessage(strconv.FormatUint(uint64(kind), 10)),
			json.RawMessage(strconv.FormatUint(uint64(id), 10)),
			json.RawMessage(strconv.Quote(strings.TrimSuffix(nums, "+"))),
		})
	}
	commissionRewards = make(map[uint32]commissionAwards, len(commissionConfigs))
	vague := 0
	for id, config := range commissionConfigs {
		var rewards commissionAwards
		for _, display := range config.DropDisplay {
			if display.Type == dropTypeItem && display.ID == itemShipExpDisplay {
				continue
			}
			parsed, ok := award(display.Type, display.ID, display.Nums)
			if !ok {
				vague++
				continue
			}
			rewards.awards = append(rewards.awards, parsed)
		}
		var special commissionDisplay
		if len(config.SpecialDrop) > 0 && config.SpecialDrop[0] == '{' {
			if err := json.Unmarshal(config.SpecialDrop, &special); err != nil {
				return err
			}
			if parsed, ok := award(special.Type, special.ID, "1"); ok {
				rewards.special = &parsed
			}
		}
		commissionRewards[id] = rewards
	}
	if vague > 0 {
		logger.WithFields("Commission", logger.FieldValue("entries", vague)).Warn("commission drop_display entries without an amount are not given")
	}
	return nil
}

// roll draws a finished commission's drops, the special one on a great
// success, keeping what grantDrops can store.
func (rewards commissionAwards) roll(great bool) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, award := range rewards.awards {
		if count := award.min + rand.Uint32N(award.max-award.min+1); count > 0 {
			drops = resolveContent(drops, award.kind, award.id, count)
		}
	}
	if great && rewards.special != nil {
		drops = resolveContent(drops, rewards.special.kind, rewards.special.id, 1)
	}
	return slices.DeleteFunc(drops, func(drop *protobuf.DROPINFO) bool {
		return !storableDrop(drop.GetType(), drop.GetId())
	})
}

// commissionInfos are the commander's commissions as COLLECTIONINFOs, with
// the ships of the running ones (EventProxy's "inEvent" ships).
func commissionInfos(ctx context.Context, queries *db.Queries, commanderID uint32) ([]*protobuf.COLLECTIONINFO, error) {
	rows, err := queries.ListCommissions(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	ships, err := queries.ListCommissionShips(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	byCommission := map[uint32][]uint32{}
	for _, ship := range ships {
		byCommission[ship.CollectionID] = append(byCommission[ship.CollectionID], ship.ShipID)
	}
	list := make([]*protobuf.COLLECTIONINFO, 0, len(rows))
	for _, row := range rows {
		list = append(list, commissionInfo(row, byCommission[row.CollectionID]))
	}
	return list, nil
}

func commissionInfo(row db.CommanderCommission, ships []uint32) *protobuf.COLLECTIONINFO {
	return &protobuf.COLLECTIONINFO{
		Id:         proto.Uint32(row.CollectionID),
		FinishTime: proto.Uint32(row.FinishTime),
		OverTime:   proto.Uint32(row.OverTime),
		ShipIdList: ships,
	}
}

// nightCommissionUntil is the end of the night commission window now is in
// (EventProxy:checkNightEvent: from night_collection_begin to
// night_collection_end the next day, server hours); false outside it.
func nightCommissionUntil(now time.Time) (time.Time, bool) {
	day, hour := startOfDay(now), uint32(now.In(gamedata.Zone()).Hour())
	end := time.Duration(nightCommissionEnd) * time.Hour
	switch {
	case hour >= nightCommissionBegin:
		return day.AddDate(0, 0, 1).Add(end), true
	case hour < nightCommissionEnd:
		return day.Add(end), true
	}
	return time.Time{}, false
}

// drawCommissions offers up to n commissions of the types kinds open at
// level, not already offered (nor except), with over_time over (0 for no
// limit).
func drawCommissions(ctx context.Context, queries *db.Queries, commanderID, level uint32, kinds []uint32, n int, over, except uint32) ([]*protobuf.COLLECTIONINFO, error) {
	rows, err := queries.ListCommissions(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	var candidates []uint32
	for id, config := range commissionConfigs {
		if slices.Contains(kinds, config.Type) && config.Lv <= level && id != except && !slices.ContainsFunc(rows, func(row db.CommanderCommission) bool { return row.CollectionID == id }) {
			candidates = append(candidates, id)
		}
	}
	rand.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
	var offered []*protobuf.COLLECTIONINFO
	for _, id := range candidates[:min(n, len(candidates))] {
		if err := queries.OfferCommission(ctx, db.OfferCommissionParams{CommanderID: commanderID, CollectionID: id, OverTime: over}); err != nil {
			return nil, err
		}
		offered = append(offered, commissionInfo(db.CommanderCommission{CollectionID: id, OverTime: over}, nil))
	}
	return offered, nil
}

// drawDailyCommissions offers n daily-slot commissions (not except): Daily
// Resource Extraction ones while fewer than commissionDailyPerDay were drawn
// this server day, extra ones after (the wiki's Commissions page).
func drawDailyCommissions(ctx context.Context, queries *db.Queries, commanderID, level uint32, n int, except uint32) ([]*protobuf.COLLECTIONINFO, error) {
	drawn, err := queries.GetCommissionDailyDrawn(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		drawn = commissionDailyPerDay
	} else if err != nil {
		return nil, err
	}
	var offered []*protobuf.COLLECTIONINFO
	for range n {
		kinds := []uint32{commissionTypeExtra}
		if drawn < commissionDailyPerDay {
			kinds = []uint32{commissionTypeDaily, commissionTypeAwakening}
		}
		one, err := drawCommissions(ctx, queries, commanderID, level, kinds, 1, 0, except)
		if err != nil {
			return nil, err
		}
		offered = append(offered, one...)
		drawn++
	}
	if err := queries.AddCommissionDailyDrawn(ctx, db.AddCommissionDailyDrawnParams{DailyDrawn: uint32(n), CommanderID: commanderID}); err != nil {
		return nil, err
	}
	return offered, nil
}

// refreshCommissions brings the commissions on offer up to now and returns
// those it offered or changed:
//   - the ones never started whose over_time passed are dropped (the client
//     drops them itself, EventInfo.StateExpire);
//   - on a new server day (the client asks for it, EventListScene's
//     checkZeroHourEvent flush) commissionDailyCount daily commissions are
//     drawn, the previous day's that weren't started kept until
//     commissionDailyGrace into the day, as captured. A commander's first
//     refresh only records the day: the starter commissions are its draw;
//   - during the night window commissionNightCount night commissions
//     (type 5) are offered until the window ends whenever none is offered
//     or running (the client flushes when its urgency tab finds none,
//     checkNightEvent).
func refreshCommissions(ctx context.Context, queries *db.Queries, commanderID, level uint32, now time.Time) ([]*protobuf.COLLECTIONINFO, error) {
	unix := uint32(now.Unix())
	rows, err := queries.ListCommissions(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	var changed []*protobuf.COLLECTIONINFO
	night := false
	for _, row := range rows {
		if row.FinishTime == 0 && row.OverTime > 0 && row.OverTime <= unix {
			if err := queries.DeleteCommission(ctx, db.DeleteCommissionParams{CommanderID: commanderID, CollectionID: row.CollectionID}); err != nil {
				return nil, err
			}
			continue
		}
		night = night || commissionConfigs[row.CollectionID].Type == commissionTypeNight
	}
	refreshed, err := queries.GetCommissionRefresh(ctx, commanderID)
	day := startOfDay(now)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := queries.SetCommissionRefresh(ctx, db.SetCommissionRefreshParams{CommanderID: commanderID, RefreshedAt: now.Unix()}); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	case refreshed < day.Unix():
		grace := day.Add(commissionDailyGrace)
		for _, row := range rows {
			if !slices.Contains(commissionDailyTypes, commissionConfigs[row.CollectionID].Type) || row.FinishTime != 0 || row.OverTime != 0 {
				continue
			}
			if !now.Before(grace) {
				if err := queries.DeleteCommission(ctx, db.DeleteCommissionParams{CommanderID: commanderID, CollectionID: row.CollectionID}); err != nil {
					return nil, err
				}
				continue
			}
			row.OverTime = uint32(grace.Unix())
			if err := queries.SetCommissionOverTime(ctx, db.SetCommissionOverTimeParams{OverTime: row.OverTime, CommanderID: commanderID, CollectionID: row.CollectionID}); err != nil {
				return nil, err
			}
			changed = append(changed, commissionInfo(row, nil))
		}
		if err := queries.SetCommissionRefresh(ctx, db.SetCommissionRefreshParams{CommanderID: commanderID, RefreshedAt: now.Unix()}); err != nil {
			return nil, err
		}
		offered, err := drawDailyCommissions(ctx, queries, commanderID, level, commissionDailyCount, 0)
		if err != nil {
			return nil, err
		}
		changed = append(changed, offered...)
	}
	if until, ok := nightCommissionUntil(now); ok && !night {
		offered, err := drawCommissions(ctx, queries, commanderID, level, []uint32{commissionTypeNight}, commissionNightCount, uint32(until.Unix()), 0)
		if err != nil {
			return nil, err
		}
		changed = append(changed, offered...)
	}
	return changed, nil
}

// handleStartCommission answers EventStartCommand
// (controller/command/event/eventstartcommand.lua): result 0 sends the
// ships, the client then takes the commission's oil itself and sets its
// finish time to now + collect_time. The commission must be offered and not
// expired, a team free (user_level max_collection), and the ships the
// commander's, not on another commission, at least ship_num of them (at
// most the dock's 6), all of its ship_type, one at ship_lv
// (EventProxy:CanJoinEvent). No official capture has it: the reply is
// built from the Lua alone.
func (game *Game) handleStartCommission(client *connection.Client, req *protobuf.CS_13003) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := uint32(time.Now().Unix())
		config, ok := commissionConfigs[req.GetId()]
		if !ok {
			return errRefused
		}
		row, err := tx.GetCommission(tx.ctx, db.GetCommissionParams{CommanderID: tx.commanderID, CollectionID: req.GetId()})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		if row.FinishTime != 0 || (row.OverTime > 0 && row.OverTime <= now) {
			return errRefused
		}
		commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		rows, err := tx.ListCommissions(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		running := uint32(0)
		for _, other := range rows {
			if other.FinishTime != 0 {
				running++
			}
		}
		if running >= commanderLevels[commander.Level].MaxCollection {
			return errRefused
		}
		ids := req.GetShipIdList()
		if uint32(len(ids)) < config.ShipNum || len(ids) > commissionMaxShips {
			return errRefused
		}
		busy, err := tx.ListCommissionShips(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		leveled := false
		for i, id := range ids {
			if slices.Contains(ids[:i], id) || slices.ContainsFunc(busy, func(ship db.ListCommissionShipsRow) bool { return ship.ShipID == id }) {
				return errRefused
			}
			ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: id, CommanderID: tx.commanderID})
			if errors.Is(err, sql.ErrNoRows) {
				return errRefused
			} else if err != nil {
				return err
			}
			template, ok := ownableShip(ship.TemplateID)
			if !ok || !slices.Contains(config.ShipType, template.Type) {
				return errRefused
			}
			leveled = leveled || ship.Level >= config.ShipLv
		}
		if !leveled {
			return errRefused
		}
		if err := tx.SpendResource(ResourceOil, config.Oil); err != nil {
			return err
		}
		if err := tx.SetCommissionFinishTime(tx.ctx, db.SetCommissionFinishTimeParams{FinishTime: now + config.CollectTime, CommanderID: tx.commanderID, CollectionID: req.GetId()}); err != nil {
			return err
		}
		for pos, id := range ids {
			if err := tx.AddCommissionShip(tx.ctx, db.AddCommissionShipParams{CommanderID: tx.commanderID, CollectionID: req.GetId(), Pos: uint32(pos), ShipID: id}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_13004{Result: result(err)})
}

// handleFinishCommission answers EventFinishCommand
// (controller/command/event/eventfinishcommand.lua) on a commission whose
// finish time passed: exp is what each of its ships gains (the client adds
// it with Ship:addExp, without accumulating: trainShip), drop_list its
// rewards (commissionAwards: its drop_display, the special_drop too when
// is_cri, a great success), new_collection what the client adds once it
// drops the commission. A finished daily-slot one is replaced by the day's
// next daily one (drawDailyCommissions: "A new Commission is generated when
// a Daily is completed"), a major one (type 3) by another ("Exactly one of
// these commissions is available (or in progress) at any given time, with a
// new one generated on completion"), both from the wiki's Commissions page;
// night, urgent and activity ones aren't replaced. The rest comes from
// the client's own flushes (CS_13009: a new day, or no night commission
// left at night). It counts for TaskSubCommission;
// the client counts collect_attack_count itself. No official capture has
// it: the reply is built from the Lua alone.
func (game *Game) handleFinishCommission(client *connection.Client, req *protobuf.CS_13005) error {
	reply := &protobuf.SC_13006{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := time.Now()
		config, ok := commissionConfigs[req.GetId()]
		if !ok {
			return errRefused
		}
		row, err := tx.GetCommission(tx.ctx, db.GetCommissionParams{CommanderID: tx.commanderID, CollectionID: req.GetId()})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		if row.FinishTime == 0 || row.FinishTime > uint32(now.Unix()) {
			return errRefused
		}
		ships, err := tx.ListCommissionShips(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		great := rand.Uint32N(100) < commissionGreatSuccessPercent
		drops := commissionRewards[req.GetId()].roll(great)
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		for _, sent := range ships {
			if sent.CollectionID != req.GetId() {
				continue
			}
			ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: sent.ShipID, CommanderID: tx.commanderID})
			if errors.Is(err, sql.ErrNoRows) {
				continue
			} else if err != nil {
				return err
			}
			if trainShip(&ship, config.Exp) {
				tx.techPointsChanged()
			}
			if err := tx.saveDormShip(ship); err != nil {
				return err
			}
		}
		tx.event(TaskEvent{SubType: TaskSubCommission, Count: 1})
		if err := tx.DeleteCommission(tx.ctx, db.DeleteCommissionParams{CommanderID: tx.commanderID, CollectionID: req.GetId()}); err != nil {
			return err
		}
		if err := tx.DeleteCommissionShips(tx.ctx, db.DeleteCommissionShipsParams{CommanderID: tx.commanderID, CollectionID: req.GetId()}); err != nil {
			return err
		}
		commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		var offered []*protobuf.COLLECTIONINFO
		switch {
		case config.Type == commissionTypeActivity, config.Type == commissionTypeNight, config.Time != 0:
		case slices.Contains(commissionDailyTypes, config.Type):
			if offered, err = drawDailyCommissions(tx.ctx, tx.Queries, tx.commanderID, commander.Level, 1, req.GetId()); err != nil {
				return err
			}
		default:
			if offered, err = drawCommissions(tx.ctx, tx.Queries, tx.commanderID, commander.Level, []uint32{config.Type}, 1, 0, req.GetId()); err != nil {
				return err
			}
		}
		isCri := uint32(0)
		if great {
			isCri = 1
		}
		reply.Exp = proto.Uint32(config.Exp)
		reply.DropList = drops
		reply.NewCollection = offered
		reply.IsCri = proto.Uint32(isCri)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		reply = &protobuf.SC_13006{}
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleGiveUpCommission answers EventGiveUpCommand
// (controller/command/event/eventgiveupcommand.lua) on a running
// commission: result 0 frees its ships and offers it again (the client sets
// its finish time to 0 and clears its ships; one whose over_time passed
// meanwhile is dropped). The oil spent isn't given back, the client keeps
// it spent. No official capture has it: the reply is built from the Lua
// alone.
func (game *Game) handleGiveUpCommission(client *connection.Client, req *protobuf.CS_13007) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		now := uint32(time.Now().Unix())
		row, err := tx.GetCommission(tx.ctx, db.GetCommissionParams{CommanderID: tx.commanderID, CollectionID: req.GetId()})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		} else if err != nil {
			return err
		}
		if row.FinishTime == 0 || row.FinishTime <= now {
			return errRefused
		}
		if row.OverTime > 0 && row.OverTime <= now {
			err = tx.DeleteCommission(tx.ctx, db.DeleteCommissionParams{CommanderID: tx.commanderID, CollectionID: req.GetId()})
		} else {
			err = tx.SetCommissionFinishTime(tx.ctx, db.SetCommissionFinishTimeParams{FinishTime: 0, CommanderID: tx.commanderID, CollectionID: req.GetId()})
		}
		if err != nil {
			return err
		}
		return tx.DeleteCommissionShips(tx.ctx, db.DeleteCommissionShipsParams{CommanderID: tx.commanderID, CollectionID: req.GetId()})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_13008{Result: result(err)})
}

// handleFlushCommissions answers EventFlushCommand
// (controller/command/event/eventflushcommand.lua, type 0), which
// EventListScene sends when the server day changed since its list
// (checkZeroHourEvent) or when the urgency tab finds no night commission
// at night (checkNightEvent): collection_list replaces EventProxy's whole
// list, refreshed first (refreshCommissions). It costs nothing. No official
// capture has it: the reply is built from the Lua alone.
func (game *Game) handleFlushCommissions(client *connection.Client, req *protobuf.CS_13009) error {
	var list []*protobuf.COLLECTIONINFO
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if _, err := refreshCommissions(tx.ctx, tx.Queries, tx.commanderID, commander.Level, time.Now()); err != nil {
			return err
		}
		list, err = commissionInfos(tx.ctx, tx.Queries, tx.commanderID)
		return err
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_13010{Result: result(nil), CollectionList: list})
}
