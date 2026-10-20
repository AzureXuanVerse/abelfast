package game

import (
	"context"
	"encoding/json"
	"slices"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's level, name, prosperity rewards, warehouse (capacity,
// overflow, conversion into season points, crafting), shops and fishing:
// see island_core.go. The client applies each reply to its own model
// (Island, IslandInventoryAgency, IslandShopAgency, IslandFishingAgency), so
// every handler changes the stored PB_ISLAND exactly as the Lua does.

// IslandItem.TYPE_MATERIAL: the only items that take warehouse slots
// (IslandItem:GetNumberOfSlotsOccupied)
const islandItemMaterial = 1

// IslandAblityAgency.TYPE_INVENTORY_MAXCNT: an ability adding its effect to
// the warehouse capacity
const islandAbilityInventory = 6

// islandItemConfig is an island_item_data_template row.
type islandItemConfig struct {
	Type     uint32          `json:"type"`
	Rarity   uint32          `json:"rarity"`
	GroupMax uint32          `json:"group_max"`
	Convert  uint32          `json:"convert"`
	PtNum    uint32          `json:"pt_num"`
	Usage    string          `json:"usage"`
	UsageArg json.RawMessage `json:"usage_arg"`
}

// islandLevelConfig is an island_level row: the development exp to the
// next level and what reaching it gives.
type islandLevelConfig struct {
	Exp         uint32          `json:"island_exp"`
	Award       [][]uint32      `json:"island_level_award"`
	AbilityDrop json.RawMessage `json:"ability_drop"`
}

// islandProsperityConfig is an island_prosperity row.
type islandProsperityConfig struct {
	Prosperity uint32     `json:"prosperity"`
	Award      [][]uint32 `json:"award_display"`
}

// islandStorageConfig is an island_storage_level row: the warehouse's
// capacity at a level and what reaching that level costs.
type islandStorageConfig struct {
	Capacity uint32     `json:"capacity"`
	Material [][]uint32 `json:"upgrade_material"`
}

// islandExchangeConfig is an island_exchange_template row: one origin_item
// crafted into target_num target_item.
type islandExchangeConfig struct {
	Origin    uint32 `json:"origin_item"`
	Target    uint32 `json:"target_item"`
	TargetNum uint32 `json:"target_num"`
}

// islandInvAbilityConfig is the part of an island_ability_template row the
// warehouse and fishing read.
type islandInvAbilityConfig struct {
	Type   uint32 `json:"type"`
	Effect uint32 `json:"effect"`
}

var (
	islandItems        map[uint32]islandItemConfig
	islandLevels       map[uint32]islandLevelConfig
	islandMaxLevel     uint32
	islandProsperities map[uint32]islandProsperityConfig
	islandStorages     map[uint32]islandStorageConfig
	islandExchanges    map[uint32]islandExchangeConfig
	islandInvAbilities map[uint32]islandInvAbilityConfig
)

func (game *Game) registerIslandInventory() {
	packets.Handle(game.handleIslandUpgrade)
	packets.Handle(game.handleIslandName)
	packets.Handle(game.handleIslandOverflow)
	packets.Handle(game.handleIslandProsperityAward)
	packets.Handle(game.handleIslandStorageUpgrade)
	packets.Handle(game.handleIslandConvert)
	packets.Handle(game.handleIslandUseItem)
	packets.Handle(game.handleIslandExchange)
	game.registerIslandShops()
	game.registerIslandFishing()
}

func loadIslandInventoryData() error {
	err := gamedata.LoadAll(
		gamedata.Load("island_item_data_template", &islandItems),
		gamedata.Load("island_level", &islandLevels),
		gamedata.Load("island_prosperity", &islandProsperities),
		gamedata.Load("island_storage_level", &islandStorages),
		gamedata.Load("island_exchange_template", &islandExchanges),
		gamedata.Load("island_ability_template", &islandInvAbilities),
		loadIslandShopData,
		loadIslandFishingData,
	)
	if err != nil {
		return err
	}
	islandMaxLevel = 0
	for level := range islandLevels {
		islandMaxLevel = max(islandMaxLevel, level)
	}
	if _, ok := islandStorages[1]; !ok {
		return gamedata.Missing("island_storage_level", "level 1")
	}
	return nil
}

// islandReply is the result of an island request's transaction: its
// refusal or nil; any other error is returned as is.
func islandReply(err error) (*uint32, error) {
	if err != nil && !refused(err) {
		return nil, err
	}
	return result(err), nil
}

// islandItemDrops turns [id, count] pairs (or [type, id, count] triples)
// into rewards, multiplied by times.
func islandItemDrops(entries [][]uint32, times uint32) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, entry := range entries {
		kind, id, count := uint32(dropTypeIslandItem), uint32(0), uint32(0)
		switch len(entry) {
		case 2:
			id, count = entry[0], entry[1]
		case 3:
			kind, id, count = entry[0], entry[1], entry[2]
		default:
			continue
		}
		drops = addDrop(drops, kind, id, count*times)
	}
	return drops
}

