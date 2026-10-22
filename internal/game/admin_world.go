package game

// The back-office's Operation Siren details and edits: the world inventory,
// the counters, the achievements and their stars, the port shops and the
// activation. Every edit runs through AdminWrite (the commander is kicked
// first) and keeps the state the way the client's models read it
// (WorldInventoryProxy, WorldStaminaManager, WorldAchievement,
// WorldEntrance:GetAchievementAwards, WorldMapPort).

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// worldSystemSubmarine is WorldConst.SystemSubmarine: the stage that lets
// a submarine fleet deploy (World:BuildFormationIds).
const worldSystemSubmarine = 6

// worldMerchantEvent is the Mysterious Merchant (world_event_data), whose
// shop is the temporary port's (port_camp 0, shopPort).
const worldMerchantEvent = 1901

// worldRegion is a world_regions_data row.
type worldRegion struct {
	Name string `json:"name"`
}

// worldRegions are the world_regions_data regions, read on first use.
var worldRegions = sync.OnceValues(func() (map[uint32]worldRegion, error) {
	return gamedata.Table[worldRegion]("world_regions_data")
})

// WorldDropView is a drop; a world item's (type 12) name and icon are
// filled here, the others' by the caller.
type WorldDropView struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Count  uint32 `json:"count"`
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity"`
}

func worldDropView(kind, id, count uint32) WorldDropView {
	view := WorldDropView{Type: kind, ID: id, Count: count}
	if kind == dropTypeWorldItem {
		label := worldItems[id]
		view.Name, view.Icon, view.Rarity = label.Name, strings.ToLower(label.Icon), label.Rarity
	}
	return view
}

// WorldItemView is an item of the world inventory, or of the catalog.
type WorldItemView struct {
	ID     uint32 `json:"id"`
	Count  uint32 `json:"count"`
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity"`
	Usage  string `json:"usage"`
}

func worldItemView(id, count uint32) WorldItemView {
	label := worldItems[id]
	return WorldItemView{ID: id, Count: count, Name: label.Name, Icon: strings.ToLower(label.Icon), Rarity: label.Rarity, Usage: label.Usage}
}

// WorldTargetView is an achievement (world_target_data) of an entrance.
type WorldTargetView struct {
	ID       uint32 `json:"id"`
	Desc     string `json:"desc"`
	Cryptic  bool   `json:"cryptic"`
	Achieved bool   `json:"achieved"`
	Count    uint32 `json:"count"` // its conditions' counts added up
	Need     uint32 `json:"need"`
}

// WorldStarView is a star of an entrance (target_drop_show).
type WorldStarView struct {
	Star    uint32        `json:"star"`
	Drop    WorldDropView `json:"drop"`
	Earned  bool          `json:"earned"`
	Claimed bool          `json:"claimed"`
}

// WorldEntranceView is an entrance with achievements.
type WorldEntranceView struct {
	ID       uint32            `json:"id"`
	Name     string            `json:"name"`
	Achieved uint32            `json:"achieved"`
	Targets  []WorldTargetView `json:"targets"`
	Stars    []WorldStarView   `json:"stars"`
}

// WorldRegionView is a world_regions_data region's entrances.
type WorldRegionView struct {
	ID        uint32              `json:"id"`
	Name      string              `json:"name"`
	Achieved  uint32              `json:"achieved"`
	Total     uint32              `json:"total"`
	Claimed   uint32              `json:"claimed"`
	Stars     uint32              `json:"stars"`
	Entrances []WorldEntranceView `json:"entrances"`
}

// WorldGoodsView is a goods of a port's offer.
type WorldGoodsView struct {
	ID    uint32        `json:"id"`
	Left  uint32        `json:"left"`
	Max   uint32        `json:"max"`
	Item  WorldDropView `json:"item"`
	Price WorldDropView `json:"price"`
}

// WorldPortView is a port's saved offer.
type WorldPortView struct {
	ID          uint32           `json:"id"`
	Name        string           `json:"name"`
	Goods       []WorldGoodsView `json:"goods"`
	Bought      int              `json:"bought"`
	NextRefresh int64            `json:"next_refresh"`
}

