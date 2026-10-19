package game

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Operation Siren's ports: their shops (CS_33401 / CS_33403), the dock's
// repairs (CS_33407) and the fleet redeployment (CS_33409). Built from the
// client's Lua and the world_* tables alone (no capture has the mode):
// controller/command/world/worldport*command.lua, worldshiprepaircommand.lua,
// worldfleetredeploycommand.lua, view/world/worldportlayer.lua,
// view/ship/dockyardscene.lua (MODE_WORLD), World:CalcRepairCost /
// CalcOrderCost; check on the device first.

// worldGoods is a world_goods_data row (WorldGoods, a port's shop) or a
// world_newshop_data row (WorldNShopCommodity, the new port shop of the
// worlds activated after world_newshop_date).
type worldGoods struct {
	ItemType  uint32 `json:"item_type"`
	ItemID    uint32 `json:"item_id"`
	ItemNum   uint32 `json:"item_num"`
	PriceType uint32 `json:"price_type"`
	PriceID   uint32 `json:"price_id"`
	PriceNum  uint32 `json:"price_num"`
	Frequency uint32 `json:"frequency"` // purchases allowed
	PortID    uint32 `json:"port_id"`   // new shop: the port it is sold in
	UnlockNum uint32 `json:"unlock_num"`
}

// worldPortProgress is a world_port_data row's open_condition.
type worldPortOpen struct {
	OpenCondition [][2]uint32 `json:"open_condition"` // {realm, progress}
}

// worldTreasureEntrance is the part of a world_chapter_colormask row the
// item uses and the new shop read.
type worldTreasureEntrance struct {
	// [coordinates item, map]: the treasure maps a usage_world_map item
	// leads to (WorldInventoryMediator.OnMap)
	TeasureChapter [][2]uint32 `json:"teasure_chapter"`
}

// WorldConst.OpReqRedeploy, the cd_list id of the redeployment.
const worldOpRedeploy = 23

// WorldItem.MoneyId, the Operation Supply Coins repairs are paid in.
const worldItemMoney = 100

var (
	worldPortGoods map[uint32]worldGoods // world_goods_data
	worldNewGoods  map[uint32]worldGoods // world_newshop_data
	worldPortOpens map[uint32]worldPortOpen
	worldEntrances map[uint32]worldTreasureEntrance

	// worldGoodsGroups are the world_goods_data ids by group (id / 100: the
	// same goods at a few prices or counts), for the shops of the ports of
	// a realm (ids below worldTempGoods) and of the temporary ports
	// (port_camp 0, the "mingshi" shop of WorldPortLayer:GetPaintingInfo).
	worldGoodsGroups, worldTempGoodsGroups [][]uint32

	worldPortSettings struct {
		// world_port_service_1_interval / _2_interval: {level difference,
		// rate} (World:CalcRepairCost)
		repairRates, brokenRates [][2]float64
		// world_port_service_1_price: {hp_rant up to, price}
		repairPrices [][2]uint32
		brokenPrice  uint32 // world_port_service_2_price
		// world_fleet_redeploy_cost: {cost, cooldown seconds}
		redeployCost [2]uint32
		// world_newshop_date: worlds activated from then on use the new
		// port shop (World:UsePortNShop)
		newShopDate time.Time
		brokenBuff  uint32 // world_death_buff, WorldConst.BrokenBuffId
	}
)

// worldTempGoods is the first world_goods_data id of the temporary ports'
// goods: ids 200000 and up are the cheaper copies of the realm ports' AP
// refills, repair packs, tuning samples and tokens.
const worldTempGoods = 200000

