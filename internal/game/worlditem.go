package game

import (
	"context"
	"encoding/json"
	"maps"
	"math"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Operation Siren's inventory: using world items (CS_33301). Built from the
// client's Lua and the world_* tables alone (no capture has the mode):
// controller/command/world/inventory/worlditemusecommand.lua,
// view/world/worldinventorylayer.lua, worldallocatelayer.lua,
// worldassigneditemview.lua, WorldMediator's WORLD_ITEM_USE_DONE; check on
// the device first.

// world_item_data_template usages (WorldItem.Usage*) the client sends
// CS_33301 for. usage_world_map (coordinates, WorldInventoryMediator.OnMap)
// and usage_undefined (coins, crates) are never used through it.
const (
	worldUsageBuff          = "usage_world_buff"          // [quota, ship buff]: a buff stack per item on each ship picked
	worldUsageHeal          = "usage_world_healing"       // [quota, hp_rant]: hp in 1/10000 per item
	worldUsageHealValue     = "usage_world_healing_value" // [quota, hp]: hp points per item
	worldUsageRecoverAP     = "usage_world_recoverAP"     // [points]: action points per item
	worldUsageDrop          = "usage_drop"                // a server-side drop table id
	worldUsageDropAppointed = "usage_drop_appointed"      // [[type, id, count]...]: the one picked
	worldUsageClean         = "usage_world_clean"         // every zone securing reward at once
	worldUsageSLGBuff       = "usage_worldSLGbuff"        // [global buff, floors] per item
	worldUsageFlag          = "usage_world_flag"          // [World.gobalFlag index]
)

// worldItemConfig is a world_item_data_template row.
type worldItemConfig struct {
	Name     string          `json:"name"`
	Rarity   uint32          `json:"rarity"`
	Icon     string          `json:"icon"`
	Display  string          `json:"display"`
	Usage    string          `json:"usage"`
	UsageArg json.RawMessage `json:"usage_arg"`
	// what the monthly reset turns it into: every TransformNum held give
	// TransformNumber of {TransformType, TransformID} (Supply Shop
	// Vouchers, resource 3002, for most; worldResetDrops)
	TransformType   uint32 `json:"item_transform_item_type"`
	TransformID     uint32 `json:"item_transform_item_id"`
	TransformNum    uint32 `json:"item_transform_num"`
	TransformNumber uint32 `json:"item_transform_item_number"`
}

// worldEventComplete is a world_event_complete row: a zone securing reward
// (CHAPTERAWARDINFO.award).
type worldEventComplete struct {
	RewardItems [][]uint32 `json:"event_reward_item"`    // [type, id, count]
	RewardBuff  []uint32   `json:"event_reward_slgbuff"` // [global buff, floors]
}

// iconItem is the part of an item_data_statistics or
// item_virtual_data_statistics row a world item is matched on: its icon,
// and what a container lists (a bag box's usage_drop_appointed usage_arg,
// a virtual container's display_icon).
type iconItem struct {
	Icon        string          `json:"icon"`
	Usage       string          `json:"usage"`
	UsageArg    json.RawMessage `json:"usage_arg"`
	DisplayIcon json.RawMessage `json:"display_icon"`
}

// containerContents reads a container's list of [type, id, count], nil when raw
// isn't one.
func containerContents(raw json.RawMessage) []dropRef {
	var drops [][]uint32
	if json.Unmarshal(raw, &drops) != nil {
		return nil
	}
	var pool []dropRef
	for _, drop := range drops {
		if len(drop) >= 2 {
			pool = append(pool, dropRef{drop[0], drop[1]})
		}
	}
	return pool
}

// worldResource is WorldConst.ResourceID, the Supply Shop Vouchers (a
// player resource) the monthly reset turns world items into.
const worldResource = 3002

var (
	// worldResourceMax is gameset world_resource_max, the most vouchers a
	// commander holds
	worldResourceMax uint32

	worldItems          map[uint32]worldItemConfig
	worldEventCompletes map[uint32]worldEventComplete

	// usage_drop's drop tables are the server's; what an item gives is read
	// from its icon, an asset name every region shares (the names and
	// texts are translated): the bag item it is (Props/<id>: Nanoceramic
	// Alloy, T1 Ordnance Testing Report...), else the container of the same
	// icon (T1 Development Materials Container, Universal T4 Part), else a
	// coordinate logger (worldLoggerIcon).
	worldBagIDs         map[uint32]bool // item_data_statistics ids whose icon is Props/<id>
	worldContainerIcons map[string][]dropRef
)

// worldLoggerIcon reads a coordinate logger's icon: Wluopanputong2..6 the
// R2..R6 loggers, Wsairenluopanputong4..6 the Siren ones, without a digit
// the ones whose display text gives the corrosion level, or range ("a
// Corrosion level of 4-6", "侵蚀等级4-6"): it opens into one of the
// coordinates items (usage_world_map, Siren ones with usage_arg [1]) of the
// treasure maps (world_chapter_colormask teasure_chapter) of that hazard
// level. Nothing structured gives the digit-less ones' levels: the client
// sends usage_drop items as is (WorldItem.UsageDrop,
// mod/experiment/world/model/worlditem.lua) and their usage_arg
// (1000021-1000023) is a server drop table no table holds (world_event_data
// 1000021 is an unrelated event); their other fields (rarity, scrap value)
// don't follow the levels, and the tables naming them (item 99985,
// world_task_data 219400-219600) only hand them out. So the digits of the
// text, which every region writes the same way.
var (
	worldLoggerIcon  = regexp.MustCompile(`^Props/W(sairen)?luopanputong(\d*)$`)
	worldLoggerLevel = regexp.MustCompile(`(\d+)(?:-(\d+))?`)
)

// worldLoggerLevels overrides a logger's levels where its text is wrong:
// the wiki's Operation Siren page has Coordinate Logger 2100001, whose EN
// text says "Corrosion level of 4", open R3 and R4 zones, as the other
// regions' texts say (3-4).
var worldLoggerLevels = map[uint32][2]uint64{2100001: {3, 4}}

// loggerLevels are the hazard levels coordinate logger id opens (match is
// its worldLoggerIcon match).
func loggerLevels(id uint32, item worldItemConfig, match []string) (low, high uint64) {
	if match[2] != "" {
		low, _ = strconv.ParseUint(match[2], 10, 32)
		return low, low
	}
	if levels, ok := worldLoggerLevels[id]; ok {
		return levels[0], levels[1]
	}
	text := worldLoggerLevel.FindStringSubmatch(item.Display)
	if text == nil {
		return 0, 0
	}
	low, _ = strconv.ParseUint(text[1], 10, 32)
	high = low
	if text[2] != "" {
		high, _ = strconv.ParseUint(text[2], 10, 32)
	}
	return low, high
}

// loadWorldItemData reads the world items' tables.
func loadWorldItemData() error {
	var bag, virtual map[uint32]iconItem
	var gameset map[string]uint32
	err := gamedata.LoadAll(
		gamedata.Load("world_item_data_template", &worldItems),
		gamedata.Load("world_event_complete", &worldEventCompletes),
		gamedata.Load("item_data_statistics", &bag),
		gamedata.Load("item_virtual_data_statistics", &virtual),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
	)
	if err != nil {
		return err
	}
	if worldResourceMax = gameset["world_resource_max"]; worldResourceMax == 0 {
		return gamedata.Missing("gameset", "world_resource_max")
	}
	worldBagIDs, worldContainerIcons = map[uint32]bool{}, map[string][]dropRef{}
	// the virtual containers first, then the bag boxes, the lowest id of an
	// icon kept
	for _, id := range slices.Sorted(maps.Keys(virtual)) {
		if pool := containerContents(virtual[id].DisplayIcon); len(pool) > 0 && worldContainerIcons[virtual[id].Icon] == nil {
			worldContainerIcons[virtual[id].Icon] = pool
		}
	}
	for _, id := range slices.Sorted(maps.Keys(bag)) {
		row := bag[id]
		if row.Icon == "Props/"+strconv.FormatUint(uint64(id), 10) {
			worldBagIDs[id] = true
		}
		if row.Usage != "usage_drop_appointed" || worldContainerIcons[row.Icon] != nil {
			continue
		}
		if pool := containerContents(row.UsageArg); len(pool) > 0 {
			worldContainerIcons[row.Icon] = pool
		}
	}
	return nil
}

// worldItemArgs is a usage_arg read as numbers.
func (item worldItemConfig) args() []uint32 {
	return gamedata.IDs(item.UsageArg)
}

// worldItemDrops draws what count of usage_drop world item id give, nil when
// it can't be read (worldBagIDs...).
func worldItemDrops(id uint32, item worldItemConfig, count uint32) []*protobuf.DROPINFO {
	var pool []dropRef
	if bagID, ok := strings.CutPrefix(item.Icon, "Props/"); ok {
		if bagID, err := strconv.ParseUint(bagID, 10, 32); err == nil && worldBagIDs[uint32(bagID)] {
			return []*protobuf.DROPINFO{dropInfo(dropTypeItem, uint32(bagID), count)}
		}
	}
	switch match := worldLoggerIcon.FindStringSubmatch(item.Icon); {
	case match != nil:
		low, high := loggerLevels(id, item, match)
		siren := match[1] != ""
		for _, entrance := range worldEntrances {
			for _, treasure := range entrance.TeasureChapter {
				coordinates, ok := worldItems[treasure[0]]
				if !ok || coordinates.Usage != "usage_world_map" || (len(coordinates.args()) > 0) != siren {
					continue
				}
				if row, ok := worldRandomMaps[treasure[1]]; ok && uint64(row.HazardLevel) >= low && uint64(row.HazardLevel) <= high {
					ref := dropRef{dropTypeWorldItem, treasure[0]}
					if !slices.Contains(pool, ref) {
						pool = append(pool, ref)
					}
				}
			}
		}
	default:
		pool = slices.Clone(worldContainerIcons[item.Icon])
	}
	if len(pool) == 0 {
		return nil
	}
	slices.SortFunc(pool, func(a, b dropRef) int { return int(a.id) - int(b.id) })
	var drops []*protobuf.DROPINFO
	for range count {
		ref := pool[rand.IntN(len(pool))]
		drops = mergeDrop(drops, ref.kind, ref.id, 1)
	}
	return drops
}

// grantWorldDrops gives drops: world items (drop type 12) into the world
// inventory, everything else through grantDrops.
func (tx *Tx) grantWorldDrops(state *worldState, drops []*protobuf.DROPINFO) error {
	var others []*protobuf.DROPINFO
	for _, drop := range drops {
		if drop.GetType() == dropTypeWorldItem {
			state.addItem(drop.GetId(), int64(drop.GetNumber()))
		} else {
			others = append(others, drop)
		}
	}
	if len(others) == 0 {
		return nil
	}
	return tx.grantDrops(others)
}

// addWorldBuff adds floors to a buff of a list (WorldBuff:AddFloor, 0 to
// 999), dropping it at 0.
func addWorldBuff(buffs []worldBuff, id uint32, floors int64) []worldBuff {
	i := slices.IndexFunc(buffs, func(buff worldBuff) bool { return buff.ID == id })
	if i < 0 {
		buffs = append(buffs, worldBuff{ID: id})
		i = len(buffs) - 1
	}
	stack := min(max(int64(buffs[i].Stack)+floors, 0), 999)
	if stack == 0 {
		return slices.Delete(buffs, i, i+1)
	}
	buffs[i].Stack = uint32(stack)
	return buffs
}

// worldShipMaxHP is a ship's durability at its level, without gear or
// bonuses (the client's getProperties adds them: WorldMapShip:
// RegenerateValue's hp_rant is an estimate here).
func (tx *Tx) worldShipMaxHP(id uint32) (float64, error) {
	ship, err := tx.GetShip(tx.ctx, db.GetShipParams{ID: id, CommanderID: tx.commanderID})
	if err != nil {
		return 0, err
	}
	return guildShipAttr(db.GuildOperationShip{TemplateID: ship.TemplateID, Level: ship.Level}, 1), nil
}

// handleWorldItemUse answers WorldItemUseCommand (CS_33301): use count of a
// world item, arg naming the ships (buff and repair items, picked in
// WorldAllocateLayer up to usage_arg[1] of them) or the pick of a selection
// (usage_drop_appointed, the [type, id, count] entry). On result 0 the
// client removes the items, then by usage: adds the ship buffs, heals the
// ships, adds the action points (ExchangeStamina, past the max), sets the
// global flag (usage_world_flag 1: treasure_flag) or adds the global buff
// (WorldMediator), claims every zone securing reward (usage_world_clean,
// WorldScene:GetAllPessingAward: the rewards' slgbuffs; their zones count
// as secured from then on), and adds drop_list
// (PlayerConst.addTranDrop, world items into the world inventory) for the
// others. Errors show "Operation Siren item usage failure:" result.
func (game *Game) handleWorldItemUse(client *connection.Client, req *protobuf.CS_33301) error {
	ctx := context.Background()
	now := serverNow()
	var drops []*protobuf.DROPINFO
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.worldState(now)
		if err != nil {
			return err
		}
		id, count, arg := req.GetId(), req.GetCount(), req.GetArg()
		item, ok := worldItems[id]
		if !state.activated() || !ok || count == 0 {
			return errRefused
		}
		if state.item(id) < count {
			return ErrNotEnough
		}
		args := item.args()
		ships := func() ([]*worldShip, error) {
			if len(args) < 2 || len(arg) == 0 || uint32(len(arg)) > args[0] {
				return nil, errRefused
			}
			var list []*worldShip
			for i, shipID := range arg {
				ship := state.worldShip(shipID)
				if ship == nil || slices.Contains(arg[:i], shipID) {
					return nil, errRefused
				}
				list = append(list, ship)
			}
			return list, nil
		}
		switch item.Usage {
		case worldUsageBuff:
			list, err := ships()
			if err != nil {
				return err
			}
			for _, ship := range list {
				ship.Buffs = addWorldBuff(ship.Buffs, args[1], int64(count))
			}
		case worldUsageHeal:
			list, err := ships()
			if err != nil {
				return err
			}
			for _, ship := range list {
				ship.HPRant = min(10000, ship.HPRant+args[1]*count)
			}
		case worldUsageHealValue:
			list, err := ships()
			if err != nil {
				return err
			}
			for _, ship := range list {
				hp, err := tx.worldShipMaxHP(ship.ID)
				if err != nil {
					return err
				}
				if hp <= 0 {
					return errRefused
				}
				ship.HPRant = uint32(min(10000, float64(ship.HPRant)+math.Floor(float64(args[1]*count)/hp*10000)))
			}
		case worldUsageRecoverAP:
			if len(args) == 0 {
				return errRefused
			}
			state.Stamina.Power += args[0] * count
		case worldUsageSLGBuff:
			if len(args) < 2 {
				return errRefused
			}
			state.Buffs = addWorldBuff(state.Buffs, args[0], int64(args[1]*count))
		case worldUsageFlag:
			if len(args) == 0 || args[0] == 0 {
				return errRefused
			}
			for uint32(len(state.GlobalFlags)) < args[0] {
				state.GlobalFlags = append(state.GlobalFlags, 0)
			}
			state.GlobalFlags[args[0]-1] = 1
		case worldUsageClean:
			// a CHAPTERAWARDINFO with flag 1 is still to take
			// (WorldProxy:NetUpdateWorldPressingAward)
			for i, award := range state.ChapterAwards {
				if award.Flag != 1 {
					continue
				}
				state.ChapterAwards[i].Flag = 0
				// WorldScene:GetAllPessingAward also opens the
				// entrance's transfers (MarkMapTransport), which after a
				// reload only a secured base map gives
				// (WorldAtlas:BuildTransportDic): the base map goes in
				// clean_chapter ("all normal zones will remain scouted
				// until the next reset", story GWORLDS06)
				if base := worldColormasks[award.ID].Chapter; base != 0 && !slices.Contains(state.CleanChapters, base) {
					state.CleanChapters = append(state.CleanChapters, base)
				}
				reward := worldEventCompletes[award.Award]
				for _, entry := range reward.RewardItems {
					if len(entry) >= 3 {
						drops = mergeDrop(drops, entry[0], entry[1], entry[2])
					}
				}
				if len(reward.RewardBuff) >= 2 {
					state.Buffs = addWorldBuff(state.Buffs, reward.RewardBuff[0], int64(reward.RewardBuff[1]))
				}
			}
		case worldUsageDrop:
			if drops = worldItemDrops(id, item, count); len(drops) == 0 {
				return errRefused
			}
		case worldUsageDropAppointed:
			var choices [][]uint32
			if json.Unmarshal(item.UsageArg, &choices) != nil || len(arg) < 3 || !slices.ContainsFunc(choices, func(choice []uint32) bool { return slices.Equal(choice, arg) }) {
				return errRefused
			}
			drops = []*protobuf.DROPINFO{dropInfo(arg[0], arg[1], arg[2]*count)}
		default:
			return errRefused
		}
		for _, drop := range drops {
			if drop.GetType() != dropTypeWorldItem && !storableDrop(drop.GetType(), drop.GetId()) {
				return errRefused
			}
			if _, ok := worldItems[drop.GetId()]; drop.GetType() == dropTypeWorldItem && !ok {
				return errRefused
			}
		}
		state.addItem(id, -int64(count))
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
	return client.Send(&protobuf.SC_33302{Result: result(err), DropList: drops})
}

// resetDrops are what the monthly reset turns the world inventory into
// (the rest is lost): every item_transform_num of an item gives
// item_transform_item_number of its transform (Operation Supply Coins 100
// to 1 voucher, Special Item Tokens 1 to 1, Energy Storage Devices 1 to 5,
// the coordinates and loggers 10 to 800, the Gear Design Plans their bag
// item), the vouchers up to world_resource_max held.
func (tx *Tx) worldResetDrops(state *worldState) ([]*protobuf.DROPINFO, error) {
	var drops []*protobuf.DROPINFO
	for _, item := range state.Items {
		config := worldItems[item.ID]
		if config.TransformType == 0 || config.TransformID == 0 || config.TransformNum == 0 {
			continue
		}
		if n := item.Count / config.TransformNum * config.TransformNumber; n > 0 {
			drops = mergeDrop(drops, config.TransformType, config.TransformID, n)
		}
	}
	rows, err := tx.ListResources(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	held := uint32(0)
	for _, row := range rows {
		if row.ResourceID == worldResource {
			held = row.Amount
		}
	}
	room := worldResourceMax - min(held, worldResourceMax)
	for i := 0; i < len(drops); i++ {
		if drops[i].GetType() != dropTypeResource || drops[i].GetId() != worldResource {
			continue
		}
		if drops[i].Number = proto.Uint32(min(drops[i].GetNumber(), room)); drops[i].GetNumber() == 0 {
			drops = slices.Delete(drops, i, i+1)
			i--
		}
	}
	return drops, tx.grantDrops(drops)
}