// WorldFleetPick is a fleet of an activation: its ships and meowfficers.
type WorldFleetPick struct {
	Ships      []uint32   `json:"ships"`
	Commanders []worldPos `json:"commanders"`
}

// WorldActivationShip is a ship the activation can deploy.
type WorldActivationShip struct {
	AdminShip
	Team string `json:"team"`
}

// WorldActivationView is what the fleet selection offers a world not
// activated (the WorldMediator entry in controller/const/scene.lua).
type WorldActivationView struct {
	Map         uint32                `json:"map"`
	MapName     string                `json:"map_name"`
	Entrance    uint32                `json:"entrance"`
	Fleets      uint32                `json:"fleets"`
	Submarine   bool                  `json:"submarine"`
	TotalLevel  uint32                `json:"total_level"` // world_fleet_unlock_level: one fleet's levels above it
	Defaults    []WorldFleetPick      `json:"defaults"`
	SubDefaults *WorldFleetPick       `json:"sub_defaults,omitempty"`
	Ships       []WorldActivationShip `json:"ships"`
}

// WorldDetail is the back-office's Operation Siren page.
type WorldDetail struct {
	Started   bool            `json:"started"`
	Activated bool            `json:"activated"`
	Items     []WorldItemView `json:"items"`
	Catalog   []WorldItemView `json:"catalog"`
	CountMax  uint32          `json:"count_max"`
	Counters  struct {
		Exchanges     uint32 `json:"exchanges"`
		ExchangesMax  uint32 `json:"exchanges_max"`
		DailyQueued   int    `json:"daily_queued"`
		DailyLimit    uint32 `json:"daily_limit"`
		DailyRefresh  int64  `json:"daily_refresh"`
		TaskFinish    uint32 `json:"task_finish"`
		TasksDoing    uint32 `json:"tasks_doing"`
		TasksMax      uint32 `json:"tasks_max"`
		Steps         uint32 `json:"steps"`
		Treasures     uint32 `json:"treasures"`
		Cooldowns     int    `json:"cooldowns"`
		NewShopBought uint32 `json:"new_shop_bought"`
		OutShopBought uint32 `json:"out_shop_bought"`
	} `json:"counters"`
	Regions    []WorldRegionView    `json:"regions"`
	Ports      []WorldPortView      `json:"ports"`
	Activation *WorldActivationView `json:"activation,omitempty"`
}

// started is a world activated at least once.
func (state *worldState) started() bool {
	return state.ActivateTime != 0 || state.Count.Activations != 0
}