// loadWorldPortData reads the ports' tables.
func loadWorldPortData() error {
	var gameset map[string]uint32
	var newShopDate []uint32
	err := gamedata.LoadAll(
		gamedata.Load("world_goods_data", &worldPortGoods),
		gamedata.Load("world_newshop_data", &worldNewGoods),
		gamedata.Load("world_port_data", &worldPortOpens),
		gamedata.Load("world_chapter_colormask", &worldEntrances),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
		func() error {
			return gamedata.GamesetDescription("world_port_service_1_interval", &worldPortSettings.repairRates)
		},
		func() error {
			return gamedata.GamesetDescription("world_port_service_2_interval", &worldPortSettings.brokenRates)
		},
		func() error {
			return gamedata.GamesetDescription("world_port_service_1_price", &worldPortSettings.repairPrices)
		},
		func() error {
			return gamedata.GamesetDescription("world_fleet_redeploy_cost", &worldPortSettings.redeployCost)
		},
		func() error { return gamedata.GamesetDescription("world_newshop_date", &newShopDate) },
	)
	if err != nil {
		return err
	}
	if len(worldPortSettings.repairRates) == 0 || len(worldPortSettings.brokenRates) == 0 || len(worldPortSettings.repairPrices) == 0 {
		return gamedata.Missing("gameset", "world_port_service_*")
	}
	if len(newShopDate) != 3 {
		return gamedata.Missing("gameset", "world_newshop_date")
	}
	worldPortSettings.newShopDate = time.Date(int(newShopDate[0]), time.Month(newShopDate[1]), int(newShopDate[2]), 0, 0, 0, 0, gamedata.Zone)
	worldPortSettings.brokenPrice = gameset["world_port_service_2_price"]
	worldPortSettings.brokenBuff = gameset["world_death_buff"]
	if worldPortSettings.brokenPrice == 0 || worldPortSettings.brokenBuff == 0 {
		return gamedata.Missing("gameset", "world_port_service_2_price / world_death_buff")
	}
	groups := map[uint32][]uint32{}
	for id := range worldPortGoods {
		groups[id/100] = append(groups[id/100], id)
	}
	worldGoodsGroups, worldTempGoodsGroups = nil, nil
	for _, group := range slices.Sorted(maps.Keys(groups)) {
		ids := groups[group]
		slices.Sort(ids)
		if ids[0] >= worldTempGoods {
			worldTempGoodsGroups = append(worldTempGoodsGroups, ids)
		} else {
			worldGoodsGroups = append(worldGoodsGroups, ids)
		}
	}
	return nil
}

// activePort is the port of the world's active map (WorldMap:GetPort), 0
// for none.
func (state *worldState) activePort() uint32 {
	template := worldTemplates[state.mapTemplate(state.MapID)]
	if !state.activated() || template == nil {
		return 0
	}
	return template.port
}

// port is the saved offer of a port, nil for none yet.
func (state *worldState) port(id uint32) *worldPortState {
	for _, port := range state.Ports {
		if port.PortID == id {
			return port
		}
	}
	return nil
}

// markPort keeps port_list (the ports with goods left) up to date for a
// port's offer.
func (state *worldState) markPort(port *worldPortState) {
	left := slices.ContainsFunc(port.Goods, func(goods worldCount) bool { return goods.Count > 0 })
	i := slices.Index(state.PortMarks, port.PortID)
	switch {
	case left && i < 0:
		state.PortMarks = append(state.PortMarks, port.PortID)
	case !left && i >= 0:
		state.PortMarks = slices.Delete(state.PortMarks, i, i+1)
	}
}

// usesNewShop is World:UsePortNShop: a world reset at least once and
// activated after world_newshop_date sells world_newshop_data instead of
// its ports' goods.
func (state *worldState) usesNewShop() bool {
	activations := state.Count.Activations
	if state.activated() {
		activations--
	}
	return activations > 0 && int64(state.ActivateTime) >= worldPortSettings.newShopDate.Unix()
}

// portOffer is a port's offer at now: the one saved until its
// next_refresh_time, then a new one. The tables don't say which goods a
// port sells (the official server picks them): each port offers one goods
// of every world_goods_data group, drawn for the commander, the port and
// the day, temporary ports (port_camp 0) from the groups of ids 200000 and
// up. Offers refresh at the server's midnight, when the client asks again
// anyway (WorldMapPort:IsValid's zeroHourTime). Its tasks (task_list) are
// the world's portTasks at the time.
func (state *worldState) portOffer(commanderID, portID uint32, now time.Time) *worldPortState {
	port := state.port(portID)
	if port == nil {
		port = &worldPortState{PortID: portID}
		state.Ports = append(state.Ports, port)
	}
	port.Tasks = state.portTasks(portID)
	if port.NextRefresh > uint32(now.Unix()) {
		return port
	}
	day := startOfDay(now).Unix()
	rng := rand.New(rand.NewPCG(uint64(commanderID)<<32|uint64(portID), uint64(day)))
	groups := worldGoodsGroups
	if worldPorts[portID].PortCamp == 0 {
		groups = worldTempGoodsGroups
	}
	port.Goods = nil
	for _, ids := range groups {
		id := ids[rng.IntN(len(ids))]
		port.Goods = append(port.Goods, worldCount{ID: id, Count: max(worldPortGoods[id].Frequency, 1)})
	}
	port.NextRefresh = uint32(nextDay(now).Unix())
	state.markPort(port)
	return port
}

