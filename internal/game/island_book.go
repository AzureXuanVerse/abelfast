package game

import (
	"context"
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's illustrated book (IslandBookAgency, IslandIllustration,
// mod/island/command/book): characters, NPCs, items and fish, each to unlock
// once met (cond_list), then worth collection points (book_collects: the
// entry's collect_add, plus collect_upgrade / collect_star for a
// character's levels and limit breaks, an item's count obtained, a fish's
// cups) that pay the island_collection_reward of each kind. Built from the
// Lua and the tables alone.

// IslandIllustration.TYPES
const (
	islandBookChar = 1
	islandBookNpc  = 2
	islandBookItem = 3
	islandBookFish = 4
)

// islandBookConfig is an island_illustrated_guide row.
type islandBookConfig struct {
	Type           uint32     `json:"type"`
	UnlockID       uint32     `json:"unlock_id"`
	CollectAdd     uint32     `json:"collect_add"`
	CollectStar    [][]uint32 `json:"collect_star"`
	CollectUpgrade [][]uint32 `json:"collect_upgrade"`
	AwardUnlock    [][]uint32 `json:"award_unlock"`
}

// islandBookRewardConfig is an island_collection_reward row: the reward
// once a kind's points reach need_exp.
type islandBookRewardConfig struct {
	Type    uint32   `json:"type"`
	NeedExp uint32   `json:"need_exp"`
	Award   []uint32 `json:"award_display"`
}

var (
	islandBook        map[uint32]islandBookConfig
	islandBookRewards map[uint32]islandBookRewardConfig
	islandBookByLink  map[[2]uint32]uint32 // entry by (type, unlock_id)
)

func (game *Game) registerIslandBook() {
	packets.Handle(game.handleIslandBookCondition)
	packets.Handle(game.handleIslandBookUnlock)
	packets.Handle(game.handleIslandBookPoints)
	packets.Handle(game.handleIslandBookAward)
}

func loadIslandBookData() error {
	err := gamedata.LoadAll(
		gamedata.Load("island_illustrated_guide", &islandBook),
		gamedata.Load("island_collection_reward", &islandBookRewards),
	)
	if err != nil {
		return err
	}
	islandBookByLink = map[[2]uint32]uint32{}
	for id, entry := range islandBook {
		islandBookByLink[[2]uint32{entry.Type, entry.UnlockID}] = id
	}
	return nil
}

// addIslandBookCondition marks a book entry as met (cond_list), what lets
// the client unlock it; false if it already was.
func addIslandBookCondition(state *islandState, kind, link uint32) bool {
	book := state.private().ViewBook
	for _, condition := range book.CondList {
		if condition.GetType() == kind {
			if slices.Contains(condition.UnlockIds, link) {
				return false
			}
			condition.UnlockIds = append(condition.UnlockIds, link)
			return true
		}
	}
	book.CondList = append(book.CondList, &protobuf.PB_BOOK_COND{Type: proto.Uint32(kind), UnlockIds: []uint32{link}})
	return true
}

// islandBookItemsObtained counts island items obtained for their book
// entries (view_book item_list, what an item's collect_star counts), marks
// them met, and pushes the counts added (SC_21342,
// IslandBookAgency.HandlePushData).
func (tx *Tx) islandBookItemsObtained(state *islandState, items map[uint32]uint32) {
	book := state.private().ViewBook
	var pushed []*protobuf.PB_ISLAND_ITEM
	for _, id := range slices.Sorted(maps.Keys(items)) {
		if _, ok := islandBookByLink[[2]uint32{islandBookItem, id}]; !ok || items[id] == 0 {
			continue
		}
		found := false
		for _, item := range book.ItemList {
			if item.GetId() == id {
				item.Num = proto.Uint32(item.GetNum() + items[id])
				found = true
			}
		}
		if !found {
			book.ItemList = append(book.ItemList, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(items[id])})
		}
		addIslandBookCondition(state, islandBookItem, id)
		pushed = append(pushed, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(id), Num: proto.Uint32(items[id])})
	}
	if len(pushed) > 0 {
		tx.push(&protobuf.SC_21342{ItemList: pushed})
	}
}

// handleIslandBookCondition answers IslandUpdateIllustrationCommand
// (CS_21340): the client met a character, NPC or fish (type, its id); the
// entry can be unlocked from now on.
func (game *Game) handleIslandBookCondition(client *connection.Client, req *protobuf.CS_21340) error {
	if _, ok := islandBookByLink[[2]uint32{req.GetType(), req.GetCondId()}]; !ok {
		return client.Send(&protobuf.SC_21341{Result: result(errRefused)})
	}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		addIslandBookCondition(state, req.GetType(), req.GetCondId())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21341{Result: proto.Uint32(0)})
}

// islandBookCollect is an entry's points as stored, created at its base.
func islandBookCollect(state *islandState, id uint32) *protobuf.PB_BOOK_COLLECT {
	book := state.private().ViewBook
	for _, collect := range book.BookCollects {
		if collect.GetId() == id {
			return collect
		}
	}
	collect := &protobuf.PB_BOOK_COLLECT{Id: proto.Uint32(id), Base: proto.Uint32(islandBook[id].CollectAdd)}
	book.BookCollects = append(book.BookCollects, collect)
	return collect
}