// WorldDetail is the commander's Operation Siren in detail.
func (game *Game) WorldDetail(ctx context.Context, id uint32) (WorldDetail, error) {
	regions, err := worldRegions()
	if err != nil {
		return WorldDetail{}, err
	}
	now := serverNow()
	state, err := loadWorldState(ctx, game.DB.Queries, id, now)
	if err != nil {
		return WorldDetail{}, err
	}
	detail := WorldDetail{Started: state.started(), Activated: state.activated(), CountMax: math.MaxUint32}
	detail.Items = []WorldItemView{}
	for _, item := range state.Items {
		detail.Items = append(detail.Items, worldItemView(item.ID, item.Count))
	}
	for _, itemID := range slices.Sorted(maps.Keys(worldItems)) {
		detail.Catalog = append(detail.Catalog, worldItemView(itemID, 0))
	}

	counters := &detail.Counters
	counters.Exchanges, counters.ExchangesMax = state.Stamina.Exchanges, uint32(len(worldSettings.supplyPrices))
	counters.DailyQueued, counters.DailyLimit, counters.DailyRefresh = len(state.Daily.Tasks), worldTaskSettings.dailyLimit, int64(state.Daily.Refresh)
	counters.TaskFinish, counters.TasksDoing, counters.TasksMax = state.TaskFinishCount, state.doingTasks(), worldTaskSettings.doingMax
	counters.Steps, counters.Treasures, counters.Cooldowns = state.Count.Step, state.Count.Treasure, len(state.CDs)
	for _, goods := range state.Goods {
		counters.NewShopBought += goods.Count
	}
	for _, goods := range state.OutShopBuys {
		counters.OutShopBought += goods.Count
	}

	detail.Regions = state.regionViews(regions)
	detail.Ports = []WorldPortView{}
	for _, port := range state.Ports {
		name := strings.TrimSpace(worldPorts[port.PortID].Name)
		if worldPorts[port.PortID].PortCamp == 0 {
			name = worldEvents[worldMerchantEvent].Name
		}
		view := WorldPortView{ID: port.PortID, Name: name, Goods: []WorldGoodsView{}, Bought: len(port.Bought), NextRefresh: int64(port.NextRefresh)}
		for _, goods := range port.Goods {
			config := worldPortGoods[goods.ID]
			view.Goods = append(view.Goods, WorldGoodsView{
				ID: goods.ID, Left: goods.Count, Max: max(config.Frequency, 1),
				Item:  worldDropView(config.ItemType, config.ItemID, config.ItemNum),
				Price: worldDropView(config.PriceType, config.PriceID, config.PriceNum),
			})
		}
		detail.Ports = append(detail.Ports, view)
	}
	if !state.activated() {
		if detail.Activation, err = game.worldActivationView(ctx, id, state); err != nil {
			return detail, err
		}
	}
	return detail, nil
}

// regionViews are the entrances with achievements (WorldAtlas'
// achEntranceList) by region, with World:CountAchievements' counts.
func (state *worldState) regionViews(names map[uint32]worldRegion) []WorldRegionView {
	byRegion := map[uint32]*WorldRegionView{}
	for _, entranceID := range slices.Sorted(maps.Keys(worldTargetEntrances)) {
		entrance := worldTargetEntrances[entranceID]
		if len(entrance.NormalTarget)+len(entrance.CrypticTarget) == 0 {
			continue
		}
		colormask := worldColormasks[entranceID]
		region := byRegion[colormask.Regions]
		if region == nil {
			region = &WorldRegionView{ID: colormask.Regions, Name: names[colormask.Regions].Name}
			byRegion[colormask.Regions] = region
		}
		view := WorldEntranceView{ID: entranceID, Name: worldRandomMaps[colormask.Chapter].Name, Targets: []WorldTargetView{}, Stars: []WorldStarView{}}
		for _, targetID := range worldEntranceTargetIDs[entranceID] {
			config := worldTargets[targetID]
			target := WorldTargetView{ID: targetID, Desc: config.TargetDesc, Cryptic: slices.Contains(entrance.CrypticTarget, targetID), Achieved: state.achieved(targetID)}
			progress := state.target(targetID)
			for _, condition := range config.Condition {
				target.Need += condition[1]
				if progress == nil {
					continue
				}
				if i := slices.IndexFunc(progress.Process, func(p worldCount) bool { return p.ID == condition[0] }); i >= 0 {
					target.Count += min(progress.Process[i].Count, condition[1])
				}
			}
			if target.Achieved {
				view.Achieved++
			}
			view.Targets = append(view.Targets, target)
		}
		claimed := state.claimedStars(entranceID)
		for _, star := range slices.Sorted(maps.Keys(entrance.stars)) {
			drop := entrance.stars[star]
			view.Stars = append(view.Stars, WorldStarView{
				Star: star, Drop: worldDropView(drop[0], drop[1], drop[2]),
				Earned: star <= view.Achieved, Claimed: slices.Contains(claimed, star),
			})
		}
		region.Achieved += view.Achieved
		region.Total += uint32(len(view.Targets))
		region.Claimed += uint32(len(claimed))
		region.Stars += uint32(len(view.Stars))
		region.Entrances = append(region.Entrances, view)
	}
	regions := []WorldRegionView{}
	for _, regionID := range slices.Sorted(maps.Keys(byRegion)) {
		regions = append(regions, *byRegion[regionID])
	}
	return regions
}