// portInfo is the offer as PORT_INFO.
func (port *worldPortState) portInfo() *protobuf.PORT_INFO {
	return &protobuf.PORT_INFO{
		PortId:          proto.Uint32(port.PortID),
		TaskList:        port.Tasks,
		GoodsList:       worldGoodsInfos(port.Goods),
		NextRefreshTime: proto.Uint32(port.NextRefresh),
	}
}

// handleWorldPortReq answers WorldPortReqCommand (CS_33401, WorldPortLayer
// as it opens and once the offer is past its refresh time): the port of the
// active map map_id, its tasks and goods (WorldProxy:NetUpdateMapPort). The
// client then clears the port's new mark (UpdatePortMark(id, nil, false));
// port_id 0 is its "port req error." tip.
func (game *Game) handleWorldPortReq(client *connection.Client, req *protobuf.CS_33401) error {
	ctx := context.Background()
	now := serverNow()
	var info *protobuf.PORT_INFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		portID := state.activePort()
		if req.GetMapId() != state.MapID || portID == 0 {
			return errRefused
		}
		if _, ok := worldPorts[portID]; !ok {
			return errRefused
		}
		port := state.portOffer(tx.commanderID, portID, now)
		if i := slices.Index(state.NewPortMarks, portID); i >= 0 {
			state.NewPortMarks = slices.Delete(state.NewPortMarks, i, i+1)
		}
		info = port.portInfo()
		return tx.saveWorldState(state)
	})
	if refused(err) {
		info = &protobuf.PORT_INFO{PortId: proto.Uint32(0), NextRefreshTime: proto.Uint32(0)}
	} else if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_33402{Port: info})
}

// CS_33403 shop types.
const (
	worldShopPort = 1 // WorldPortShoppingCommand, world_goods_data
	worldShopNew  = 2 // WorldPortNewShoppingCommand, world_newshop_data
)

// handleWorldPortShopping answers WorldPortShoppingCommand (shop_type 1,
// count 1, a goods of the active map's port) and WorldPortNewShoppingCommand
// (shop_type 2, count of a world_newshop_data goods, up to its frequency
// over the world's goods_list, once unlocked by unlock_num zones secured
// without a port, WorldAtlas:GetPressingUnlockCount). The client takes the
// price itself (reducePlayerOwn: world items from the world inventory) and
// adds drop_list (PlayerConst.addTranDrop: world items into the world
// inventory too); errors show errorTip("world_port_shopping_error_", result).
func (game *Game) handleWorldPortShopping(client *connection.Client, req *protobuf.CS_33403) error {
	ctx := context.Background()
	now := serverNow()
	var drops []*protobuf.DROPINFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		count := req.GetCount()
		if !state.activated() || count == 0 {
			return errRefused
		}
		var goods worldGoods
		switch req.GetShopType() {
		case worldShopPort:
			portID := state.activePort()
			port := state.port(portID)
			if portID == 0 || port == nil || port.NextRefresh <= uint32(now.Unix()) {
				return errRefused
			}
			i := slices.IndexFunc(port.Goods, func(g worldCount) bool { return g.ID == req.GetShopId() })
			if i < 0 || port.Goods[i].Count < count {
				return errRefused
			}
			goods = worldPortGoods[req.GetShopId()]
			port.Goods[i].Count -= count
			state.markPort(port)
		case worldShopNew:
			var ok bool
			if goods, ok = worldNewGoods[req.GetShopId()]; !ok || goods.UnlockNum > state.pressingUnlockCount() {
				return errRefused
			}
			bought := uint32(0)
			i := slices.IndexFunc(state.Goods, func(g worldCount) bool { return g.ID == req.GetShopId() })
			if i >= 0 {
				bought = state.Goods[i].Count
			}
			if bought+count > goods.Frequency {
				return errRefused
			}
			if i < 0 {
				state.Goods = append(state.Goods, worldCount{ID: req.GetShopId(), Count: count})
			} else {
				state.Goods[i].Count += count
			}
		default:
			return errRefused
		}
		if goods.ItemNum == 0 {
			return errRefused
		}
		if err := tx.spendWorldPrice(state, goods.PriceType, goods.PriceID, goods.PriceNum*count); err != nil {
			return err
		}
		drops = []*protobuf.DROPINFO{dropInfo(goods.ItemType, goods.ItemID, goods.ItemNum*count)}
		if err := tx.grantWorldDrops(state, drops); err != nil {
			return err
		}
		return tx.saveWorldState(state)
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_33404{Result: result(err), DropList: drops})
}

