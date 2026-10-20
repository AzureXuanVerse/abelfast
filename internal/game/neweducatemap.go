package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The map (NewEducateMapScene, "Go Outside"): the work and travel sites,
// the shipgirls, the random events and the shop, each visit paid with
// action points.

// NewEducateConst.SITE_STATE_TYPE / SITE_NORMAL_TYPE
const (
	neStateEvent  = 1
	neStateNormal = 2
	neStateShip   = 3
	neStateShop   = 4
)

// How much a round's map offers is in no table: guesses, from the wiki's
// screenshots and descriptions (two shipgirls, one per personality side;
// a few events; a shop shelf).
const (
	neMapEvents = 3
	neMapGoods  = 6
)

// neGoodsCharacter is the character a child2_shop row (which has no
// character column) belongs to: Oceana's goods are below 3000, Explorer's
// 3000 to 3999 (9000 up are endless / test goods, never offered).
func neGoodsCharacter(id uint32) uint32 {
	switch {
	case id < 3000:
		return 1
	case id < 4000:
		return 2
	}
	return 0
}

// works are the current work and travel site levels (NewEducateChar:
// InitSiteData: level 1 until upgraded).
func (state *neState) work(kind uint32) uint32 {
	for _, id := range state.Works {
		if neNormals[id].Type == kind {
			return id
		}
	}
	var first uint32
	for id, normal := range neNormals {
		if normal.Character == state.ID && normal.Type == kind && normal.Level == 1 && (first == 0 || id < first) {
			first = id
		}
	}
	return first
}

// rollShops draws the shop's goods.
func (state *neState) rollShops() []uint32 {
	var pool []uint32
	for id := range neGoods {
		if neGoodsCharacter(id) == state.ID {
			pool = append(pool, id)
		}
	}
	slices.Sort(pool)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	pool = pool[:min(len(pool), neMapGoods)]
	slices.Sort(pool)
	return pool
}

// rollMap draws a round's map: events by child2_site_event_group weight
// among those whose condition holds (one per site position), the shop's
// goods, and a shipgirl of each side (site display bg) not yet at her last
// level.
func (ne *neGame) rollMap() {
	state := ne.neState
	site := &neSiteCache{Buys: map[uint32]uint32{}}
	if state.unlocked("rand_event") && !state.InTemp {
		weights := map[uint32]int64{}
		for _, id := range neCharEvents[state.ID] {
			if event := neEvents[id]; state.match(event.Condition, nil) {
				weights[id] = max(event.Weight, 1)
			}
		}
		site.Events = nePick(weights, neMapEvents)
		slices.Sort(site.Events)
	}
	if state.unlocked("shop") && neCharShop[state.ID] {
		site.Shops = state.rollShops()
	}
	state.Site = site
	state.SiteShips = nil
	if !state.unlocked("char_event") || state.InTemp {
		return
	}
	bySide := map[string][]uint32{}
	for _, group := range neCharShips[state.ID] {
		current := state.ShipLevels[group]
		if current == 0 {
			current = neShipGroups[group][0]
		}
		if neShips[current].Node == 0 {
			continue // last level reached
		}
		bySide[neShipBg[group]] = append(bySide[neShipBg[group]], current)
	}
	for _, side := range []string{"blue", "red"} {
		if ships := bySide[side]; len(ships) > 0 {
			state.SiteShips = append(state.SiteShips, ships[rand.IntN(len(ships))])
		}
	}
}

// handleNewEducateGetMap answers NewEducateGetMapCommand, asked once a
// round: the map's events, shop and shipgirls.
func (game *Game) handleNewEducateGetMap(client *connection.Client, req *protobuf.CS_29060) error {
	var site *protobuf.TBFSMCACHESITE
	var ships []uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.Site == nil {
			ne.rollMap()
		}
		if ne.Node == 0 && (ne.System == neSysChoose || ne.System == neSysTalent || ne.System == neSysTopic || ne.System == neSysMind) {
			ne.System = neSysMap
		}
		site = ne.siteCache()
		ships = ne.SiteShips
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29061{Result: result(err), FsmSite: site, Characters: ships, Drop: neDrops(nil, nil)}
	})
}