// addDrop adds count of a reward to drops, merged with the same one.
func addDrop(drops []*protobuf.DROPINFO, kind, id, count uint32) []*protobuf.DROPINFO {
	for _, drop := range drops {
		if drop.GetType() == kind && drop.GetId() == id {
			drop.Number = proto.Uint32(drop.GetNumber() + count)
			return drops
		}
	}
	return append(drops, &protobuf.DROPINFO{Type: proto.Uint32(kind), Id: proto.Uint32(id), Number: proto.Uint32(count)})
}

// handleIslandUpgrade answers UpgradeIslandCommand (CS_21000, sent while
// Island:CanLevelUp): once the development exp reaches island_level's
// island_exp the island goes up a level keeping the rest of the exp
// (Island:Upgrade), and gets that level's island_level_award as warehouse
// items (Island:GetUpgradeAwards), which the client adds from drop_list.
// The client asks again until it can't level up. GUESSES: ability_drop
// (empty in every row) is given as abilities, and cost (Development Funds,
// shown nowhere and never taken by the client) is not charged.
func (game *Game) handleIslandUpgrade(client *connection.Client, req *protobuf.CS_21000) error {
	reply := &protobuf.SC_21001{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		public := state.public()
		level := public.GetLevel()
		config, ok := islandLevels[level]
		if !ok || level >= islandMaxLevel || public.GetExp() < config.Exp {
			return errRefused
		}
		public.Exp = proto.Uint32(public.GetExp() - config.Exp)
		public.Level = proto.Uint32(level + 1)
		drops := islandItemDrops(config.Award, 1)
		for _, ability := range gamedata.IDs(config.AbilityDrop) {
			drops = addDrop(drops, dropTypeIslandAbility, ability, 1)
		}
		reply.DropList = drops
		return tx.grantDrops(drops)
	})
	if reply.Ret, err = islandReply(err); err != nil {
		return err
	}
	if reply.GetRet() != 0 {
		reply.DropList = nil
	}
	return client.Send(reply)
}

// handleIslandName answers SetIslandNameCommand (CS_21004): the island's
// name (type 2 is the first naming, IslandSetNamePage, 1 a rename). The
// client checks the name is 1 to 18 characters without banned words
// (nameValidityCheck) and sets it on ret 0. GUESS: renaming is free and
// unlimited (Island:CanModifyName is always true, island_rename_consutme_tip
// names no cost).
func (game *Game) handleIslandName(client *connection.Client, req *protobuf.CS_21004) error {
	name := req.GetName()
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 18 {
		return client.Send(&protobuf.SC_21005{Ret: result(errRefused)})
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		state.public().Name = proto.String(name)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21005{Ret: proto.Uint32(0)})
}

// handleIslandProsperityAward answers GetIslandProsperityAwardCommand
// (CS_21010): an island_prosperity level's award_display, once, when the
// island's prosperity reached it (Island:CanGetProsperityAwards); the
// client marks it received and adds drop_list.
func (game *Game) handleIslandProsperityAward(client *connection.Client, req *protobuf.CS_21010) error {
	reply := &protobuf.SC_21011{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		public := state.public()
		config, ok := islandProsperities[req.GetLevel()]
		if !ok || slices.Contains(public.ProsperityRewarded, req.GetLevel()) || public.GetProsperity() < config.Prosperity {
			return errRefused
		}
		public.ProsperityRewarded = append(public.ProsperityRewarded, req.GetLevel())
		reply.DropList = islandItemDrops(config.Award, 1)
		return tx.grantDrops(reply.DropList)
	})
	if reply.Ret, err = islandReply(err); err != nil {
		return err
	}
	if reply.GetRet() != 0 {
		reply.DropList = nil
	}
	return client.Send(reply)
}

// handleIslandStorageUpgrade answers UpgradeIslandInventoryCommand
// (CS_21012): the warehouse goes up a level for the next
// island_storage_level row's upgrade_material, which the client takes
// itself (GAME.CONSUME_ITEM) on ret 0. The last level is refused
// (island_bag_max_level).
func (game *Game) handleIslandStorageUpgrade(client *connection.Client, req *protobuf.CS_21012) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		public := state.public()
		next, ok := islandStorages[public.GetStorageLevel()+1]
		if !ok {
			return errRefused
		}
		if err := tx.payIslandCosts(state, next.Material); err != nil {
			return err
		}
		public.StorageLevel = proto.Uint32(public.GetStorageLevel() + 1)
		return nil
	})
	ret, err := islandReply(err)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21013{Ret: ret})
}