// spendWorldPrice pays a price: world items (drop type 12) from the world
// inventory, player resources from the commander's.
func (tx *Tx) spendWorldPrice(state *worldState, kind, id, number uint32) error {
	switch kind {
	case dropTypeWorldItem:
		if state.item(id) < number {
			return ErrNotEnough
		}
		state.addItem(id, -int64(number))
		return nil
	case dropTypeResource:
		return tx.SpendResource(id, number)
	}
	return errRefused
}

// pressingUnlockCount is WorldAtlas:GetPressingUnlockCount: the zones
// secured (clean_chapter) whose entrance has no port.
func (state *worldState) pressingUnlockCount() uint32 {
	n := uint32(0)
	for _, mapID := range state.CleanChapters {
		for _, entrance := range worldColormasks {
			if entrance.Chapter == mapID && entrance.PortMapIcon == 0 {
				n++
				break
			}
		}
	}
	return n
}

// worldShipLevel is a ship's level, 0 for a ship the commander hasn't.
func (tx *Tx) worldShipLevel(id uint32) (uint32, error) {
	ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: id, CommanderID: tx.commanderID})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return ship.Level, err
}

// worldLevel is World:GetLevel: the highest of the fleets' average ship
// levels (WorldBaseFleet:GetLevel, floored).
func (tx *Tx) worldLevel(state *worldState) (float64, error) {
	best := 0.0
	for _, fleet := range state.Fleets {
		if len(fleet.Ships) == 0 {
			continue
		}
		total := uint32(0)
		for _, ship := range fleet.Ships {
			level, err := tx.worldShipLevel(ship.ID)
			if err != nil {
				return 0, err
			}
			total += level
		}
		best = max(best, math.Floor(float64(total)/float64(len(fleet.Ships))))
	}
	return best, nil
}

// broken tells whether a world ship has the broken buff (WorldMapShip:IsBroken).
func (ship *worldShip) broken() bool {
	return slices.ContainsFunc(ship.Buffs, func(buff worldBuff) bool {
		return buff.ID == worldPortSettings.brokenBuff && buff.Stack > 0
	})
}

// worldRate is the rate of the first {up to, rate} entry diff is within,
// the last one past them all (World:CalcRepairCost's _.detect).
func worldRate(rates [][2]float64, diff float64) float64 {
	for _, rate := range rates {
		if diff <= rate[0] {
			return rate[1]
		}
	}
	return rates[len(rates)-1][1]
}

// repairCost is World:CalcRepairCost: a broken ship costs the broken rate
// of its level above the world's times world_port_service_2_price, a
// damaged one the repair rate times the price of its hp bracket; a ship at
// full hp nothing.
func repairCost(ship *worldShip, level, worldLevel float64) float64 {
	diff := level - worldLevel
	switch {
	case ship.broken():
		return worldRate(worldPortSettings.brokenRates, diff) * float64(worldPortSettings.brokenPrice)
	case ship.HPRant < 10000:
		prices := worldPortSettings.repairPrices
		price := prices[len(prices)-1][1]
		for _, entry := range prices {
			if ship.HPRant <= entry[0] {
				price = entry[1]
				break
			}
		}
		return worldRate(worldPortSettings.repairRates, diff) * float64(price)
	}
	return 0
}

// worldShip finds a ship of the deployed fleets, nil for none.
func (state *worldState) worldShip(id uint32) *worldShip {
	for _, fleet := range state.Fleets {
		for i := range fleet.Ships {
			if fleet.Ships[i].ID == id {
				return &fleet.Ships[i]
			}
		}
	}
	return nil
}