// claimedStars are the stars of an entrance claimed (target_fetch_list).
func (state *worldState) claimedStars(entranceID uint32) []uint32 {
	for _, fetch := range state.TargetFetches {
		if fetch.ID == entranceID {
			return fetch.Stars
		}
	}
	return nil
}

// entranceAchieved counts an entrance's achievements achieved
// (World:CountAchievements, normal and cryptic).
func (state *worldState) entranceAchieved(entranceID uint32) uint32 {
	n := uint32(0)
	for _, targetID := range worldEntranceTargetIDs[entranceID] {
		if state.achieved(targetID) {
			n++
		}
	}
	return n
}

// worldEdit runs an edit of a world activated at least once.
func (game *Game) worldEdit(ctx context.Context, id uint32, fn func(tx *Tx, state *worldState) error) (bool, error) {
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		state, err := tx.worldState(serverNow())
		if err != nil {
			return err
		}
		if !state.started() {
			return AdminError{Msg: "Operation Siren was never activated"}
		}
		if err := fn(tx, state); err != nil {
			return err
		}
		return tx.saveWorldState(state)
	})
}

// SetWorldItem sets a world item's count, 0 removing it
// (WorldInventoryProxy: any world_item_data_template item, no cap).
func (game *Game) SetWorldItem(ctx context.Context, id, itemID, count uint32) (bool, error) {
	if _, ok := worldItems[itemID]; !ok {
		return false, adminRefused("no world item %d", itemID)
	}
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		state.addItem(itemID, int64(count)-int64(state.item(itemID)))
		return nil
	})
}

// World counters the back-office sets (SetWorldCounter) or resets
// (ResetWorldCounter).
const (
	WorldCounterExchanges  = "exchanges"   // the week's oil exchanges (world_supply_price)
	WorldCounterTaskFinish = "task_finish" // task_finish_count
	WorldCounterSteps      = "steps"       // step_count
	WorldCounterTreasures  = "treasures"   // treasure_count
	WorldCounterDaily      = "daily"       // the daily task queue
	WorldCounterCooldowns  = "cooldowns"   // cd_list
	WorldCounterNewShop    = "new_shop"    // goods_list
	WorldCounterOutShop    = "out_shop"    // out_shop_buy_list
)

// SetWorldCounter sets a counter. The oil exchanges (which
// WorldStaminaManager reads as staminaExchangeTimes) go up to
// world_supply_price's length (GetExchangeData).
func (game *Game) SetWorldCounter(ctx context.Context, id uint32, counter string, value uint32) (bool, error) {
	exchanges := uint32(len(worldSettings.supplyPrices))
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		switch counter {
		case WorldCounterExchanges:
			if value > exchanges {
				return adminRefused("at most %d exchanges a week", exchanges)
			}
			state.Stamina.Exchanges = value
		case WorldCounterTaskFinish:
			state.TaskFinishCount = value
		case WorldCounterSteps:
			state.Count.Step = value
		case WorldCounterTreasures:
			state.Count.Treasure = value
		default:
			return AdminError{Msg: "no counter " + counter}
		}
		return nil
	})
}

// ResetWorldCounter resets a counter: the daily task queue is emptied and
// drawn again when the panel opens next (dailyOffer, the day's tasks), the
// orders' cooldowns end, the new port shop's and the voucher shop's
// purchases start over; the set counters go to 0.
func (game *Game) ResetWorldCounter(ctx context.Context, id uint32, counter string) (bool, error) {
	switch counter {
	case WorldCounterExchanges, WorldCounterTaskFinish, WorldCounterSteps, WorldCounterTreasures:
		return game.SetWorldCounter(ctx, id, counter, 0)
	}
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		switch counter {
		case WorldCounterDaily:
			state.Daily = worldDaily{}
		case WorldCounterCooldowns:
			state.CDs = nil
		case WorldCounterNewShop:
			state.Goods = nil
		case WorldCounterOutShop:
			state.OutShopBuys = nil
		default:
			return AdminError{Msg: "no counter " + counter}
		}
		return nil
	})
}

