package game

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The wild resources (island_wild_gather: bee nests, flowers...) and the
// collection fragments (island_collect_fragment) lying about the island's
// scenes (IslandGatherCollectAgency, model/wildcollect). A resource stands
// on one of its object_list objects (island_world_objects, which say the
// scene); taken, it grows back (refresh_time), maybe elsewhere. A
// fragment, once picked, counts toward its collection (collect_sys).
// GUESSES: a resource grows back at the next server midnight, on its
// object_list entry picked by that day; a resource gives one of the item
// its icon shows; the activities' resources (activity_id) are left out;
// only the owner takes them, visitors sign them (show 3).

// IslandGatherCollectAgency.ShowTpye
const (
	wildShowOwner = 1
	wildShowSign  = 3
)

// PB_ISLAND_GATHER_PUSH.push_type
const wildPushUpdate = 1

type productionGatherConfig struct {
	AbilityID      uint32   `json:"ability_id"`
	ActivityID     uint32   `json:"activity_id"`
	AutoParameters []uint32 `json:"auto_parameters"`
	Icon           string   `json:"icon"`
	ObjectList     []uint32 `json:"object_list"`
	Show           uint32   `json:"show"`
}

type productionFragmentConfig struct {
	CollectionID uint32   `json:"collection_id"`
	ObjectList   []uint32 `json:"object_list"`
	Show         uint32   `json:"show"`
	Task         uint32   `json:"task"`
}

type productionObjectConfig struct {
	MapID uint32 `json:"mapId"`
}

var (
	productionGathers   map[uint32]productionGatherConfig
	productionFragments map[uint32]productionFragmentConfig
	productionObjects   map[uint32]productionObjectConfig
)

func loadProductionWildData() error {
	return gamedata.LoadAll(
		gamedata.Load("island_wild_gather", &productionGathers),
		gamedata.Load("island_collect_fragment", &productionFragments),
		gamedata.Load("island_world_objects", &productionObjects),
	)
}

// the island's taken resources, in islandState.extra
const productionWildKey = "production_wild"

// wildTaken is a taken resource: where it stood, until when it is gone, who
// signed it.
type wildTaken struct {
	Pos   uint32 `json:"pos"`
	Until uint32 `json:"until"`
	Mark  uint32 `json:"mark"`
}

func (state *islandState) wildTaken() (map[uint32]wildTaken, error) {
	taken := map[uint32]wildTaken{}
	_, err := state.getExtra(productionWildKey, &taken)
	return taken, err
}

// gatherPos is where a resource stands once back at since.
func gatherPos(id, since uint32) uint32 {
	objects := productionGathers[id].ObjectList
	if len(objects) == 0 {
		return 0
	}
	return objects[(id+since/86400)%uint32(len(objects))]
}

// gatherVisible tells whether a resource is on the island for its owner
// (own) or a visitor.
func (state *islandState) gatherVisible(id uint32, own bool) bool {
	config := productionGathers[id]
	return config.ActivityID == 0 && state.productionHasAbility(config.AbilityID) && (own || config.Show != wildShowOwner)
}

// wildGathers are the island's resources as the client gets them
// (PB_ISLAND_WILD_GATHER): there (state 0) or taken until refresh_time.
func (state *islandState) wildGathers(own bool, now uint32) []*protobuf.PB_ISLAND_WILD_GATHER {
	taken, _ := state.wildTaken()
	var list []*protobuf.PB_ISLAND_WILD_GATHER
	for _, id := range sortedKeys(productionGathers) {
		if !state.gatherVisible(id, own) {
			continue
		}
		entry := taken[id]
		gather := &protobuf.PB_ISLAND_WILD_GATHER{
			Id: proto.Uint32(id), Pos: proto.Uint32(gatherPos(id, entry.Until)), State: proto.Uint32(0),
			Mark: proto.Uint32(0), RefreshTime: proto.Uint32(0),
		}
		if entry.Until > now {
			gather.Pos = proto.Uint32(entry.Pos)
			gather.State = proto.Uint32(1)
			gather.Mark = proto.Uint32(entry.Mark)
			gather.RefreshTime = proto.Uint32(entry.Until)
		}
		list = append(list, gather)
	}
	return list
}