// handleWorldShipRepair answers WorldShipRepairCommand (CS_33407, the port's
// dock, DockyardScene MODE_WORLD): repair ship_list to full hp and lift
// their broken buff (WorldMapShip:Repair) for the sum of their
// World:CalcRepairCost in Operation Supply Coins, which the client takes
// itself (RemoveItem(WorldItem.MoneyId, totalCost)). Errors show
// errorTip("world_ship_repair_err_", result).
func (game *Game) handleWorldShipRepair(client *connection.Client, req *protobuf.CS_33407) error {
	ctx := context.Background()
	now := serverNow()
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		if state.activePort() == 0 || len(req.GetShipList()) == 0 {
			return errRefused
		}
		worldLevel, err := tx.worldLevel(state)
		if err != nil {
			return err
		}
		total := 0.0
		for i, id := range req.GetShipList() {
			ship := state.worldShip(id)
			if ship == nil || slices.Contains(req.GetShipList()[:i], id) {
				return errRefused
			}
			level, err := tx.worldShipLevel(id)
			if err != nil {
				return err
			}
			total += repairCost(ship, float64(level), worldLevel)
			ship.HPRant = 10000
			ship.Buffs = slices.DeleteFunc(ship.Buffs, func(buff worldBuff) bool { return buff.ID == worldPortSettings.brokenBuff })
		}
		cost := uint32(math.Floor(total))
		if state.item(worldItemMoney) < cost {
			return ErrNotEnough
		}
		state.addItem(worldItemMoney, -int64(cost))
		return tx.saveWorldState(state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_33408{Result: result(err)})
}

// redeployCost is World:CalcOrderCost(OpReqRedeploy) (CalcCDTimeCost):
// world_fleet_redeploy_cost[1] action points, less in proportion to the
// part of its cooldown elapsed since the last redeployment.
func (state *worldState) redeployCost(now time.Time) uint32 {
	return worldCDCost(worldPortSettings.redeployCost, state.lastCD(worldOpRedeploy), now.Unix())
}

// handleWorldFleetRedeploy answers WorldFleetRedeployCommand (CS_33409, the
// fleet selection opened by the redeploy order in a port of the world's
// realm, SVOrderPanel / WSCommand:OpRedeploy, never with an Energy Matrix
// carried): the fleets become elite_fleet_list (the activation's rules,
// worldFleets) at full hp and resolve ("HP and Resolve have been restored")
// for World:CalcOrderCost(OpReqRedeploy) action points, which the client
// takes itself, and the order's cooldown starts (SetReqCDTime). The new
// fleets take the old ones' cells in order, the extra ones the active
// fleet's; the first of group_list becomes the active fleet. Errors show
// errorTip("world_fleet_redeploy_error_", result).
func (game *Game) handleWorldFleetRedeploy(client *connection.Client, req *protobuf.CS_33409) error {
	ctx := context.Background()
	now := serverNow()
	var groups []*protobuf.GROUPINCHAPTER_P33
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		portID := state.activePort()
		if portID == 0 || worldPorts[portID].PortCamp != state.Camp {
			return errRefused
		}
		var old []*worldFleet
		for _, fleet := range state.Fleets {
			if len(fleet.Carries) > 0 {
				return errRefused
			}
			if !fleet.Submarine {
				old = append(old, fleet)
			}
		}
		if len(old) == 0 {
			return errRefused
		}
		fleets, err := tx.worldFleets(req.GetEliteFleetList(), state.Count.Progress)
		if err != nil {
			return err
		}
		if !state.Stamina.consume(state.redeployCost(now)) {
			return ErrNotEnough
		}
		normal := 0
		for _, fleet := range fleets {
			if fleet.Submarine {
				continue
			}
			from := old[0]
			if normal < len(old) {
				from = old[normal]
			}
			fleet.Row, fleet.Column = from.Row, from.Column
			fleet.StartRow, fleet.StartCol = from.StartRow, from.StartCol
			normal++
		}
		state.Fleets = fleets
		state.DefaultFleets = nil
		for _, fleet := range fleets {
			base := worldBaseFleet{ID: fleet.ID, Commanders: fleet.Commanders}
			for _, ship := range fleet.Ships {
				base.Ships = append(base.Ships, ship.ID)
			}
			state.DefaultFleets = append(state.DefaultFleets, base)
			groups = append(groups, fleet.groupInfo())
		}
		state.setCD(worldOpRedeploy, now)
		if err := tx.worldTasksUpdated(state, &worldHappenings{}); err != nil { // fleet expansion tasks
			return err
		}
		return tx.saveWorldState(state)
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		groups = nil
	}
	return client.Send(&protobuf.SC_33410{Result: result(err), GroupList: groups})
}