// targetEntrance is the entrance an achievement is of, 0 for none.
func targetEntrance(targetID uint32) uint32 {
	for entranceID, targets := range worldEntranceTargetIDs {
		if slices.Contains(targets, targetID) {
			return entranceID
		}
	}
	return 0
}

// SetWorldTarget marks an achievement achieved (every condition at its
// count, as worldTargetsUpdated leaves them) or not (its progress gone).
// An achievement isn't unmarked while the entrance's claimed stars would
// outnumber its achievements (the stars are earned by that count).
func (game *Game) SetWorldTarget(ctx context.Context, id, targetID uint32, achieved bool) (bool, error) {
	config, ok := worldTargets[targetID]
	entranceID := targetEntrance(targetID)
	if !ok || len(config.Condition) == 0 || entranceID == 0 {
		return false, adminRefused("no achievement %d", targetID)
	}
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		i := slices.IndexFunc(state.Targets, func(t worldTarget) bool { return t.ID == targetID })
		if !achieved {
			if i < 0 {
				return nil
			}
			if state.achieved(targetID) {
				left := state.entranceAchieved(entranceID) - 1
				for _, star := range state.claimedStars(entranceID) {
					if star > left {
						return adminRefused("star %d is claimed: unclaim it first", star)
					}
				}
			}
			state.Targets = slices.Delete(state.Targets, i, i+1)
			return nil
		}
		target := worldTarget{ID: targetID}
		for _, condition := range config.Condition {
			target.Process = append(target.Process, worldCount{ID: condition[0], Count: condition[1]})
		}
		if i < 0 {
			state.Targets = append(state.Targets, target)
		} else {
			state.Targets[i] = target
		}
		return nil
	})
}

// SetWorldStar marks an entrance's star claimed or not, without its
// reward: a star is claimed once earned (handleWorldAchieve: as many
// achievements achieved).
func (game *Game) SetWorldStar(ctx context.Context, id, entranceID, star uint32, claimed bool) (bool, error) {
	entrance := worldTargetEntrances[entranceID]
	if entrance == nil {
		return false, adminRefused("no entrance %d", entranceID)
	}
	if _, ok := entrance.stars[star]; !ok {
		return false, adminRefused("entrance %d has no star %d", entranceID, star)
	}
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		i := slices.IndexFunc(state.TargetFetches, func(f worldTargetFetch) bool { return f.ID == entranceID })
		if !claimed {
			if i >= 0 {
				fetch := &state.TargetFetches[i]
				fetch.Stars = slices.DeleteFunc(fetch.Stars, func(s uint32) bool { return s == star })
				if len(fetch.Stars) == 0 {
					state.TargetFetches = slices.Delete(state.TargetFetches, i, i+1)
				}
			}
			return nil
		}
		if star > state.entranceAchieved(entranceID) {
			return adminRefused("star %d isn't earned: %d achievements achieved", star, state.entranceAchieved(entranceID))
		}
		if i < 0 {
			state.TargetFetches = append(state.TargetFetches, worldTargetFetch{ID: entranceID})
			i = len(state.TargetFetches) - 1
		}
		if fetch := &state.TargetFetches[i]; !slices.Contains(fetch.Stars, star) {
			fetch.Stars = append(fetch.Stars, star)
			slices.Sort(fetch.Stars)
		}
		return nil
	})
}

// RestockWorldPort refills a port's offer: each goods back to its
// frequency, and no longer counted bought (port.Bought), so later offers
// may draw it again.
func (game *Game) RestockWorldPort(ctx context.Context, id, portID uint32) (bool, error) {
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		port := state.port(portID)
		if port == nil {
			return adminRefused("port %d has no offer", portID)
		}
		for i, goods := range port.Goods {
			port.Goods[i].Count = max(worldPortGoods[goods.ID].Frequency, 1)
			port.Bought = slices.DeleteFunc(port.Bought, func(bought uint32) bool { return bought == goods.ID })
		}
		state.markPort(port)
		return nil
	})
}

