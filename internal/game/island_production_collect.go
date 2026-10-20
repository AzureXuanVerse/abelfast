package game

import (
	"context"
	"math"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Work by hand at the sites: the mine's and the woods' nodes
// (IslandCollectSlotPlace: build_collect, the normal nodes in collect_list
// in the order they grow back, get_num of them taken until refresh_time
// when they all are back) and the fields (IslandHandSlot: hand_list), and
// the automatic collection of the nodes and the wild resources by
// characters (IslandCollectionPanel).

// IslandCollectSlotPlace.slotType
const (
	nodeNormal = 1
	nodeTask   = 2
)

// island gold, what the automatic collection costs
const islandGoldItem = 1

// productionAutoCost is an island_set mining_auto_parameters entry: what
// collecting a node of a site automatically costs.
type productionAutoCost struct {
	coin, energy, exp uint32
}

var (
	productionAutoCosts map[uint32]productionAutoCost
	// island_set mining_recovery_tim_new: how long the nodes take to grow
	// back after the first is taken
	productionNodeRecovery uint32
)

// nodeQueue is a site's normal nodes in the order they grow back.
func nodeQueue(collect *protobuf.PB_ISLAND_BUILD_COLLECT) []uint32 {
	var queue []uint32
	for _, id := range collect.GetCollectList() {
		if !productionTaskNodes[id] {
			queue = append(queue, id)
		}
	}
	return queue
}

// refreshNodes brings a site's nodes back once their refresh_time is past
// (IslandCollectSlotPlace.UpdatePerSecond).
func refreshNodes(collect *protobuf.PB_ISLAND_BUILD_COLLECT, now uint32) {
	if collect.GetRefreshTime() <= now {
		collect.GetNum = proto.Uint32(0)
	}
}

// nodeReady tells whether a normal node can be taken now
// (IslandCollectSlotNew.GetCanCollectTimeStamps): the first
// len(queue) - get_num of the queue are.
func nodeReady(collect *protobuf.PB_ISLAND_BUILD_COLLECT, id uint32) bool {
	queue := nodeQueue(collect)
	index := slices.Index(queue, id)
	return index >= 0 && uint32(index) < uint32(len(queue))-min(collect.GetGetNum(), uint32(len(queue)))
}

// takeNodes counts taken nodes: the first taken after the nodes are back
// starts their regrowth (GUESS: mining_recovery_tim_new from then, sooner
// with a TYPE_RECOVER_CAMP / ORE ability: by its time of day).
func (state *islandState) takeNodes(place uint32, collect *protobuf.PB_ISLAND_BUILD_COLLECT, taken, now uint32) {
	if collect.GetGetNum() == 0 {
		refresh := now + productionNodeRecovery
		kind := uint32(islandAbilityRecoverCamp)
		if place == productionPlaceMine {
			kind = islandAbilityRecoverOre
		}
		for _, id := range state.public().GetAbilityList() {
			if ability := productionAbilities[id]; ability.Type == kind {
				if by := uint32(startOfDay(serverNow()).Unix()) + ability.Effect; by > now && by < refresh {
					refresh = by
				}
			}
		}
		collect.RefreshTime = proto.Uint32(refresh)
	}
	collect.GetNum = proto.Uint32(min(collect.GetGetNum()+taken, uint32(len(nodeQueue(collect)))))
}

// handProduct is what a cycle of a formula worked by hand gives. GUESS:
// its drop_display (what the fields show), else its commission_product.
func handProduct(formulaID uint32) [][2]uint32 {
	formula := productionFormulas[formulaID]
	if pairs := productionPairs(formula.DropDisplay); len(pairs) > 0 {
		return pairs
	}
	return productionPairs(formula.CommissionProduct)
}

// nodeFormula is the formula a node yields: its slot's first.
func nodeFormula(slotID uint32) uint32 {
	formulas := productionSlotFormulas(productionSlots[slotID])
	if len(formulas) == 0 {
		return 0
	}
	return formulas[0]
}

// handEvents counts work done by hand at a site: HAND_GATHER (by formula),
// HAND_IN_PLACE and the BUILDING_HAND achievement (by site).
func (tx *Tx) handEvents(place, formulaID, count uint32, drops []*protobuf.DROPINFO) error {
	if count == 0 {
		return nil
	}
	if err := tx.islandTaskEvent(islandTaskHandGather, count, formulaID); err != nil {
		return err
	}
	if err := tx.islandTaskEvent(islandTaskHandInPlace, count, place); err != nil {
		return err
	}
	if err := tx.islandAchievementEvent(islandAchievementBuildingHand, place, count); err != nil {
		return err
	}
	for _, drop := range drops {
		if err := tx.islandTaskEvent(islandTaskProdItem, drop.GetNumber(), drop.GetId()); err != nil {
			return err
		}
	}
	return nil
}

// handleIslandSlotCollect answers IslandSlotCollectCommand (CS_21507): the
// commander takes a node of the mine or the woods by hand, a normal one
// that has grown back (type 1: it goes to the back of the queue) or a
// task's (type 2: gone for good). drop_list has its formula's product
// (handProduct), refresh_time when the nodes are all back.
func (game *Game) handleIslandSlotCollect(client *connection.Client, req *protobuf.CS_21507) error {
	reply := &protobuf.SC_21508{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place, slotID := req.GetBuildId(), req.GetAreaId()
		build := state.building(place)
		if build == nil || build.BuildCollect == nil || !slices.Contains(build.BuildCollect.GetCollectList(), slotID) {
			reply.Result = result(errRefused)
			return nil
		}
		collect := build.BuildCollect
		refreshNodes(collect, now)
		switch {
		case req.GetType() == nodeNormal && !productionTaskNodes[slotID] && nodeReady(collect, slotID):
			state.takeNodes(place, collect, 1, now)
			list := slices.DeleteFunc(collect.CollectList, func(id uint32) bool { return id == slotID })
			collect.CollectList = append(list, slotID)
		case req.GetType() == nodeTask && productionTaskNodes[slotID]:
			collect.CollectList = slices.DeleteFunc(collect.CollectList, func(id uint32) bool { return id == slotID })
		default:
			reply.Result = result(errRefused)
			return nil
		}
		formulaID := nodeFormula(slotID)
		drops := productionItemDrops(handProduct(formulaID), 1, 0)
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		if err := tx.handEvents(place, formulaID, 1, drops); err != nil {
			return err
		}
		reply.Result = result(nil)
		reply.RefreshTime = proto.Uint32(collect.GetRefreshTime())
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handArea is a field of a site, nil if it has none such.
func handArea(build *protobuf.PB_ISLAND_BUILD, slotID uint32) *protobuf.PB_ISLAND_HAND_AREA {
	for _, area := range build.GetHandList() {
		if area.GetId() == slotID {
			return area
		}
	}
	return nil
}

// growTime is how long a formula takes to grow on a field. GUESS: its
// workload at base_efficiency, sped up by the site's speed abilities like a
// delegation.
func (state *islandState) growTime(formulaID, place uint32) uint32 {
	bonus := uint32(0)
	if kind := productionSpeedAbility(place); kind != 0 {
		bonus = state.islandAbilityEffects(kind)
	}
	return uint32(math.Floor(float64(productionFormulas[formulaID].Workload) / (float64(productionBaseEfficiency) * (1 + 0.01*float64(bonus)))))
}

// handleIslandHandPlant answers IslandSlotHandPlantCommand (CS_21509): the
// commander sows a formula (a seed) on empty fields of a site, paying its
// cost per field (taken by the client too). The reply has the sown fields
// growing; the others on the island see them (SC_21221). Refused when a
// field isn't empty, its ground is taken by a delegation (exclusion_slot,
// GUESS), or the formula isn't the field's or unlocked.
func (game *Game) handleIslandHandPlant(client *connection.Client, req *protobuf.CS_21509) error {
	reply := &protobuf.SC_21510{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place, formulaID, slotList := req.GetBuildId(), req.GetFormulaId(), req.GetSlotList()
		build := state.building(place)
		if build == nil || len(slotList) == 0 || !state.formulaUnlocked(formulaID) {
			reply.Result = result(errRefused)
			return nil
		}
		for i, slotID := range slotList {
			area := handArea(build, slotID)
			slot := productionSlots[slotID]
			busy := false
			for _, other := range productionSlotExclusion(slotID) {
				busy = busy || shipAppoint(build, other) != nil
			}
			if area == nil || area.GetState() != 0 || slot.Place != place || busy || slices.Contains(slotList[:i], slotID) ||
				!slices.Contains(productionSlotFormulas(slot), formulaID) {
				reply.Result = result(errRefused)
				return nil
			}
		}
		count := uint32(len(slotList))
		cost := productionPairs(productionFormulas[formulaID].Cost)
		for _, pair := range cost {
			if state.islandItemCount(pair[0]) < pair[1]*count {
				reply.Result = result(ErrNotEnough)
				return nil
			}
		}
		for _, pair := range cost {
			if err := state.takeIslandItem(pair[0], pair[1]*count); err != nil {
				return err
			}
		}
		end := now + state.growTime(formulaID, place)
		for _, slotID := range slotList {
			area := handArea(build, slotID)
			area.State = proto.Uint32(1)
			area.FormulaId = proto.Uint32(formulaID)
			area.StartTime = proto.Uint32(now)
			area.EndTime = proto.Uint32(end)
			reply.HandList = append(reply.HandList, proto.Clone(area).(*protobuf.PB_ISLAND_HAND_AREA))
		}
		reply.Result = result(nil)
		return tx.islandTaskEvent(islandTaskHandProd, count, formulaID)
	})
	if err != nil {
		return err
	}
	if len(reply.HandList) > 0 {
		game.tellIsland(client, client.CommanderID, &protobuf.SC_21221{IslandId: proto.Uint32(client.CommanderID), HandList: reply.HandList})
	}
	return client.Send(reply)
}

// handleIslandHandPlantAward answers IslandSlotHandPlantAwardCommand
// (CS_21511): the commander harvests grown fields of a site, emptying them;
// drop_list has each field's product (handProduct).
func (game *Game) handleIslandHandPlantAward(client *connection.Client, req *protobuf.CS_21511) error {
	reply := &protobuf.SC_21512{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		place := req.GetBuildId()
		build := state.building(place)
		if build == nil || len(req.GetAreaIds()) == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		for i, slotID := range req.GetAreaIds() {
			area := handArea(build, slotID)
			if area == nil || area.GetState() != 1 || area.GetEndTime() > now || slices.Contains(req.GetAreaIds()[:i], slotID) {
				reply.Result = result(errRefused)
				return nil
			}
		}
		byFormula := map[uint32]uint32{}
		var drops []*protobuf.DROPINFO
		for _, slotID := range req.GetAreaIds() {
			area := handArea(build, slotID)
			byFormula[area.GetFormulaId()]++
			drops = append(drops, productionItemDrops(handProduct(area.GetFormulaId()), 1, 0)...)
			*area = *newHandArea(slotID)
		}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		for _, formulaID := range sortedKeys(byFormula) {
			if err := tx.handEvents(place, formulaID, byFormula[formulaID], nil); err != nil {
				return err
			}
		}
		for _, drop := range drops {
			if err := tx.islandTaskEvent(islandTaskProdItem, drop.GetNumber(), drop.GetId()); err != nil {
				return err
			}
		}
		reply.Result = result(nil)
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandStopHandPlant answers IslandStopHandPlantHalfWayCommand
// (CS_21516): the commander digs up growing fields, emptying them. GUESS:
// the seeds are lost.
func (game *Game) handleIslandStopHandPlant(client *connection.Client, req *protobuf.CS_21516) error {
	reply := &protobuf.SC_21517{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		build := state.building(req.GetBuildId())
		if build == nil || len(req.GetSlotList()) == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		for _, slotID := range req.GetSlotList() {
			if area := handArea(build, slotID); area == nil || area.GetState() != 1 {
				reply.Result = result(errRefused)
				return nil
			}
		}
		for _, slotID := range req.GetSlotList() {
			*handArea(build, slotID) = *newHandArea(slotID)
		}
		reply.Result = result(nil)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// IslandAutoCollectHelper.SelectType - 1, as CS_21539 sends it
const (
	autoCollectNodes  = 1
	autoCollectGather = 2
	autoCollectBoth   = 3
)

// handleIslandAutoCollect answers IslandAutomaticCollectionCommand
// (CS_21539): up to three characters collect every node of the mine and the
// woods that has grown back (of the default ones, mining_default_slot) and/or
// every wild resource of the island at once. It costs, per node, the site's
// mining_auto_parameters and per resource its auto_parameters ([coin,
// energy, exp]): gold (cost_gold, taken by the client) and the characters'
// energy, cut by their collecting grades' gather_effect %, taken from each
// in turn, each getting the exp of its share. The reply has the products,
// the characters' energy and exp, the resources taken (gather_list) and the
// sites' nodes all taken (build_refresh).
func (game *Game) handleIslandAutoCollect(client *connection.Client, req *protobuf.CS_21539) error {
	reply := &protobuf.SC_21540{}
	var gathered []*protobuf.PB_ISLAND_GATHER_PUSH
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		kind := req.GetType()
		var ships []*protobuf.PB_ISLAND_SHIP
		for i, id := range req.GetShipList() {
			ship := state.islandShip(id)
			if ship == nil || shipBusy(ship, now) || slices.Contains(req.GetShipList()[:i], id) || i >= 3 {
				reply.Result = result(errRefused)
				return nil
			}
			ships = append(ships, ship)
		}
		if len(ships) == 0 || kind < autoCollectNodes || kind > autoCollectBoth {
			reply.Result = result(errRefused)
			return nil
		}
		var coin, energy, exp uint32
		var drops []*protobuf.DROPINFO
		type nodeTake struct {
			place   uint32
			formula uint32
		}
		var nodes []nodeTake
		if kind == autoCollectNodes || kind == autoCollectBoth {
			for _, place := range []uint32{productionPlaceMine, productionPlaceFelling} {
				build := state.building(place)
				if build == nil || build.BuildCollect == nil {
					continue
				}
				refreshNodes(build.BuildCollect, now)
				for _, id := range build.BuildCollect.GetCollectList() {
					if !productionTaskNodes[id] && nodeReady(build.BuildCollect, id) && slices.Contains(productionDefaultNodes[place], id) {
						cost := productionAutoCosts[place]
						coin, energy, exp = coin+cost.coin, energy+cost.energy, exp+cost.exp
						nodes = append(nodes, nodeTake{place, nodeFormula(id)})
						drops = append(drops, productionItemDrops(handProduct(nodeFormula(id)), 1, 0)...)
					}
				}
			}
		}
		var gathers []uint32
		if kind == autoCollectGather || kind == autoCollectBoth {
			for _, gather := range state.wildGathers(true, now) {
				if gather.GetState() != 0 {
					continue
				}
				config := productionGathers[gather.GetId()]
				if len(config.AutoParameters) >= 3 {
					coin, energy, exp = coin+config.AutoParameters[0], energy+config.AutoParameters[1], exp+config.AutoParameters[2]
				}
				gathers = append(gathers, gather.GetId())
				drops = append(drops, gatherDrops(gather.GetId())...)
			}
		}
		if len(nodes) == 0 && len(gathers) == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		cut := uint32(0)
		for _, ship := range ships {
			cut += statGrade(shipStat(ship, 2)).GatherEffect
		}
		need := uint32(math.Floor(float64(energy) * (1 - float64(cut)*0.01)))
		have := uint32(0)
		for _, ship := range ships {
			have += shipEnergy(ship, now)
		}
		if have < need || state.islandItemCount(islandGoldItem) < coin {
			reply.Result = result(ErrNotEnough)
			return nil
		}
		if err := state.takeIslandItem(islandGoldItem, coin); err != nil {
			return err
		}
		left := need
		for _, ship := range ships {
			current := shipEnergy(ship, now)
			taken := min(current, left)
			left -= taken
			gained := uint32(0)
			if energy > 0 {
				gained = uint32(math.Floor(float64(taken) / float64(energy) * float64(exp)))
			}
			setShipEnergy(ship, current-taken, now)
			shipAddExp(ship, gained)
			reply.ShipList = append(reply.ShipList, &protobuf.PB_GATHER_SHIP{
				ShipId: proto.Uint32(ship.GetId()), CurPower: proto.Uint32(current - taken),
				RecoverTime: proto.Uint32(now), AddExp: proto.Uint32(gained),
			})
		}
		for _, place := range []uint32{productionPlaceMine, productionPlaceFelling} {
			count := uint32(0)
			for _, node := range nodes {
				if node.place == place {
					count++
				}
			}
			if count == 0 {
				continue
			}
			build := state.building(place)
			state.takeNodes(place, build.BuildCollect, uint32(len(nodeQueue(build.BuildCollect))), now)
			reply.BuildRefresh = append(reply.BuildRefresh, &protobuf.PB_BUILD_REFRESH{
				BuildId: proto.Uint32(place), RefreshTime: proto.Uint32(build.BuildCollect.GetRefreshTime()),
			})
			for _, node := range nodes {
				if node.place == place {
					if err := tx.handEvents(place, node.formula, 1, nil); err != nil {
						return err
					}
				}
			}
		}
		for _, id := range gathers {
			push, err := state.takeGather(tx, id, now)
			if err != nil {
				return err
			}
			gathered = append(gathered, push)
		}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		for _, drop := range drops {
			if err := tx.islandTaskEvent(islandTaskProdItem, drop.GetNumber(), drop.GetId()); err != nil {
				return err
			}
		}
		reply.Result = result(nil)
		reply.DropList = drops
		reply.GatherList = gathers
		reply.CostGold = proto.Uint32(coin)
		return nil
	})
	if err != nil {
		return err
	}
	if len(gathered) > 0 {
		game.tellIsland(client, client.CommanderID, &protobuf.SC_21528{IslandId: proto.Uint32(client.CommanderID), GatherList: gathered})
	}
	return client.Send(reply)
}

// handleIslandAutoCollectData answers IslandGetAutoCollectionCommand
// (CS_21541, opening the automatic collection panel): every wild resource
// of the commander's island in every scene with its state.
func (game *Game) handleIslandAutoCollectData(client *connection.Client, req *protobuf.CS_21541) error {
	reply := &protobuf.SC_21542{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		reply.Result = result(nil)
		reply.GatherList = state.wildGathers(true, productionNow())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