// mapAction checks that the map can be used now: no chain or priority
// state running, not during the schedule.
func (state *neState) mapAction() error {
	if state.Site == nil || state.Node != 0 || len(state.Priority) > 0 || len(state.TarotSelects) > 0 {
		return errRefused
	}
	switch state.System {
	case neSysChoose, neSysTalent, neSysTopic, neSysMind, neSysMap:
		return nil
	}
	return errRefused
}

// handleNewEducateMapNormal answers NewEducateMapNormalCommand, a visit to
// the work or travel site: paid (cost), counted, and its chain gives the
// site's drop_display.
func (game *Game) handleNewEducateMapNormal(client *connection.Client, req *protobuf.CS_29062) error {
	var node uint32
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		id := req.GetWorkId()
		normal, ok := neNormals[id]
		if err := ne.mapAction(); err != nil {
			return err
		}
		if !ok || normal.Character != ne.ID || ne.work(normal.Type) != id || !ne.unlocked("out") {
			return errRefused
		}
		if err := ne.pay(neDropsOf(normal.Cost)); err != nil {
			return err
		}
		ne.WorkCounter[id]++
		ne.System = neSysMap
		ne.Site.StateKey, ne.Site.StateVal = neStateNormal, id
		node = normal.Node
		drops = ne.startReward(node, neDropsOf(normal.Drops), neTrigMap, nil)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29063{Result: result(err), FirstNode: proto.Uint32(node), Drop: drops}
	})
}

// handleNewEducateMapEvent answers NewEducateMapEventCommand: a random
// event of the round's map, paid, played once.
func (game *Game) handleNewEducateMapEvent(client *connection.Client, req *protobuf.CS_29064) error {
	var node uint32
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		id := req.GetEvent()
		if err := ne.mapAction(); err != nil {
			return err
		}
		if !slices.Contains(ne.Site.Events, id) {
			return errRefused
		}
		event := neEvents[id]
		if err := ne.pay(neDropsOf(event.Cost)); err != nil {
			return err
		}
		ne.EventCount[id]++
		ne.Site.Events = slices.DeleteFunc(ne.Site.Events, func(e uint32) bool { return e == id })
		ne.System = neSysMap
		ne.Site.StateKey, ne.Site.StateVal = neStateEvent, id
		node = event.Node
		drops = ne.startReward(node, nil, neTrigMap, nil)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29065{Result: result(err), FirstNode: proto.Uint32(node), Drop: drops}
	})
}

// handleNewEducateMapShip answers NewEducateMapShipCommand, a talk with a
// shipgirl: paid, she moves to her next level (seen once a round), and the
// chain gives her drop_display.
func (game *Game) handleNewEducateMapShip(client *connection.Client, req *protobuf.CS_29068) error {
	var node uint32
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		id := req.GetCharacter()
		ship, ok := neShips[id]
		if err := ne.mapAction(); err != nil {
			return err
		}
		if !ok || !slices.Contains(ne.SiteShips, id) || slices.Contains(ne.Site.Selected, id) || ship.Node == 0 {
			return errRefused
		}
		if err := ne.pay(neDropsOf(ship.Cost)); err != nil {
			return err
		}
		levels := neShipGroups[ship.Group]
		if i := slices.Index(levels, id); i >= 0 && i+1 < len(levels) {
			next := levels[i+1]
			ne.ShipLevels[ship.Group] = next
			ne.SiteShips[slices.Index(ne.SiteShips, id)] = next
			ne.Site.Selected = append(ne.Site.Selected, next)
		}
		ne.System = neSysMap
		ne.Site.StateKey, ne.Site.StateVal = neStateShip, id
		node = ship.Node
		drops = ne.startReward(node, neDropsOf(ship.Drops), neTrigMap, nil)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29069{Result: result(err), FirstNode: proto.Uint32(node), Drop: drops}
	})
}