// RefreshWorldPort ends a port's offer now: the port draws a new one when
// the client asks next (portOffer, CS_33401).
func (game *Game) RefreshWorldPort(ctx context.Context, id, portID uint32) (bool, error) {
	return game.worldEdit(ctx, id, func(tx *Tx, state *worldState) error {
		port := state.port(portID)
		if port == nil {
			return adminRefused("port %d has no offer", portID)
		}
		port.NextRefresh = 0
		return nil
	})
}

// worldActivationEntry is where the fleet selection activates the world
// (the WorldMediator entry in controller/const/scene.lua): a rookie
// (World:IsRookie, never activated and no progress) on its realm's
// world_default_entrance (WorldConst.GetRealmRookieId), else map 2 through
// entrance 2.
func (state *worldState) worldActivationEntry() (mapID, entranceID uint32) {
	camp := max(state.Camp, 1)
	if state.Count.Activations == 0 && state.Count.Progress == 0 && int(camp) <= len(worldSettings.defaultEntrances) {
		entry := worldSettings.defaultEntrances[camp-1]
		return entry[0], entry[1]
	}
	return 2, 2
}

// worldFleetLevelLimit is world_fleet_unlock_level's fleet_totle_level
// condition (WorldFleetSelectLayer:IsPropertyLimitationSatisfy): at least
// count normal fleets whose ships' levels add up to more than level; 0, 0
// for none.
func worldFleetLevelLimit() (count, level uint32) {
	var conditions [][]json.RawMessage
	if gamedata.GamesetDescription("world_fleet_unlock_level", &conditions) != nil {
		return 0, 0
	}
	for _, condition := range conditions {
		var name string
		if len(condition) == 4 && json.Unmarshal(condition[0], &name) == nil && name == "fleet_totle_level" {
			return rawUint(condition[2]), rawUint(condition[3])
		}
	}
	return 0, 0
}

func (game *Game) worldActivationView(ctx context.Context, id uint32, state *worldState) (*WorldActivationView, error) {
	mapID, entranceID := state.worldActivationEntry()
	_, level := worldFleetLevelLimit()
	view := &WorldActivationView{
		Map: mapID, MapName: worldRandomMaps[mapID].Name, Entrance: entranceID,
		Fleets: worldStageFleets(state.Count.Progress), Submarine: worldSystemOpen(worldSystemSubmarine, state.Count.Progress),
		TotalLevel: level, Defaults: []WorldFleetPick{}, Ships: []WorldActivationShip{},
	}
	ships, err := game.DB.ListShipsByCommander(ctx, id)
	if err != nil {
		return nil, err
	}
	teams := map[uint32]string{}
	for _, ship := range ships {
		teams[ship.ID] = shipTeamOf(ship)
	}
	for _, fleet := range state.DefaultFleets {
		pick := WorldFleetPick{Ships: []uint32{}, Commanders: fleet.Commanders}
		for _, shipID := range fleet.Ships {
			if teams[shipID] != "" {
				pick.Ships = append(pick.Ships, shipID)
			}
		}
		if len(pick.Ships) > 0 && teams[pick.Ships[0]] == teamSubmarine {
			view.SubDefaults = &pick
		} else if uint32(len(view.Defaults)) < view.Fleets {
			view.Defaults = append(view.Defaults, pick)
		}
	}
	for i, ship := range adminShips(ships) {
		view.Ships = append(view.Ships, WorldActivationShip{AdminShip: ship, Team: teams[ships[i].ID]})
	}
	return view, nil
}