// payIslandCosts takes [type, id, count] costs: warehouse items
// (DROP_TYPE_ISLAND_ITEM), player resources or bag items.
func (tx *Tx) payIslandCosts(state *islandState, costs [][]uint32) error {
	for _, cost := range costs {
		if len(cost) < 3 {
			return errRefused
		}
		if cost[0] == dropTypeIslandItem {
			if err := state.takeIslandItem(cost[1], cost[2]); err != nil {
				return err
			}
			continue
		}
		if err := tx.payDrops([][]uint32{cost}); err != nil {
			return err
		}
	}
	return nil
}

// handleIslandOverflow answers CS_21006. IslandGetOverFlowItemCommand
// (the overflow tab's "one key" button) sends it to move the items that
// didn't fit (item_list_cache) into the warehouse: item_list is what moved,
// which the client takes off its overflow list and adds to the warehouse.
// SetIslandSetManifestoCommand sends it too with a signature field CS_21006
// doesn't have (so the server sees the same type 0) and reads a ret field
// SC_21007 doesn't have: the manifesto can't be stored (PB_ISLAND_PUBLIC
// has no field for it) and that client path always fails. GUESS: the
// items move highest rarity first, then by id (the order of
// IslandInventoryAgency:SplitItemList4Add), as many as the capacity takes.
func (game *Game) handleIslandOverflow(client *connection.Client, req *protobuf.CS_21006) error {
	reply := &protobuf.SC_21007{Result: proto.Uint32(0)}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		private := state.private()
		cache := private.ItemListCache
		private.ItemListCache = nil
		slices.SortStableFunc(cache, func(a, b *protobuf.PB_ISLAND_ITEM) int {
			ra, rb := islandItems[a.GetId()].Rarity, islandItems[b.GetId()].Rarity
			if ra != rb {
				return int(rb) - int(ra)
			}
			return int(a.GetId()) - int(b.GetId())
		})
		var kept []*protobuf.PB_ISLAND_ITEM
		for _, item := range cache {
			moved := state.islandItemRoom(item.GetId(), item.GetNum())
			if moved > 0 {
				state.addIslandItem(item.GetId(), moved)
				reply.ItemList = append(reply.ItemList, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(item.GetId()), Num: proto.Uint32(moved)})
			}
			if moved < item.GetNum() {
				kept = append(kept, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(item.GetId()), Num: proto.Uint32(item.GetNum() - moved)})
			}
		}
		private.ItemListCache = kept
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandConvert answers IslandConvertSeasonPtCommand (CS_21014, the
// warehouse's sell mode: type 1 from the warehouse, 2 from the overflow
// list): the items go and the client adds their pt_num each in season
// points itself (only convert 1 items: the page lists no others), plus
// item_list as warehouse items. SellIslandItemCommand and
// BatchSellIslandItemCommand send it too but nothing runs them any more.
// GUESS: item_list is empty (the client already counts the season points).
func (game *Game) handleIslandConvert(client *connection.Client, req *protobuf.CS_21014) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		points := uint32(0)
		for _, item := range req.GetItemList() {
			config, ok := islandItems[item.GetId()]
			if !ok || config.Convert != 1 || item.GetNum() == 0 {
				return errRefused
			}
			switch req.GetType() {
			case 1:
				err = state.takeIslandItem(item.GetId(), item.GetNum())
			case 2:
				err = state.takeIslandOverflow(item.GetId(), item.GetNum())
			default:
				err = errRefused
			}
			if err != nil {
				return err
			}
			points += config.PtNum * item.GetNum()
		}
		if points == 0 {
			return nil
		}
		return tx.grantDrops([]*protobuf.DROPINFO{{
			Type: proto.Uint32(dropTypeIslandSeasonPt), Id: proto.Uint32(0), Number: proto.Uint32(points),
		}})
	})
	ret, err := islandReply(err)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21015{Result: ret})
}

// handleIslandUseItem answers IslandUseItemCommand (CS_21026). Nothing in
// the client sends it any more (IslandMediator.ON_USE_ITEM is never
// emitted) and no island item has a use it would run: the gifts
// (usage_island_gift) go through the characters' requests and the baits
// (usage_fish) through fishing, so it is refused.
func (game *Game) handleIslandUseItem(client *connection.Client, req *protobuf.CS_21026) error {
	return client.Send(&protobuf.SC_21027{Result: result(errRefused)})
}