// handleIslandBookUnlock answers IslandUnlockIllustrationCommand
// (CS_21343): met entries unlocked, worth their collect_add points, with
// their award_unlock rewards.
func (game *Game) handleIslandBookUnlock(client *connection.Client, req *protobuf.CS_21343) error {
	reply := &protobuf.SC_21344{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		book := state.private().ViewBook
		for _, id := range req.GetBookIds() {
			entry, ok := islandBook[id]
			met := slices.ContainsFunc(book.CondList, func(condition *protobuf.PB_BOOK_COND) bool {
				return condition.GetType() == entry.Type && slices.Contains(condition.UnlockIds, entry.UnlockID)
			})
			// a character the island has counts as met (IslandBookAgency.InitShipTypeData)
			if entry.Type == islandBookChar && islandShipOwned(state, entry.UnlockID) {
				met = true
			}
			if !ok || !met || slices.Contains(book.BookList, id) {
				return errRefused
			}
			book.BookList = append(book.BookList, id)
			islandBookCollect(state, id)
			reply.DropList = append(reply.DropList, configDrops(entry.AwardUnlock)...)
		}
		return tx.grantDrops(reply.DropList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21344{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// islandShipOwned tells whether the island has a character
// (island_chara_template id).
func islandShipOwned(state *islandState, id uint32) bool {
	return slices.ContainsFunc(state.public().ShipSys.ShipList, func(ship *protobuf.PB_ISLAND_SHIP) bool { return ship.GetId() == id })
}

// handleIslandBookPoints answers IslandGetCollectPointCommand (CS_21345):
// the points unlocked entries earned since, by IslandCharIllustration
// (collect_upgrade by level, collect_star by limit break),
// IslandItemIllustration (collect_star by count obtained) and
// IslandFishIllustration (collect_star 1 and 2 by the fish's cups); the
// updated entries in collect_list.
func (game *Game) handleIslandBookPoints(client *connection.Client, req *protobuf.CS_21345) error {
	reply := &protobuf.SC_21346{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		book := state.private().ViewBook
		for _, id := range req.GetBookIds() {
			entry := islandBook[id]
			if !slices.Contains(book.BookList, id) {
				return errRefused
			}
			collect := islandBookCollect(state, id)
			reached := func(list *[]*protobuf.PB_LV_COLLECT, steps [][]uint32, value func(step uint32) bool) {
				for _, step := range steps {
					if len(step) < 2 || !value(step[0]) || slices.ContainsFunc(*list, func(got *protobuf.PB_LV_COLLECT) bool { return got.GetLv() == step[0] }) {
						continue
					}
					*list = append(*list, &protobuf.PB_LV_COLLECT{Lv: proto.Uint32(step[0]), Value: proto.Uint32(step[1])})
				}
			}
			switch entry.Type {
			case islandBookChar:
				for _, ship := range state.public().ShipSys.ShipList {
					if ship.GetId() == entry.UnlockID {
						reached(&collect.LvList, entry.CollectUpgrade, func(lv uint32) bool { return lv <= ship.GetLv() })
						reached(&collect.StarList, entry.CollectStar, func(lv uint32) bool { return lv <= ship.GetBreakLv() })
					}
				}
			case islandBookItem:
				count := uint32(0)
				for _, item := range book.ItemList {
					if item.GetId() == entry.UnlockID {
						count = item.GetNum()
					}
				}
				reached(&collect.StarList, entry.CollectStar, func(need uint32) bool { return need <= count })
			case islandBookFish:
				for _, fish := range state.private().FishSys.FishWeight {
					if fish.GetFishId() == entry.UnlockID {
						cups := fish.GetGoldState()
						reached(&collect.StarList, entry.CollectStar, func(cup uint32) bool { return cups == 3 || cups == cup })
					}
				}
			}
			reply.CollectList = append(reply.CollectList, collect)
		}
		return nil
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21346{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}

// handleIslandBookAward answers IslandGetPointAwardCommand (CS_21347, lv
// an island_collection_reward id): its reward once the points of its kind
// reach need_exp, once.
func (game *Game) handleIslandBookAward(client *connection.Client, req *protobuf.CS_21347) error {
	reply := &protobuf.SC_21348{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		book := state.private().ViewBook
		config, ok := islandBookRewards[req.GetLv()]
		if !ok || slices.Contains(book.BookAwards, req.GetLv()) {
			return errRefused
		}
		points := uint32(0)
		for _, collect := range book.BookCollects {
			if islandBook[collect.GetId()].Type != config.Type || !slices.Contains(book.BookList, collect.GetId()) {
				continue
			}
			points += collect.GetBase()
			for _, got := range append(slices.Clone(collect.LvList), collect.StarList...) {
				points += got.GetValue()
			}
		}
		if points < config.NeedExp {
			return errRefused
		}
		book.BookAwards = append(book.BookAwards, req.GetLv())
		reply.DropList = configDrops([][]uint32{config.Award})
		return tx.grantDrops(reply.DropList)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_21348{Result: result(err)})
	}
	if err != nil {
		return err
	}
	reply.Result = proto.Uint32(0)
	return client.Send(islandStamped(reply))
}