// handleNewEducateShopping answers NewEducateShoppingCommand: goods of the
// round's shop, paid with the buffs' discounts (as the client computes
// them), within the goods' limit; the shopping trigger's effects come on
// top.
func (game *Game) handleNewEducateShopping(client *connection.Client, req *protobuf.CS_29066) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		id, num := req.GetShop(), int64(req.GetNum())
		goods, ok := neGoods[id]
		if err := ne.mapAction(); err != nil {
			return err
		}
		if !ok || num <= 0 || !slices.Contains(ne.Site.Shops, id) {
			return errRefused
		}
		if goods.Limit != -1 && int64(ne.Site.Buys[id])+num > goods.Limit {
			return errRefused
		}
		cost := ne.goodsCost(id)
		cost.Number *= num
		if err := ne.pay([]neDrop{cost}); err != nil {
			return err
		}
		ne.Site.Buys[id] += uint32(num)
		ne.Site.StateKey, ne.Site.StateVal = neStateShop, 0
		var base []neDrop
		for range num {
			base = append(base, ne.goodsDrops(goods)...)
		}
		before := len(ne.Priority)
		drops = ne.reward(base, neTrigShop, nil)
		for i := range len(ne.Priority) - before {
			ne.Priority[i].FromShop = true
		}
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29067{Result: result(err), Drop: drops} })
}

// NewEducateGoods.TYPE
const (
	neGoodsBuff    = 1
	neGoodsAttr    = 2
	neGoodsRes     = 3
	neGoodsChoose  = 4
	neGoodsUpEntry = 5
)

// goodsDrops is what one of goods gives: a buff, attributes, resources, a
// choice among readings (goods_id_list's pool) or a reading level-up.
func (state *neState) goodsDrops(goods neGoodsConfig) []neDrop {
	switch goods.Type {
	case neGoodsBuff:
		return []neDrop{{neDropBuff, goods.Goods, max(goods.Num, 1)}}
	case neGoodsAttr:
		return []neDrop{{neDropAttr, goods.Goods, goods.Num}}
	case neGoodsRes:
		return []neDrop{{neDropRes, goods.Goods, goods.Num}}
	case neGoodsChoose:
		var pools []uint32
		_ = json.Unmarshal(goods.GoodsList, &pools)
		var drops []neDrop
		for _, pool := range pools {
			drops = append(drops, neDrop{neDropChoose, pool, max(goods.Num, 1)})
		}
		return drops
	case neGoodsUpEntry:
		return []neDrop{{neDropUpEntry, goods.Goods, max(goods.Num, 1)}}
	}
	return nil
}

// handleNewEducateRefreshShop answers NewEducateRefreshShopCommand: new
// goods, paid with a shop refresh when the character has some left, else
// with funds (gameset child2_shop_refresh_price), counted; funds refreshes
// are capped by child2_shop_refresh_count a round.
func (game *Game) handleNewEducateRefreshShop(client *connection.Client, req *protobuf.CS_29072) error {
	var shops []uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if err := ne.mapAction(); err != nil {
			return err
		}
		if len(ne.Site.Shops) == 0 && !neCharShop[ne.ID] {
			return errRefused
		}
		if res := ne.resOfType(neResRefreshShop); res != 0 && ne.Res[res] > 0 {
			ne.Res[res]--
		} else {
			if neShopRefreshCount > 0 && ne.Site.Refresh >= neShopRefreshCount {
				return errRefused
			}
			if err := ne.pay([]neDrop{{neDropRes, ne.resOfType(neResMoney), neShopRefreshPrice}}); err != nil {
				return err
			}
			ne.Site.Refresh++
		}
		ne.Site.Shops = ne.rollShops()
		ne.Site.Buys = map[uint32]uint32{}
		shops = ne.Site.Shops
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29073{Result: result(err), Shops: shops} })
}

// handleNewEducateUpgradeNormalSite answers
// NewEducateUpgradeNormalSiteCommand, sent by the map when a work / travel
// site's special_args holds: the site's next level.
func (game *Game) handleNewEducateUpgradeNormalSite(client *connection.Client, req *protobuf.CS_29070) error {
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		id := req.GetWorkId()
		normal, ok := neNormals[id]
		if !ok || normal.Character != ne.ID || ne.work(normal.Type) != id || !ne.match(normal.Upgrade, nil) {
			return errRefused
		}
		var next uint32
		for nid, n := range neNormals {
			if n.Character == ne.ID && n.Type == normal.Type && n.Level == normal.Level+1 {
				next = nid
			}
		}
		if next == 0 {
			return errRefused
		}
		ne.Works = slices.DeleteFunc(ne.Works, func(w uint32) bool { return neNormals[w].Type == normal.Type })
		ne.Works = append(ne.Works, next)
		slices.Sort(ne.Works)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29071{Result: result(err)} })
}