// handleIslandExchange answers IslandExchangeItemCommand (CS_21066,
// crafting: fish into fish meat): each make takes num origin_item and gives
// num times target_num target_item (island_exchange_template); the client
// takes the origin items itself and adds drop_list.
func (game *Game) handleIslandExchange(client *connection.Client, req *protobuf.CS_21066) error {
	reply := &protobuf.SC_21067{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		var drops []*protobuf.DROPINFO
		for _, entry := range req.GetMakes() {
			config, ok := islandExchanges[entry.GetMakeId()]
			if !ok || entry.GetNum() == 0 {
				return errRefused
			}
			if err := state.takeIslandItem(config.Origin, entry.GetNum()); err != nil {
				return err
			}
			drops = addDrop(drops, dropTypeIslandItem, config.Target, config.TargetNum*entry.GetNum())
		}
		if len(drops) == 0 {
			return errRefused
		}
		reply.DropList = drops
		return tx.grantDrops(drops)
	})
	if reply.Result, err = islandReply(err); err != nil {
		return err
	}
	if reply.GetResult() != 0 {
		reply.DropList = nil
	}
	return client.Send(reply)
}

// islandCapacity is how many slots the warehouse has
// (IslandInventoryAgency:GetCapacity): its level's capacity plus the
// inventory abilities' effect.
func (state *islandState) islandCapacity() uint32 {
	capacity := islandStorages[state.public().GetStorageLevel()].Capacity
	for _, ability := range state.public().GetAbilityList() {
		if config := islandInvAbilities[ability]; config.Type == islandAbilityInventory {
			capacity += config.Effect
		}
	}
	return capacity
}

// islandSlots is how many warehouse slots count of an item takes
// (IslandItem:GetNumberOfSlotsOccupied): only materials do, one per
// group_max (one in all when group_max is 0).
func islandSlots(id, count uint32) uint32 {
	config := islandItems[id]
	switch {
	case config.Type != islandItemMaterial || count == 0:
		return 0
	case config.GroupMax == 0:
		return 1
	}
	return (count + config.GroupMax - 1) / config.GroupMax
}

// islandUsedSlots is how many warehouse slots are taken
// (IslandInventoryAgency:GetLength).
func (state *islandState) islandUsedSlots() uint32 {
	used := uint32(0)
	for _, item := range state.private().GetItemList() {
		used += islandSlots(item.GetId(), item.GetNum())
	}
	return used
}

// islandItemRoom is how many of count of an item still fit in the
// warehouse (all of them for an item that takes no slot).
func (state *islandState) islandItemRoom(id, count uint32) uint32 {
	if islandSlots(id, count) == 0 {
		return count
	}
	have := state.islandItemCount(id)
	capacity, used := state.islandCapacity(), state.islandUsedSlots()-islandSlots(id, have)
	if used >= capacity {
		return 0
	}
	group := islandItems[id].GroupMax
	if group == 0 {
		return count
	}
	room := (capacity-used)*group - have
	return min(count, room)
}

// islandItemOverflows tells whether count of an item given now goes to the
// overflow list (DROP_TYPE_ISLAND_OVERFLOWITEM) instead of the warehouse:
// a material when items already wait there or when the warehouse can't
// take all of it (the client adds drop types 41 to the warehouse and 42 to
// the overflow list as told, IslandDropHelper.AddItems). GUESS: a reward is
// never split; only materials overflow (nothing else takes a slot).
func (state *islandState) islandItemOverflows(id, count uint32) bool {
	if islandSlots(id, count) == 0 {
		return false
	}
	return len(state.private().GetItemListCache()) > 0 || state.islandItemRoom(id, count) < count
}

// addIslandOverflow puts count of an item on the overflow list
// (item_list_cache).
func (state *islandState) addIslandOverflow(id, count uint32) {
	private := state.private()
	for _, item := range private.ItemListCache {
		if item.GetId() == id {
			item.Num = proto.Uint32(item.GetNum() + count)
			return
		}
	}
	private.ItemListCache = append(private.ItemListCache, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(count)})
}

// takeIslandOverflow removes count of an item from the overflow list;
// ErrNotEnough when there are fewer.
func (state *islandState) takeIslandOverflow(id, count uint32) error {
	private := state.private()
	for i, item := range private.ItemListCache {
		if item.GetId() != id {
			continue
		}
		if item.GetNum() < count {
			return ErrNotEnough
		}
		item.Num = proto.Uint32(item.GetNum() - count)
		if item.GetNum() == 0 {
			private.ItemListCache = slices.Delete(private.ItemListCache, i, i+1)
		}
		return nil
	}
	return ErrNotEnough
}