// gatherDrops is what a resource gives. GUESS: one of the item its icon
// shows (IslandProps/item_<id>).
func gatherDrops(id uint32) []*protobuf.DROPINFO {
	icon := productionGathers[id].Icon
	index := strings.LastIndex(icon, "item_")
	if index < 0 {
		return nil
	}
	item, err := strconv.ParseUint(icon[index+len("item_"):], 10, 32)
	if err != nil {
		return nil
	}
	return []*protobuf.DROPINFO{{Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(uint32(item)), Number: proto.Uint32(1)}}
}

// takeGather marks a resource taken until the next server midnight and
// counts it (ACTIVIEY_GATHER for an activity's); the push is what the
// others on the island get (SC_21528).
func (state *islandState) takeGather(tx *Tx, id, now uint32) (*protobuf.PB_ISLAND_GATHER_PUSH, error) {
	taken, err := state.wildTaken()
	if err != nil {
		return nil, err
	}
	until := uint32(nextDay(time.Unix(int64(now), 0)).Unix())
	pos := gatherPos(id, taken[id].Until)
	taken[id] = wildTaken{Pos: pos, Until: until}
	if err := state.setExtra(productionWildKey, taken); err != nil {
		return nil, err
	}
	if activity := productionGathers[id].ActivityID; activity != 0 {
		if err := tx.islandTaskEvent(islandTaskActivityGather, 1, activity); err != nil {
			return nil, err
		}
	}
	return &protobuf.PB_ISLAND_GATHER_PUSH{
		Id: proto.Uint32(id), Pos: proto.Uint32(pos), State: proto.Uint32(1), Mark: proto.Uint32(0),
		RefreshTime: proto.Uint32(until), PushType: proto.Uint32(wildPushUpdate),
	}, nil
}

// fragmentVisible tells whether a fragment lies about the island: its
// collection not done nor the fragment picked (IslandGatherCollectAgency.
// ExistFragment), for the owner or, unless show 1, a visitor. GUESS: those
// a task puts there (task) are left out.
func (state *islandState) fragmentVisible(id uint32, own bool) bool {
	config, ok := productionFragments[id]
	if !ok || config.Task != 0 || (!own && config.Show == wildShowOwner) || len(config.ObjectList) == 0 {
		return false
	}
	collect := state.private().GetCollectSys()
	if slices.Contains(collect.GetFinishList(), config.CollectionID) {
		return false
	}
	for _, item := range collect.GetCollectItem() {
		if item.GetId() == config.CollectionID && slices.Contains(item.GetHadFragment(), id) {
			return false
		}
	}
	return true
}

// islandMapGathers adds to an EnterMapCommand reply (SC_21214) the wild
// resources and collection fragments of scene mapID on island state.
func islandMapGathers(tx *Tx, state *islandState, mapID uint32, reply *protobuf.SC_21214) error {
	own := state.commanderID == tx.commanderID
	for _, gather := range state.wildGathers(own, productionNow()) {
		if productionObjects[gather.GetPos()].MapID == mapID {
			reply.GatherList = append(reply.GatherList, gather)
		}
	}
	for _, id := range sortedKeys(productionFragments) {
		pos := productionFragments[id].ObjectList
		if state.fragmentVisible(id, own) && productionObjects[pos[0]].MapID == mapID {
			reply.FragmentList = append(reply.FragmentList, &protobuf.PB_ISLAND_COLLECT_FRAGMENT{
				Id: proto.Uint32(id), Pos: proto.Uint32(pos[0]), Mark: proto.Uint32(0),
			})
		}
	}
	return nil
}

// handleIslandWildGather answers IslandWildGatherCommand (CS_21524): the
// commander takes a wild resource of its own island that is there; the
// reply's drop_list has what it gives (gatherDrops), and the others on the
// island see it go (SC_21528).
func (game *Game) handleIslandWildGather(client *connection.Client, req *protobuf.CS_21524) error {
	reply := &protobuf.SC_21525{}
	var push *protobuf.PB_ISLAND_GATHER_PUSH
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		now := productionNow()
		id := req.GetGatherId()
		taken, err := state.wildTaken()
		if err != nil {
			return err
		}
		if req.GetIslandId() != client.CommanderID || !state.gatherVisible(id, true) || taken[id].Until > now {
			reply.Result = result(errRefused)
			return nil
		}
		if push, err = state.takeGather(tx, id, now); err != nil {
			return err
		}
		drops := gatherDrops(id)
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		reply.Result = result(nil)
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	if push != nil {
		game.tellIsland(client, client.CommanderID, &protobuf.SC_21528{IslandId: proto.Uint32(client.CommanderID), GatherList: []*protobuf.PB_ISLAND_GATHER_PUSH{push}})
	}
	return client.Send(reply)
}

