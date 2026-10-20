package game

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Entering an island and being on it with others (EnterIslandCommand,
// GetIslandDataCommand, EnterMapCommand, IslandSyncMgr): who is on which
// island and scene, the scene objects' seats (IslandSyncControlCommand), and
// the movements relayed between the players of a scene
// (IslandSyncCommand). All of it lives in memory: an island is left when the
// connection closes, and the client enters again after a reconnect
// (IslandReconnectCommand).

// IslandConst.PLAYER_DATA_STATE_*: what an SC_21206 entry says of a player
const (
	islandPlayerUpdate = 0 // moved to another scene (map_id) or changed
	islandPlayerEnter  = 1
	islandPlayerExit   = 2
)

// islandWorld is who is on which island right now.
type islandWorld struct {
	mu      sync.Mutex
	islands map[uint32]*islandRoom // by island (its owner's commander id)
	where   map[uint32]uint32      // the island each commander is on
}

// islandRoom is an island with players on it.
type islandRoom struct {
	players map[uint32]*islandPresence // by commander
	objects map[[2]uint32]*protobuf.PB_OBJECT
}

// islandPresence is a commander on an island.
type islandPresence struct {
	client *connection.Client
	mapID  uint32 // 0 until the first scene is loaded (CS_21213)
	seen   time.Time
}

// registerIsland registers the island's handlers, every subsystem's.
func (game *Game) registerIsland() {
	packets.Handle(game.handleIslandEnter)
	packets.Handle(game.handleIslandQueue)
	packets.Handle(game.handleIslandData)
	packets.Handle(game.handleIslandEnterMap)
	packets.Handle(game.handleIslandExit)
	packets.Handle(game.handleIslandHeartbeat)
	packets.Handle(game.handleIslandSync)
	packets.Handle(game.handleIslandSyncControl)
	packets.Handle(game.handleIslandRecordPosition)
	packets.Handle(game.handleIslandReconnect)
	game.registerIslandInventory()
	game.registerIslandTasks()
	game.registerIslandOrders()
	game.registerIslandProduction()
	game.registerIslandShips()
	game.registerIslandSocial()
}

// islandPlayer is a commander as the players of an island see it
// (IslandPlayer): portrait, level, name, scene and outfit (worn parts and
// their colors).
func (game *Game) islandPlayer(ctx context.Context, commanderID, state, mapID uint32) (*protobuf.PB_PLAYER, error) {
	commander, err := game.DB.GetCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	player := &protobuf.PB_PLAYER{
		Id: proto.Uint32(commanderID), State: proto.Uint32(state), Name: proto.String(commander.Name),
		Level: proto.Uint32(commander.Level), Display: game.displayInfo(commander), MapId: proto.Uint32(mapID),
	}
	island, err := readIsland(ctx, game.DB.Queries, commanderID)
	if err != nil || island == nil {
		return player, err
	}
	dress := island.private().GetUserDress()
	player.CurDress = dress.GetCurDress()
	colors := map[uint32]uint32{}
	for _, owned := range dress.GetHadDress() {
		colors[owned.GetId()] = owned.GetColor()
	}
	for _, worn := range dress.GetCurDress() {
		if color := colors[worn.GetId()]; color != 0 {
			player.DressColor = append(player.DressColor, &protobuf.PB_DRESS_COLOR{Id: proto.Uint32(worn.GetId()), Color: proto.Uint32(color)})
		}
	}
	return player, nil
}

// joinIsland puts the client's commander on an island, leaving the one it
// was on, and lists who is there (itself included). The others see it
// arrive (SC_21206).
func (game *Game) joinIsland(ctx context.Context, client *connection.Client, islandID uint32) ([]*protobuf.PB_PLAYER, error) {
	id := client.CommanderID
	game.leaveIsland(ctx, client)
	world := &game.islands
	world.mu.Lock()
	if world.islands == nil {
		world.islands = map[uint32]*islandRoom{}
		world.where = map[uint32]uint32{}
	}
	room := world.islands[islandID]
	if room == nil {
		room = &islandRoom{players: map[uint32]*islandPresence{}, objects: map[[2]uint32]*protobuf.PB_OBJECT{}}
		world.islands[islandID] = room
	}
	room.players[id] = &islandPresence{client: client, seen: time.Now()}
	world.where[id] = islandID
	present := slices.Sorted(maps.Keys(room.players))
	world.mu.Unlock()
	client.OnClose(func() { game.leaveIsland(context.Background(), client) })

	players := make([]*protobuf.PB_PLAYER, 0, len(present))
	for _, player := range present {
		entry, err := game.islandPlayer(ctx, player, islandPlayerEnter, game.islandMapOf(player))
		if err != nil {
			return nil, err
		}
		players = append(players, entry)
		if player == id {
			game.tellIsland(client, islandID, &protobuf.SC_21206{IslandId: proto.Uint32(islandID), PlayerList: []*protobuf.PB_PLAYER{entry}})
		}
	}
	return players, nil
}

// islandMapOf is the scene a commander is in on its island, 0 if none.
func (game *Game) islandMapOf(commanderID uint32) uint32 {
	world := &game.islands
	world.mu.Lock()
	defer world.mu.Unlock()
	if room := world.islands[world.where[commanderID]]; room != nil && room.players[commanderID] != nil {
		return room.players[commanderID].mapID
	}
	return 0
}

// islandOf is the island the client's commander is on (0 if none), and
// whether this connection is the one there.
func (game *Game) islandOf(client *connection.Client) (uint32, bool) {
	world := &game.islands
	world.mu.Lock()
	defer world.mu.Unlock()
	islandID, ok := world.where[client.CommanderID]
	if !ok {
		return 0, false
	}
	presence := world.islands[islandID].players[client.CommanderID]
	return islandID, presence != nil && presence.client == client
}

// leaveIsland takes the client's commander off the island it is on, if this
// connection is the one there: its seats are freed and the others see it
// leave (SC_21207, SC_21206).
func (game *Game) leaveIsland(ctx context.Context, client *connection.Client) {
	id := client.CommanderID
	world := &game.islands
	world.mu.Lock()
	islandID, ok := world.where[id]
	room := world.islands[islandID]
	if !ok || room == nil || room.players[id] == nil || room.players[id].client != client {
		world.mu.Unlock()
		return
	}
	mapID := room.players[id].mapID
	delete(room.players, id)
	delete(world.where, id)
	var freed []*protobuf.PB_OBJECT
	for _, object := range room.objects {
		kept := object.Slots[:0]
		for _, slot := range object.Slots {
			if slot.GetOwnerId() != id {
				kept = append(kept, slot)
			}
		}
		if len(kept) != len(object.Slots) {
			object.Slots = kept
			freed = append(freed, proto.Clone(object).(*protobuf.PB_OBJECT))
		}
	}
	if len(room.players) == 0 {
		delete(world.islands, islandID)
	}
	world.mu.Unlock()
	if len(freed) > 0 {
		game.tellIsland(client, islandID, &protobuf.SC_21207{IslandId: proto.Uint32(islandID), ObjectList: freed})
	}
	game.tellIsland(client, islandID, &protobuf.SC_21206{IslandId: proto.Uint32(islandID), PlayerList: []*protobuf.PB_PLAYER{{
		Id: proto.Uint32(id), State: proto.Uint32(islandPlayerExit), MapId: proto.Uint32(mapID),
	}}})
	_ = ctx
}

// tellIsland pushes msg to the other players on an island.
func (game *Game) tellIsland(client *connection.Client, islandID uint32, msg proto.Message) {
	game.tellIslandScene(client, islandID, 0, msg)
}

// tellIslandScene pushes msg to the other players on an island, only those
// in scene mapID unless it is 0.
func (game *Game) tellIslandScene(client *connection.Client, islandID, mapID uint32, msg proto.Message) {
	world := &game.islands
	world.mu.Lock()
	var targets []*connection.Client
	if room := world.islands[islandID]; room != nil {
		for id, presence := range room.players {
			if id != client.CommanderID && (mapID == 0 || presence.mapID == mapID) {
				targets = append(targets, presence.client)
			}
		}
	}
	world.mu.Unlock()
	for _, target := range targets {
		err := target.Run(func(target *connection.Client) error { return target.Push(msg) })
		if err != nil {
			logger.WithFields("Island", logger.FieldValue("commander", target.CommanderID), logger.FieldValue("error", err)).Warn("push not delivered")
		}
	}
}

// enterReply answers an entry (CS_21202, CS_21208): onto the island, or why
// not (EnterIslandCommand: 1 island_visit_tip1, 9 island_visit_tip3, 20
// and 40 island_visit_tip2, 19 a cooldown, 6 a queue at pos).
func (game *Game) enterIsland(ctx context.Context, client *connection.Client, islandID uint32, code string) (*protobuf.SC_21203, error) {
	reply := &protobuf.SC_21203{IslandId: proto.Uint32(islandID)}
	refusal := uint32(0)
	err := game.tx(ctx, client, func(tx *Tx) error {
		if islandID == 0 && code != "" {
			owner, err := tx.GetIslandByInviteCode(ctx, code)
			if errors.Is(err, sql.ErrNoRows) {
				refusal = islandVisitBadCode
				return nil
			}
			if err != nil {
				return err
			}
			islandID = owner
			reply.IslandId = proto.Uint32(islandID)
		}
		if islandID == client.CommanderID {
			// creates it the first time
			_, err := tx.island()
			return err
		}
		owner, err := tx.otherIsland(islandID)
		if err != nil {
			return err
		}
		if owner == nil {
			refusal = resultFailed
			return nil
		}
		refusal, err = game.islandVisitAllowed(tx, client, owner, code != "")
		return err
	})
	if err != nil {
		return nil, err
	}
	if refusal != 0 {
		reply.Result = proto.Uint32(refusal)
		return reply, nil
	}
	players, err := game.joinIsland(ctx, client, islandID)
	if err != nil {
		return nil, err
	}
	reply.Result = proto.Uint32(0)
	reply.PlayerList = players
	return reply, nil
}

// handleIslandEnter answers EnterIslandCommand (CS_21202): onto the
// commander's own island (created at the first entry), or another's by id
// or invite code when its access rules let the commander in
// (islandVisitAllowed). The client then asks for the island's data
// (CS_21200) and its first scene (CS_21213). belfast never queues anyone.
func (game *Game) handleIslandEnter(client *connection.Client, req *protobuf.CS_21202) error {
	reply, err := game.enterIsland(context.Background(), client, req.GetIslandId(), req.GetCode())
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandQueue answers IslandQueueUpMsgBox's retries (CS_21208, a
// queue's position polled every few seconds) like an entry: belfast never
// queues, so it only comes from a client that was told to wait elsewhere.
func (game *Game) handleIslandQueue(client *connection.Client, req *protobuf.CS_21208) error {
	reply, err := game.enterIsland(context.Background(), client, req.GetIslandId(), "")
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandData answers GetIslandDataCommand (CS_21200): the commander's
// own island whole with where it last left it, or another's public data
// (SharedIsland) for a visit or a look at its card. An island nobody has yet
// is a new one.
func (game *Game) handleIslandData(client *connection.Client, req *protobuf.CS_21200) error {
	ctx := context.Background()
	reply := &protobuf.SC_21201{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.otherIsland(req.GetIslandId())
		if err != nil {
			return err
		}
		if state == nil {
			state = newIsland(req.GetIslandId(), uint32(serverNow().Unix()))
		}
		if state.commanderID == client.CommanderID {
			// the daily and weekly task offers of the day
			if _, _, err := refreshIslandTasks(state, serverNow()); err != nil {
				return err
			}
			// the characters' daily greetings and skills
			islandShipsDaily(state, serverNow())
			reply.Island = state.island
			reply.PlayerPosition = state.position
			return nil
		}
		reply.Island = &protobuf.PB_ISLAND{PublicData: state.public()}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandEnterMap answers EnterMapCommand (CS_21213): the commander
// loads a scene of the island it is on. The reply has the scene's objects
// someone sits on or changed, the wild resources and collection fragments
// to pick up (islandMapGathers) and the event characters about
// (islandMapNpcs); the others see it change scene (SC_21206).
func (game *Game) handleIslandEnterMap(client *connection.Client, req *protobuf.CS_21213) error {
	ctx := context.Background()
	islandID, mapID := req.GetIslandId(), req.GetMapId()
	config, ok := islandMaps[mapID]
	current, here := game.islandOf(client)
	if !ok || !config.Time.Open(time.Now()) || !here || current != islandID {
		return client.Send(&protobuf.SC_21214{Result: result(errRefused)})
	}
	reply := &protobuf.SC_21214{Result: proto.Uint32(0)}
	world := &game.islands
	world.mu.Lock()
	room := world.islands[islandID]
	room.players[client.CommanderID].mapID = mapID
	for _, key := range slices.SortedFunc(maps.Keys(room.objects), func(a, b [2]uint32) int {
		if a[0] != b[0] {
			return int(a[0]) - int(b[0])
		}
		return int(a[1]) - int(b[1])
	}) {
		reply.ObjectList = append(reply.ObjectList, proto.Clone(room.objects[key]).(*protobuf.PB_OBJECT))
	}
	world.mu.Unlock()
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.otherIsland(islandID)
		if err != nil || state == nil {
			return err
		}
		if err := islandMapGathers(tx, state, mapID, reply); err != nil {
			return err
		}
		return islandMapNpcs(tx, state, mapID, reply)
	})
	if err != nil {
		return err
	}
	player, err := game.islandPlayer(ctx, client.CommanderID, islandPlayerUpdate, mapID)
	if err != nil {
		return err
	}
	game.tellIsland(client, islandID, &protobuf.SC_21206{IslandId: proto.Uint32(islandID), PlayerList: []*protobuf.PB_PLAYER{player}})
	return client.Send(reply)
}

// handleIslandExit answers ExitIslandCommand (CS_21204): off the island.
func (game *Game) handleIslandExit(client *connection.Client, req *protobuf.CS_21204) error {
	game.leaveIsland(context.Background(), client)
	return client.Send(&protobuf.SC_21205{Result: proto.Uint32(0)})
}

// handleIslandHeartbeat takes IslandHearBeatMgr's beat (CS_21215, every
// IslandConst.HEART_BEAT_INTERVAL seconds on an island, no reply). The
// visitors' log pushes (SC_21216) are sent as visits happen.
func (game *Game) handleIslandHeartbeat(client *connection.Client, req *protobuf.CS_21215) error {
	world := &game.islands
	world.mu.Lock()
	defer world.mu.Unlock()
	if room := world.islands[req.GetIslandId()]; room != nil {
		if presence := room.players[client.CommanderID]; presence != nil && presence.client == client {
			presence.seen = time.Now()
		}
	}
	return nil
}

// handleIslandSync relays IslandSyncCommand (CS_21211, the commander's
// position and animation, sent only while someone else is on the island,
// IslandSyncMgr.UpdateLocalPlayer) to the others in its scene (SC_21212).
//
//belfast:static
func (game *Game) handleIslandSync(client *connection.Client, req *protobuf.CS_21211) error {
	islandID, here := game.islandOf(client)
	if !here || islandID != req.GetIslandId() || len(req.GetSyncObList()) == 0 {
		return nil
	}
	game.tellIslandScene(client, islandID, game.islandMapOf(client.CommanderID), &protobuf.SC_21212{SyncObList: req.GetSyncObList()})
	return nil
}

// handleIslandSyncControl answers IslandSyncControlCommand (CS_21209,
// IslandSyncMgr.ControlUnit): the commander takes (op 1) or leaves (op 0) a
// seat (slot) of a scene object (a bench, a plaza furniture: type
// IslandConst.SYNC_TYPE_*), setting its status. A seat someone else holds is
// refused (result 1); the others see the object change (SC_21207).
func (game *Game) handleIslandSyncControl(client *connection.Client, req *protobuf.CS_21209) error {
	islandID, here := game.islandOf(client)
	if !here || islandID != req.GetIslandId() {
		return client.Send(&protobuf.SC_21210{Result: result(errRefused)})
	}
	id := client.CommanderID
	world := &game.islands
	world.mu.Lock()
	room := world.islands[islandID]
	key := [2]uint32{req.GetType(), req.GetObjId()}
	object := room.objects[key]
	if object == nil {
		object = &protobuf.PB_OBJECT{Id: proto.Uint32(req.GetObjId()), Type: proto.Uint32(req.GetType()), Status: proto.Uint32(0)}
		room.objects[key] = object
	}
	refused := false
	kept := object.Slots[:0]
	for _, slot := range object.Slots {
		switch {
		case slot.GetSlotId() == req.GetSlotId() && slot.GetOwnerId() != id && req.GetOp() == 1:
			refused = true
			kept = append(kept, slot)
		case slot.GetOwnerId() == id && (req.GetOp() == 0 || slot.GetSlotId() != req.GetSlotId()):
			// left, or moved to another seat
		case slot.GetSlotId() == req.GetSlotId() && slot.GetOwnerId() == id:
		default:
			kept = append(kept, slot)
		}
	}
	object.Slots = kept
	if !refused {
		if req.GetOp() == 1 {
			object.Slots = append(object.Slots, &protobuf.PB_SLOT{SlotId: proto.Uint32(req.GetSlotId()), OwnerId: proto.Uint32(id)})
		}
		object.Status = proto.Uint32(req.GetStatus())
	}
	update := proto.Clone(object).(*protobuf.PB_OBJECT)
	world.mu.Unlock()
	if refused {
		return client.Send(&protobuf.SC_21210{Result: result(errRefused)})
	}
	game.tellIsland(client, islandID, &protobuf.SC_21207{IslandId: proto.Uint32(islandID), ObjectList: []*protobuf.PB_OBJECT{update}})
	return client.Send(&protobuf.SC_21210{Result: proto.Uint32(0)})
}

// handleIslandRecordPosition stores where the commander stands on its own
// island (IslandRecordLastPositionCommmand, CS_21229, no reply), where it
// comes back next time (SC_21201 player_position). A scene island_map
// doesn't save (save_transform 0) is not sent by the client.
func (game *Game) handleIslandRecordPosition(client *connection.Client, req *protobuf.CS_21229) error {
	if req.GetIslandId() != client.CommanderID {
		return nil
	}
	if _, ok := islandMaps[req.GetPlayerPosition().GetMapId()]; !ok {
		return nil
	}
	return game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		state.position = req.GetPlayerPosition()
		return nil
	})
}

// handleIslandReconnect answers IslandReconnectCommand (CS_21230, after the
// connection came back while on an island): 0 when the commander is still
// there on this connection, else non-zero and the client enters again
// (GAME.ISLAND_ON_RECONNECT).
func (game *Game) handleIslandReconnect(client *connection.Client, req *protobuf.CS_21230) error {
	islandID, here := game.islandOf(client)
	if !here || islandID != req.GetIslandId() {
		return client.Send(&protobuf.SC_21231{Result: result(errRefused)})
	}
	return client.Send(&protobuf.SC_21231{Result: proto.Uint32(0)})
}