// ActivateWorld activates a world as the fleet selection's go button does
// (WorldFleetSelectMediator.OnGO, CS_33101 through handleWorldActivate):
// on the entrance worldActivationEntry picks, for realm 1 (World:GetRealm),
// with as many normal fleets as the progress allows
// (World:BuildFormationIds), each with 1 to 3 vanguard and 1 to 3 main
// ships (WorldFleetSelectLayer:CheckValid), enough of them over the
// world_fleet_unlock_level total, and a submarine fleet of up to 3 once
// SystemSubmarine is open.
func (game *Game) ActivateWorld(ctx context.Context, id uint32, fleets []WorldFleetPick, submarine *WorldFleetPick) (bool, error) {
	now := serverNow()
	state, err := loadWorldState(ctx, game.DB.Queries, id, now)
	if err != nil {
		return false, err
	}
	if state.activated() {
		return false, AdminError{Msg: "the world is activated already"}
	}
	need := worldStageFleets(state.Count.Progress)
	if uint32(len(fleets)) != need {
		return false, adminRefused("the fleet selection takes %d fleets", need)
	}
	ships, err := game.DB.ListShipsByCommander(ctx, id)
	if err != nil {
		return false, err
	}
	byID := map[uint32]uint32{} // ship id -> level
	teams := map[uint32]string{}
	for _, ship := range ships {
		byID[ship.ID], teams[ship.ID] = ship.Level, shipTeamOf(ship)
	}
	minFleets, level := worldFleetLevelLimit()
	over := uint32(0)
	var list []*protobuf.ELITEFLEETINFO
	elite := func(pick WorldFleetPick, order []string) *protobuf.ELITEFLEETINFO {
		info := &protobuf.ELITEFLEETINFO{}
		for _, team := range order {
			for _, shipID := range pick.Ships {
				if teams[shipID] == team {
					info.ShipIdList = append(info.ShipIdList, shipID)
				}
			}
		}
		for _, commander := range pick.Commanders {
			info.Commanders = append(info.Commanders, &protobuf.COMMANDERSINFO{Pos: proto.Uint32(commander.Pos), Id: proto.Uint32(commander.ID)})
		}
		return info
	}
	for n, fleet := range fleets {
		counts := map[string]int{}
		total := uint32(0)
		for _, shipID := range fleet.Ships {
			if _, ok := byID[shipID]; !ok {
				return false, adminRefused("no ship %d", shipID)
			}
			counts[teams[shipID]]++
			total += byID[shipID]
		}
		if counts[teamVanguard] < 1 || counts[teamVanguard] > teamShipMax || counts[teamMain] < 1 || counts[teamMain] > teamShipMax || counts[teamSubmarine] > 0 {
			return false, adminRefused("fleet %d takes 1 to 3 vanguard and 1 to 3 main ships", n+1)
		}
		if total > level {
			over++
		}
		// FormationIds2NetIds: main, then vanguard
		list = append(list, elite(fleet, []string{teamMain, teamVanguard}))
	}
	if over < minFleets {
		return false, adminRefused("%d fleet(s) need ship levels adding up to more than %d", minFleets, level)
	}
	if submarine != nil && len(submarine.Ships) > 0 {
		if !worldSystemOpen(worldSystemSubmarine, state.Count.Progress) {
			return false, AdminError{Msg: "the submarine fleet isn't unlocked"}
		}
		if len(submarine.Ships) > teamShipMax {
			return false, AdminError{Msg: "the submarine fleet takes up to 3 submarines"}
		}
		for _, shipID := range submarine.Ships {
			if teams[shipID] != teamSubmarine {
				return false, adminRefused("ship %d isn't a submarine", shipID)
			}
		}
		list = append(list, elite(*submarine, []string{teamSubmarine}))
	}
	seen := map[uint32]bool{}
	for _, info := range list {
		for _, shipID := range info.ShipIdList {
			if seen[shipID] {
				return false, adminRefused("ship %d is in two fleets", shipID)
			}
			seen[shipID] = true
		}
	}
	mapID, entranceID := state.worldActivationEntry()
	req := &protobuf.CS_33101{
		Id: proto.Uint32(mapID), EnterMapId: proto.Uint32(entranceID),
		EliteFleetList: list, Camp: proto.Uint32(1),
	}
	// the game's own activation, its checks included
	return game.AdminWrite(ctx, id, func(tx *Tx) error {
		_, err := tx.activateWorld(req, serverNow())
		if errors.Is(err, errRefused) {
			return AdminError{Msg: "the game refuses this activation"}
		}
		return err
	})
}