// handleIslandWildGatherSign answers IslandWildGatherSignCommand (CS_21526):
// a visitor signs a wild resource of the island it is on that its owner
// lets friends sign (show 3). GUESS: the mark is the visitor's id, kept
// until the resource is taken, and the island's players see it (SC_21528).
func (game *Game) handleIslandWildGatherSign(client *connection.Client, req *protobuf.CS_21526) error {
	islandID, id := req.GetIslandId(), req.GetGatherId()
	current, here := game.islandOf(client)
	reply := &protobuf.SC_21527{}
	var push *protobuf.PB_ISLAND_GATHER_PUSH
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.otherIsland(islandID)
		if err != nil {
			return err
		}
		now := productionNow()
		if state == nil || !here || current != islandID || islandID == client.CommanderID ||
			!state.gatherVisible(id, false) || productionGathers[id].Show != wildShowSign {
			reply.Result = result(errRefused)
			return nil
		}
		taken, err := state.wildTaken()
		if err != nil {
			return err
		}
		entry := taken[id]
		if entry.Until > now || entry.Mark != 0 {
			reply.Result = result(errRefused)
			return nil
		}
		entry.Pos = gatherPos(id, entry.Until)
		entry.Mark = client.CommanderID
		taken[id] = entry
		if err := state.setExtra(productionWildKey, taken); err != nil {
			return err
		}
		// another commander's island: Tx only saves the commander's own
		if err := saveIsland(tx.ctx, tx.Queries, state); err != nil {
			return err
		}
		push = &protobuf.PB_ISLAND_GATHER_PUSH{
			Id: proto.Uint32(id), Pos: proto.Uint32(entry.Pos), State: proto.Uint32(0), Mark: proto.Uint32(entry.Mark),
			RefreshTime: proto.Uint32(0), PushType: proto.Uint32(wildPushUpdate),
		}
		reply.Result = result(nil)
		return nil
	})
	if err != nil {
		return err
	}
	if push != nil {
		game.tellIsland(client, islandID, &protobuf.SC_21528{IslandId: proto.Uint32(islandID), GatherList: []*protobuf.PB_ISLAND_GATHER_PUSH{push}})
	}
	return client.Send(reply)
}

// handleIslandWildCollect answers IslandWildCollectCommand (CS_21529): the
// commander picks a collection fragment of its own island, which counts
// toward its collection (collect_sys.collect_item, added by the client).
// Completing the collection is not done here (no reply field says so).
func (game *Game) handleIslandWildCollect(client *connection.Client, req *protobuf.CS_21529) error {
	reply := &protobuf.SC_21530{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		id := req.GetFragmentId()
		if req.GetIslandId() != client.CommanderID || !state.fragmentVisible(id, true) {
			reply.Result = result(errRefused)
			return nil
		}
		collection := productionFragments[id].CollectionID
		collect := state.private().CollectSys
		if collect == nil {
			collect = &protobuf.PB_ISLAND_COLLECT_SYS{}
			state.private().CollectSys = collect
		}
		var item *protobuf.PB_ISLAND_COLLECT_ITEM
		for _, entry := range collect.CollectItem {
			if entry.GetId() == collection {
				item = entry
			}
		}
		if item == nil {
			item = &protobuf.PB_ISLAND_COLLECT_ITEM{Id: proto.Uint32(collection)}
			collect.CollectItem = append(collect.CollectItem, item)
		}
		item.HadFragment = append(item.HadFragment, id)
		reply.Result = result(nil)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandWildCollectSign answers IslandWildCollectSignCommand
// (CS_21531): a visitor signs a fragment. The client sends the fragment as
// gather_id, a field CS_21531 doesn't have, so the server never learns
// which: accepted, nothing kept.
//
//belfast:static
func (game *Game) handleIslandWildCollectSign(client *connection.Client, req *protobuf.CS_21531) error {
	return client.Send(&protobuf.SC_21532{Result: result(nil)})
}
