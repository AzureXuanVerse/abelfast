package game

import (
	"context"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Going out in Project Identity: TB (EducateMapScene, unlocked at gameset
// child_out_unlock): the sites (child_site of type 1, each opening at its
// unlock time; type 2 are the places inside them) offer options
// (child_site_option): a shop (type 1), a place (type 3) or an activity
// (type 2, CS_27004) paid with action points and money.

// child_site_option types (EducateSiteOption)
const (
	childOptionShop  = 1
	childOptionEvent = 2
)

// educatePolaroidChance is the chance (1/10000) that a "Wander Around"
// option (one with a polarid_list) gives one of its photos still missing.
// The wiki only says "sometimes" (Project Identity: TB, Moments): a guess.
const educatePolaroidChance = 5000

// siteOpen is EducateHelper.IsSiteUnlock (the first game's unlock times
// differ from the next ones').
func (state *educateState) siteOpen(site educateSiteConfig) bool {
	unlock := site.Unlock2
	if state.firstGame() {
		unlock = site.Unlock1
	}
	return len(unlock) < 3 || educateInUnlockTime(state.Time, unlock)
}

// educateInUnlockTime is EducateHelper.InUnlockTime: t is at or after
// [month, week, day].
func educateInUnlockTime(t tbTime, at []uint32) bool {
	return t.key() >= tbPoint(at).key()
}

// randomOptions are the week's options drawn for a site
// (child_site.option_random: one per group, weighted), the same all week
// long: the draw is seeded with the commander, the game and the week.
func (state *educateState) randomOptions(commanderID, siteID uint32) []uint32 {
	site := educateSites[siteID]
	if len(site.Random) == 0 {
		return nil
	}
	random := rand.New(rand.NewPCG(uint64(commanderID), uint64(state.Games)<<32|uint64(state.Time.index())<<16|uint64(siteID)))
	var picks []uint32
	for _, group := range site.Random {
		total := uint32(0)
		for _, entry := range group {
			if len(entry) >= 2 {
				total += entry[1]
			}
		}
		if total == 0 {
			continue
		}
		roll := random.Uint32N(total)
		for _, entry := range group {
			if len(entry) < 2 {
				continue
			}
			if roll < entry[1] {
				picks = append(picks, entry[0])
				break
			}
			roll -= entry[1]
		}
	}
	return picks
}

// handleEducateRandomOptions answers EducateRequestOptionCommand (CS_27045,
// EducateMapScene once a week): the week's random options of every site
// that has some.
func (game *Game) handleEducateRandomOptions(client *connection.Client, req *protobuf.CS_27045) error {
	state, err := loadEducate(context.Background(), game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if state == nil {
		return client.Send(&protobuf.SC_27046{Result: result(errRefused)})
	}
	reply := &protobuf.SC_27046{Result: result(nil)}
	for _, id := range sortedKeys(educateSites) {
		if picks := state.randomOptions(client.CommanderID, id); len(picks) > 0 {
			reply.Opts = append(reply.Opts, &protobuf.CHILD_SITE_OPTION{SiteId: proto.Uint32(id), OptionIds: picks})
		}
	}
	return client.Send(reply)
}

// handleEducateSiteOption answers EducateMapSiteCommand (CS_27004): does an
// activity of a site (an option of child_site.option or of its week's random
// ones, open at this time, not used up: count_limit[1] uses until its
// refresh week), paying its cost (resources: action points, money, mood).
// TB gets its result_display (scaled like a plan's; competitions and random
// rewards, "-1", are server content), one of the photos of a wander option
// that she can still get (educatePolaroidChance), and a competition's
// trophy for first place. The branch (child_site_option_branch, the
// performance played) is the first of the option's results whose attribute
// condition she meets and whose ratio rolls, the last one otherwise; an
// option's own events (child_event type 2) may follow. Built from the Lua
// alone.
func (game *Game) handleEducateSiteOption(client *connection.Client, req *protobuf.CS_27004) error {
	reply := &protobuf.SC_27005{BranchId: proto.Uint32(0)}
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		siteID, id := req.GetSiteid(), req.GetOptionid()
		site, ok := educateSites[siteID]
		option, known := educateOptions[id]
		if !ok || !known || option.Type != childOptionEvent || !state.playing() || !state.siteOpen(site) {
			return errRefused
		}
		if state.Time.key() < educateOutUnlock.key() {
			return errRefused
		}
		if !slices.Contains(site.Option, id) && !slices.Contains(state.randomOptions(client.CommanderID, siteID), id) {
			return errRefused
		}
		if !option.Time.open(state.Time) || len(option.Result) == 0 {
			return errRefused
		}
		if uses, _, ok := option.countLimit(); ok && state.Options[id] >= uses {
			return errRefused
		}
		for _, cost := range option.Cost {
			if cost[0] != childDropRes || state.resource(uint32(cost[1])) < cost[2] {
				return ErrNotEnough
			}
		}
		changes := &educateTaskChanges{}
		for _, cost := range option.Cost {
			state.addResource(uint32(cost[1]), -cost[2])
		}
		state.Options[id]++
		branch := option.Result[len(option.Result)-1]
		for _, candidate := range option.Result {
			config := educateBranches[candidate]
			if state.meets(config.Attr) && rand.Uint32N(10000) < config.Ratio {
				branch = candidate
				break
			}
		}
		reply.BranchId = proto.Uint32(branch)
		drops := state.display(option.Display, state.moodRatio())
		if item := state.drawOptionItem(id, option, branch); item != 0 {
			drops = append(drops, childDrop(childDropItem, item, 1))
		}
		if polaroid := state.drawPolaroid(option.Polaroids); polaroid != 0 {
			drops = append(drops, childDrop(childDropPolaroid, polaroid, 1))
		}
		state.apply(drops)
		reply.Drops = drops
		if event := state.drawEvent(childEventOption, id, state.Time); event != 0 {
			reply.Events = append(reply.Events, event)
		}
		state.count(childTaskSiteCost, id, 1, changes)
		state.measureAll(changes)
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	if err != nil {
		reply.Drops, reply.Events = nil, nil
	}
	return client.Send(reply)
}

// drawOptionItem is the item an option gives (educateOptionItems): a
// competition's (several branches) on its best branch, the first of its
// results; another option's every time when it has one item, half the
// time one of its items at random when it has several (the cook's dishes,
// "sometimes" on the wiki). 0 for none. A guess: the drops are server
// content.
func (state *educateState) drawOptionItem(id uint32, option educateOptionConfig, branch uint32) uint32 {
	items := educateOptionItems[id]
	switch {
	case len(items) == 0:
		return 0
	case len(option.Result) > 1:
		if branch != option.Result[0] {
			return 0
		}
	case len(items) > 1 && rand.IntN(2) == 0:
		return 0
	}
	return items[rand.IntN(len(items))]
}

// resource is EducateChar:GetResById.
func (state *educateState) resource(id uint32) int64 {
	switch id {
	case educateResMoney:
		return int64(state.Money)
	case educateResMood:
		return int64(state.Mood)
	case educateResSite:
		return int64(state.Site)
	case educateResFavor:
		return int64(state.FavorExp)
	}
	return 0
}

// drawPolaroid is a photo of one of the groups TB has none of yet and can
// get now (EducateProxy:CanGetPolaroidById: her stage, her personality when
// the photo has one), at educatePolaroidChance; 0 for none.
func (state *educateState) drawPolaroid(groups []uint32) uint32 {
	if len(groups) == 0 || rand.Uint32N(10000) >= educatePolaroidChance {
		return 0
	}
	owned := map[uint32]bool{}
	for id := range state.Polaroids {
		owned[educatePolaroids[id].Group] = true
	}
	stage, personality := state.stage(), state.personality()
	var candidates []uint32
	for _, id := range sortedKeys(educatePolaroids) {
		polaroid := educatePolaroids[id]
		if !slices.Contains(groups, polaroid.Group) || owned[polaroid.Group] || !slices.Contains(polaroid.Stage, stage) {
			continue
		}
		if personalities := gamedata.IDs(polaroid.Xingge); len(personalities) > 0 && !slices.Contains(personalities, personality) {
			continue
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		return 0
	}
	return candidates[rand.IntN(len(candidates))]
}

// handleEducateShopData answers EducateRequestShopDataCommand (CS_27043,
// EducateShopProxy:OnNewWeek for the shops restocking that week): the
// shop's goods, restocked by the week's turn.
func (game *Game) handleEducateShopData(client *connection.Client, req *protobuf.CS_27043) error {
	state, err := loadEducate(context.Background(), game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	goods, ok := map[uint32]uint32(nil), false
	if state != nil {
		goods, ok = state.Shops[req.GetShopId()]
	}
	if !ok {
		return client.Send(&protobuf.SC_27044{Result: result(errRefused), ShopData: &protobuf.CHILD_SHOP_DATA{ShopId: proto.Uint32(req.GetShopId())}})
	}
	return client.Send(&protobuf.SC_27044{Result: result(nil), ShopData: shopData(req.GetShopId(), goods)})
}

// handleEducateShopping answers EducateShoppingCommand (CS_27033): buys
// goods of a shop (child_shop_template, in stock and on sale at this time)
// for its resource (money), giving buy_num of its item each (an item gives
// its display values, EducateHelper.GetItemAddDrops). Built from the Lua
// alone.
func (game *Game) handleEducateShopping(client *connection.Client, req *protobuf.CS_27033) error {
	var drops []*protobuf.CHILD_DROP
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		shop, ok := state.Shops[req.GetShopId()]
		if !ok || !state.playing() || len(req.GetGoods()) == 0 {
			return errRefused
		}
		costs := map[uint32]int64{}
		for _, item := range req.GetGoods() {
			goods, ok := educateGoods[item.GetId()]
			if !ok || item.GetNum() == 0 || shop[item.GetId()] < item.GetNum() || !goods.time.open(state.Time) {
				return errRefused
			}
			costs[goods.Resource] += int64(goods.Price) * int64(item.GetNum())
		}
		for id, cost := range costs {
			if state.resource(id) < cost {
				return ErrNotEnough
			}
		}
		changes := &educateTaskChanges{}
		for id, cost := range costs {
			state.addResource(id, -cost)
		}
		for _, item := range req.GetGoods() {
			goods := educateGoods[item.GetId()]
			shop[item.GetId()] -= item.GetNum()
			drops = append(drops, childDrop(childDropItem, goods.Item, int64(goods.BuyNum*item.GetNum())))
			state.count(childTaskPurchase, item.GetId(), item.GetNum(), changes)
		}
		state.apply(drops)
		state.measureAll(changes)
		changes.push(tx, state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_27034{Result: result(err), Drops: drops})
}
