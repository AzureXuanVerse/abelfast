package game

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The play room lobby (protocol p23, PlayRoomProxy, view/playroom): the
// rooms the island's mini-games are played from (mode_room game types,
// PlayRoomConst.GAME_TYPE: 101 the cheat bar, 105-107 the auctions). It
// lives in memory only: rooms die when the last commander leaves, a
// commander whose connection closes leaves its room (a game running goes on
// without it, played by the server, so the client can come back to it:
// PlayRoomEntranceMediator reloads a PLAYING room's game). Built from the
// Lua and the tables alone, no official capture has any of it.
//
// What the client relies on:
//   - a room's id is its host's commander id (PlayRoomTools.GetHostID), so
//     the room is renamed when its host leaves;
//   - team_list has one PTGC_TEAM per mode_room.count entry, each of count
//     user ids (0 for an empty seat); a player in player_list but in no team
//     is a spectator (PlayRoomInfoData.viewerList);
//   - ready_list leaves the host out: the client adds it itself, and starts
//     only when every seated player is in it (PlayRoomTools.CanStartGame);
//   - PTGC_PLAYER.user_view holds a view for the game played
//     (PlayRoomTools.GetGameViewID asserts on a missing one).
//
// Room types (PlayRoomConst.PLAY_ROOM_TYPE): COMMON rooms are listed and
// open, PERSON rooms are listed but only joined on an invitation, MATCH
// rooms are a single commander's ranked matchmaking queue entry: CS_23017
// queues it, a full table (SC_23095, accepted with CS_23019 within
// match_refuseCD) merges the entries into one room, then the load
// (SC_23094, CS_23027) and the game.

// PlayRoomConst.PLAY_ROOM_TYPE
const (
	playRoomCommon = 1
	playRoomPerson = 2
	playRoomMatch  = 3
)

// PlayRoomConst.PLAY_ROOM_STATE, PTGC_ROOM.play_flag
const (
	playRoomStateWait    = 0
	playRoomStatePlaying = 1
	playRoomStateFull    = 2
)

// SC_23096.reason: 1 kicked (match_ui_room_out), 3 back to matchmaking
// (the client creates a new MATCH room and starts it again); anything else
// closes the room.
const (
	playRoomOutKicked  = 1
	playRoomOutClosed  = 2
	playRoomOutRequeue = 3
)

// results the play room commands tell apart
var (
	errPlayRoomFull      error = refusal{"play room full", 6}            // match_room_full2
	errPlayRoomPunished  error = refusal{"matchmaking cooldown", 19}     // PlayRoomTools.ShowPunishementBox(cd)
	errPlayRoomNoneFound error = refusal{"no room for a quick join", 20} // PLAY_ROOM_JOIN_ROOM_QUICK_FAIL
)

// playLoadTime is how long the players get to load the game scene
// (SC_23094.time): a GUESS, the auction's auction_match_loading_timeout is
// 15 s and the cheat bar has no setting of its own; whoever hasn't loaded
// by then starts delegated.
const playLoadTime = 30 * time.Second

// playRankSize is how many commanders CS_23025 ranks: a GUESS.
const playRankSize = 100

// playBotBase is where the bots' ids start, far above any commander id.
const playBotBase = 0xF0000000

// mode_room row: a game type's seats (count per team) and spectator seats,
// and the activity type its season score is kept in
type modeRoomConfig struct {
	ActivityType uint32   `json:"activity_type"`
	Count        []uint32 `json:"count"`
	ViewerCount  uint32   `json:"viewer_count"`
}

// island_integral_rank row: a score bracket and what each rank of a ranked
// game adds to a score in it (integral1 the winner's)
type integralRankConfig struct {
	LowerLimit uint32 `json:"lower_limit"`
	UpperLimit uint32 `json:"upper_limit"`
	Integral1  int32  `json:"integral1"`
	Integral2  int32  `json:"integral2"`
	Integral3  int32  `json:"integral3"`
	Integral4  int32  `json:"integral4"`
}

// nameCodePattern is a name_code reference in a config name.
var nameCodePattern = regexp.MustCompile(`\{namecode:(\d+)\}`)

// island_chara_template row, for the bots' looks and names
type islandCharaConfig struct {
	Name string `json:"name"`
}

var (
	modeRooms      map[uint32]modeRoomConfig
	integralRanks  []integralRankConfig // by lower_limit
	integralRankID []uint32             // their ids, same order
	islandCharas   map[uint32]islandCharaConfig
	islandBotLooks []uint32 // island_chara_template ids a bot can sit as
	// playRoomsOn is whether the region has the lobby and the Cheater's
	// Tavern: TW's gameset has none of their settings (bar_*, barbot_*,
	// match_*), which its client reads as it opens them, so there no room
	// is ever created (CS_23005 refused) and nothing else can start
	playRoomsOn bool
)

// playRoomRules are the gameset settings of the lobby and matchmaking.
var playRoomRules struct {
	defaultShip uint32        // bar_default_character, the view without a CS_23029
	roomMax     int           // game_room_remax, rooms at once
	refuseTime  time.Duration // match_refuseCD: to accept a table (the slider of PlayRoomInfoScene), and between two invitations to the same commander
	penalty     time.Duration // match_time_limit: GUESS, the matchmaking cooldown after refusing a table or quitting a ranked game
	matchGap    uint32        // match_expand: GUESS, the score gap a table starts with
	gapGrowth   [][2]uint32   // match_time_expand [seconds waited, gap added]: GUESS
	botWait     [][3]uint32   // barbot_match_waittime [integral rank id, min s, max s]: GUESS, the wait before bots fill a table
}

func loadPlayRoomData() error {
	var ranks map[uint32]integralRankConfig
	var gameset map[string]uint32
	var growth, botWait [][]uint32
	var bar cheatBarSettings
	var nameCodes map[uint32]struct {
		Name string `json:"name"`
	}
	err := gamedata.LoadAll(
		gamedata.Load("name_code", &nameCodes),
		gamedata.Load("mode_room", &modeRooms),
		gamedata.Load("island_integral_rank", &ranks),
		gamedata.Load("island_chara_template", &islandCharas),
		func() (err error) { gameset, err = gamedata.Gameset(); return err },
		func() (err error) {
			playRoomsOn, err = gamedata.Feature(
				func() error { return gamedata.GamesetDescription("match_time_expand", &growth) },
				func() error { return gamedata.GamesetDescription("barbot_match_waittime", &botWait) },
				func() error { return gamedata.GamesetDescription("bar_card", &bar.deck) },
				func() error { return gamedata.GamesetDescription("barbot_card_time", &bar.cardTime) },
				func() error { return gamedata.GamesetDescription("barbot_punishment_rotate_time", &bar.shootTime) },
				func() error { return gamedata.GamesetDescription("barbot_increase_probability_M", &bar.claim) },
			)
			return err
		},
	)
	if err != nil || !playRoomsOn {
		return err
	}
	if err := loadCheatBarData(gameset, bar); err != nil {
		return err
	}
	if len(ranks) == 0 {
		return gamedata.Missing("island_integral_rank", "rows")
	}
	integralRanks, integralRankID = nil, nil
	ids := make([]uint32, 0, len(ranks))
	for id := range ranks {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uint32) int { return cmp.Compare(ranks[a].LowerLimit, ranks[b].LowerLimit) })
	for _, id := range ids {
		integralRanks = append(integralRanks, ranks[id])
		integralRankID = append(integralRankID, id)
	}
	islandBotLooks = nil
	for id, chara := range islandCharas {
		// names written "{namecode:N}", which the client resolves only in
		// its own tables (include.lua __namecode__), not in a player name
		chara.Name = nameCodePattern.ReplaceAllStringFunc(chara.Name, func(code string) string {
			n, _ := strconv.ParseUint(nameCodePattern.FindStringSubmatch(code)[1], 10, 32)
			if row, ok := nameCodes[uint32(n)]; ok {
				return row.Name
			}
			return code
		})
		islandCharas[id] = chara
		// the ship characters (ids are ship groups); the low ids are the
		// island's workers
		if id >= 10000 && chara.Name != "" {
			islandBotLooks = append(islandBotLooks, id)
		}
	}
	slices.Sort(islandBotLooks)
	rules := &playRoomRules
	rules.defaultShip = gameset["bar_default_character"]
	if _, ok := islandCharas[rules.defaultShip]; !ok {
		return gamedata.Missing("island_chara_template", "bar_default_character %d", rules.defaultShip)
	}
	rules.roomMax = int(gameset["game_room_remax"])
	rules.refuseTime = time.Duration(gameset["match_refuseCD"]) * time.Second
	rules.penalty = time.Duration(gameset["match_time_limit"]) * time.Second
	rules.matchGap = gameset["match_expand"]
	rules.gapGrowth = nil
	for _, row := range growth {
		if len(row) == 2 {
			rules.gapGrowth = append(rules.gapGrowth, [2]uint32{row[0], row[1]})
		}
	}
	rules.botWait = nil
	for _, row := range botWait {
		if len(row) == 3 && row[1] <= row[2] {
			rules.botWait = append(rules.botWait, [3]uint32{row[0], row[1], row[2]})
		}
	}
	if len(rules.botWait) == 0 {
		return gamedata.Missing("gameset", "barbot_match_waittime rows")
	}
	return nil
}

// integralRank is the island_integral_rank bracket a score is in, the top
// one above its upper_limit.
func integralRank(score uint32) (uint32, integralRankConfig) {
	at := 0
	for i, row := range integralRanks {
		if score >= row.LowerLimit {
			at = i
		}
	}
	return integralRankID[at], integralRanks[at]
}

// integralDelta is what a ranked game's rank (1 the winner) adds to score.
func integralDelta(score, rank uint32) int64 {
	_, row := integralRank(score)
	switch rank {
	case 1:
		return int64(row.Integral1)
	case 2:
		return int64(row.Integral2)
	case 3:
		return int64(row.Integral3)
	}
	return int64(row.Integral4)
}

// playPhase is where a room is.
type playPhase int

const (
	playWait     playPhase = iota // in the lobby
	playMatching                  // a MATCH room queued for a table
	playReady                     // a table found, waiting for its players to accept (SC_23095)
	playLoading                   // loading the game scene (SC_23094)
	playPlaying                   // the game runs
)

type playRoom struct {
	id       uint32 // its host's commander id
	kind     uint32 // PLAY_ROOM_TYPE
	gameType uint32
	created  time.Time
	members  map[uint32]*protobuf.PTGC_PLAYER // everyone in it, bots included
	order    []uint32                         // members by arrival
	teams    [][]uint32                       // seats by team, 0 empty
	ready    map[uint32]bool
	invited  map[uint32]bool // who may join a PERSON room
	phase    playPhase

	matchSince time.Time     // playMatching: queued at
	score      uint32        // playMatching: its commander's score
	botAfter   time.Duration // playMatching: wait before bots fill its table

	deadline time.Time         // playReady, playLoading
	accepted map[uint32]bool   // playReady
	loads    map[uint32]uint32 // playLoading
	game     *cheatBar         // playPlaying
}

// playRooms is the lobby, guarded by mu; timers run from playRoomLoop.
type playRooms struct {
	mu        sync.Mutex
	rooms     map[uint32]*playRoom // by id
	of        map[uint32]*playRoom // by commander in it
	penalties map[uint32]time.Time // matchmaking cooldowns, until
	invites   map[[2]uint32]time.Time
	hooked    map[*connection.Client]bool
	nextBot   uint32
	loop      sync.Once
}

func (pr *playRooms) init() {
	if pr.rooms == nil {
		pr.rooms = map[uint32]*playRoom{}
		pr.of = map[uint32]*playRoom{}
		pr.penalties = map[uint32]time.Time{}
		pr.invites = map[[2]uint32]time.Time{}
		pr.hooked = map[*connection.Client]bool{}
	}
}

func isPlayBot(id uint32) bool { return id >= playBotBase }

// playOutbox collects what a lobby change sends, delivered once the lobby
// is unlocked, in order: pushes to the requesting client go ahead of (push)
// or behind (after) its reply, others through deliver; do runs work that
// needs the database (scores).
type playOutbox struct {
	game   *Game
	client *connection.Client // the request's, nil from the timers
	steps  []func()
}

func (game *Game) playOutbox(client *connection.Client) *playOutbox {
	return &playOutbox{game: game, client: client}
}

// send pushes msg to commander to right away.
func (box *playOutbox) send(to uint32, msg proto.Message, after bool) {
	if isPlayBot(to) {
		return
	}
	if box.client != nil && to == box.client.CommanderID {
		push := box.client.Push
		if after {
			push = box.client.PushAfterReply
		}
		if err := push(msg); err != nil {
			logger.WithFields("PlayRoom", logger.FieldValue("commander", to), logger.FieldValue("error", err)).Warn("push failed")
		}
		return
	}
	box.game.deliver(nil, to, msg)
}

func (box *playOutbox) push(to uint32, msg proto.Message) {
	box.steps = append(box.steps, func() { box.send(to, msg, false) })
}

func (box *playOutbox) after(to uint32, msg proto.Message) {
	box.steps = append(box.steps, func() { box.send(to, msg, true) })
}

func (box *playOutbox) do(fn func()) { box.steps = append(box.steps, fn) }

func (box *playOutbox) flush() {
	for _, step := range box.steps {
		step()
	}
	box.steps = nil
}

// lockPlayRooms locks the lobby for a request: watches the client's
// connection first (OnClose runs at once on a closed one, which needs the
// lobby unlocked) and starts the timers.
func (game *Game) lockPlayRooms(client *connection.Client) *playRooms {
	pr := &game.playRooms
	pr.loop.Do(func() { go game.playRoomLoop() })
	pr.mu.Lock()
	pr.init()
	hooked := pr.hooked[client]
	pr.hooked[client] = true
	pr.mu.Unlock()
	if !hooked {
		client.OnClose(func() { game.playRoomClosed(client) })
	}
	pr.mu.Lock()
	return pr
}

// playRoomLoop runs the lobby's and the games' timers.
func (game *Game) playRoomLoop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for now := range ticker.C {
		pr := &game.playRooms
		box := game.playOutbox(nil)
		pr.mu.Lock()
		pr.init()
		game.playRoomTick(box, now)
		pr.mu.Unlock()
		box.flush()
	}
}

func (game *Game) playRoomTick(box *playOutbox, now time.Time) {
	pr := &game.playRooms
	game.matchTables(box, now)
	for _, room := range sortedRooms(pr.rooms) {
		switch room.phase {
		case playReady:
			if !now.Before(room.deadline) {
				game.dissolveTable(box, room, now)
			}
		case playLoading:
			if !now.Before(room.deadline) {
				game.startPlayGame(box, room, now)
			}
		case playPlaying:
			if room.game != nil {
				room.game.tick(box, now)
			}
		}
	}
	for id, until := range pr.penalties {
		if now.After(until) {
			delete(pr.penalties, id)
		}
	}
	for key, at := range pr.invites {
		if now.Sub(at) > playRoomRules.refuseTime {
			delete(pr.invites, key)
		}
	}
}

func sortedRooms(rooms map[uint32]*playRoom) []*playRoom {
	list := make([]*playRoom, 0, len(rooms))
	for _, room := range rooms {
		list = append(list, room)
	}
	slices.SortFunc(list, func(a, b *playRoom) int { return a.created.Compare(b.created) })
	return list
}

// playRoomClosed takes a closed connection's commander out of its room,
// unless a newer connection took over; in a game it is played by the server
// until it comes back.
func (game *Game) playRoomClosed(client *connection.Client) {
	pr := &game.playRooms
	box := game.playOutbox(nil)
	pr.mu.Lock()
	pr.init()
	delete(pr.hooked, client)
	id := client.CommanderID
	if current := game.onlineClient(id); current == nil || current == client {
		if room := pr.of[id]; room != nil {
			if room.phase == playPlaying && room.game != nil {
				room.game.delegate(box, id)
			} else {
				game.leavePlayRoom(box, id, time.Now())
			}
		}
	}
	pr.mu.Unlock()
	box.flush()
}

// ptgcPlayer is a commander as the play rooms show it (PlayerData.New):
// level, name, guild, portrait (DISPLAYINFO like a friend's) and the look
// picked for each game type (playRoomViews).
func (game *Game) ptgcPlayer(ctx context.Context, commanderID uint32) (*protobuf.PTGC_PLAYER, error) {
	commander, err := game.DB.GetCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	guildName := ""
	member, ok, err := guildMembership(ctx, game.DB.Queries, commanderID)
	if err != nil {
		return nil, err
	}
	if ok {
		guild, err := game.DB.GetGuild(ctx, member.GuildID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		guildName = guild.Name
	}
	views, err := game.DB.ListPlayRoomViews(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	return &protobuf.PTGC_PLAYER{
		Id:        proto.Uint32(commander.ID),
		Level:     proto.Uint32(commander.Level),
		Name:      proto.String(commander.Name),
		GuildName: proto.String(guildName),
		Display:   game.displayInfo(commander),
		UserView:  playRoomViews(views, 0),
	}, nil
}

// playRoomViews are the PTGC_GAME_USER_VIEW of every mode_room game type:
// the one stored (CS_23029), else bar_default_character (or ship, for a
// bot). The skin, color and dress are the island character's, which
// belfast doesn't keep yet: 0 and none.
func playRoomViews(stored []db.CommanderPlayRoomView, ship uint32) []*protobuf.PTGC_GAME_USER_VIEW {
	if ship == 0 {
		ship = playRoomRules.defaultShip
	}
	byType := map[uint32]db.CommanderPlayRoomView{}
	for _, view := range stored {
		byType[view.GameType] = view
	}
	types := make([]uint32, 0, len(modeRooms))
	for id := range modeRooms {
		if id != 0 {
			types = append(types, id)
		}
	}
	slices.Sort(types)
	views := make([]*protobuf.PTGC_GAME_USER_VIEW, 0, len(types))
	for _, gameType := range types {
		view, ok := byType[gameType]
		if !ok {
			view = db.CommanderPlayRoomView{GameType: gameType, ShipID: ship}
		}
		views = append(views, &protobuf.PTGC_GAME_USER_VIEW{
			GameType:  proto.Uint32(view.GameType),
			ShipId:    proto.Uint32(view.ShipID),
			SkinId:    proto.Uint32(view.SkinID),
			Color:     proto.Uint32(view.Color),
			DressList: view.DressList,
		})
	}
	return views
}

// newPlayBot is a bot seated in room: a random island character's look and
// name, at level (the commander it plays with: a GUESS).
func (pr *playRooms) newPlayBot(level uint32) *protobuf.PTGC_PLAYER {
	pr.nextBot++
	id := playBotBase + pr.nextBot%0x0FFFFFFF
	ship := playRoomRules.defaultShip
	if len(islandBotLooks) > 0 {
		ship = islandBotLooks[rand.IntN(len(islandBotLooks))]
	}
	display := &protobuf.DISPLAYINFO{}
	if _, ok := shipTemplates[ship*10+1]; ok {
		display.Icon = proto.Uint32(ship*10 + 1)
	}
	return &protobuf.PTGC_PLAYER{
		Id:        proto.Uint32(id),
		Level:     proto.Uint32(max(level, 1)),
		Name:      proto.String(islandCharas[ship].Name),
		GuildName: proto.String(""),
		Display:   display,
		UserView:  playRoomViews(nil, ship),
	}
}

// newPlayRoom opens a room hosted by host (seated first).
func (pr *playRooms) newPlayRoom(kind, gameType uint32, host *protobuf.PTGC_PLAYER, now time.Time) *playRoom {
	room := &playRoom{
		id: host.GetId(), kind: kind, gameType: gameType, created: now,
		members: map[uint32]*protobuf.PTGC_PLAYER{},
		ready:   map[uint32]bool{},
		invited: map[uint32]bool{},
	}
	for _, count := range modeRooms[gameType].Count {
		room.teams = append(room.teams, make([]uint32, count))
	}
	room.add(host, true)
	pr.rooms[room.id] = room
	pr.of[room.id] = room
	return room
}

// add puts a player in the room, in the first free seat when seated (a
// spectator otherwise); it tells whether it found one.
func (room *playRoom) add(player *protobuf.PTGC_PLAYER, seated bool) bool {
	id := player.GetId()
	if seated && !room.seat(id, -1) {
		return false
	}
	room.members[id] = player
	room.order = append(room.order, id)
	return true
}

// seat puts id in team's first free seat (any team when team is -1).
func (room *playRoom) seat(id uint32, team int) bool {
	for t, seats := range room.teams {
		if team >= 0 && t != team {
			continue
		}
		for s, seated := range seats {
			if seated == 0 {
				room.teams[t][s] = id
				return true
			}
		}
	}
	return false
}

func (room *playRoom) unseat(id uint32) {
	for t, seats := range room.teams {
		for s, seated := range seats {
			if seated == id {
				room.teams[t][s] = 0
			}
		}
	}
	delete(room.ready, id)
}

func (room *playRoom) remove(id uint32) {
	room.unseat(id)
	delete(room.members, id)
	delete(room.accepted, id)
	delete(room.loads, id)
	room.order = slices.DeleteFunc(room.order, func(member uint32) bool { return member == id })
}

func (room *playRoom) seated(id uint32) bool {
	for _, seats := range room.teams {
		if slices.Contains(seats, id) {
			return true
		}
	}
	return false
}

// players are the seated, by team and seat.
func (room *playRoom) players() []uint32 {
	var ids []uint32
	for _, seats := range room.teams {
		for _, id := range seats {
			if id != 0 {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// humans are the commanders in the room, by arrival.
func (room *playRoom) humans() []uint32 {
	var ids []uint32
	for _, id := range room.order {
		if !isPlayBot(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (room *playRoom) viewers() int { return len(room.order) - len(room.players()) }

func (room *playRoom) full() bool {
	for _, seats := range room.teams {
		if slices.Contains(seats, 0) {
			return false
		}
	}
	return true
}

func (room *playRoom) state() uint32 {
	switch {
	case room.phase == playLoading || room.phase == playPlaying:
		return playRoomStatePlaying
	case room.full():
		return playRoomStateFull
	}
	return playRoomStateWait
}

func (room *playRoom) teamList() []*protobuf.PTGC_TEAM {
	teams := make([]*protobuf.PTGC_TEAM, len(room.teams))
	for i, seats := range room.teams {
		teams[i] = &protobuf.PTGC_TEAM{UserIdList: slices.Clone(seats)}
	}
	return teams
}

func (room *playRoom) playerList(ids []uint32) []*protobuf.PTGC_PLAYER {
	list := make([]*protobuf.PTGC_PLAYER, 0, len(ids))
	for _, id := range ids {
		list = append(list, room.members[id])
	}
	return list
}

// proto is the room as PlayRoomInfoData reads it.
func (room *playRoom) proto() *protobuf.PTGC_ROOM {
	var ready []uint32
	for _, id := range room.players() {
		if room.ready[id] && id != room.id {
			ready = append(ready, id)
		}
	}
	return &protobuf.PTGC_ROOM{
		Id:         proto.Uint32(room.id),
		Type:       proto.Uint32(room.kind),
		GameType:   proto.Uint32(room.gameType),
		PlayerList: room.playerList(room.order),
		TeamList:   room.teamList(),
		ReadyList:  ready,
		PlayFlag:   proto.Uint32(room.state()),
	}
}

// simple is the room in the room list and invitations (PlayRoomData): its
// name is the host's (match_ui_room_name), player_num the seated.
func (room *playRoom) simple() *protobuf.PTGC_ROOM_SIMPLE {
	return &protobuf.PTGC_ROOM_SIMPLE{
		Id:        proto.Uint32(room.id),
		Type:      proto.Uint32(room.kind),
		GameType:  proto.Uint32(room.gameType),
		Name:      proto.String(room.members[room.id].GetName()),
		PlayerNum: proto.Uint32(uint32(len(room.players()))),
		PlayFlag:  proto.Uint32(room.state()),
	}
}

// tellRoom pushes the room (SC_23099) to its commanders but except, the
// requester's own copy ahead of its reply.
func (room *playRoom) tellRoom(box *playOutbox, except uint32) {
	update := &protobuf.SC_23099{Room: room.proto()}
	for _, id := range room.humans() {
		if id != except {
			box.push(id, update)
		}
	}
}

// rehost renames the room after its host when the host left: the first
// commander seated, else the first spectator, seated. It closes the room
// when no commander is left.
func (pr *playRooms) rehost(room *playRoom) bool {
	if _, ok := room.members[room.id]; ok && !isPlayBot(room.id) {
		return true
	}
	host := uint32(0)
	for _, id := range room.players() {
		if !isPlayBot(id) {
			host = id
			break
		}
	}
	if host == 0 {
		for _, id := range room.humans() {
			if room.seat(id, -1) {
				host = id
				break
			}
		}
	}
	if pr.rooms[room.id] == room {
		delete(pr.rooms, room.id)
	}
	if host == 0 {
		for _, id := range room.humans() {
			if pr.of[id] == room {
				delete(pr.of, id)
			}
		}
		return false
	}
	room.id = host
	delete(room.ready, host)
	pr.rooms[host] = room
	return true
}

// punish starts the matchmaking cooldown of id; cooldown is when it ends
// (SC_23012.time, SC_23006.cd: a unix time), 0 without one.
func (pr *playRooms) punish(id uint32, now time.Time) {
	pr.penalties[id] = now.Add(playRoomRules.penalty)
}

func (pr *playRooms) cooldown(id uint32, now time.Time) uint32 {
	if until, ok := pr.penalties[id]; ok && now.Before(until) {
		return uint32(until.Unix())
	}
	return 0
}

// leavePlayRoom takes id out of its room: a lobby room carries on under its
// next host, a queued MATCH room closes, a table being accepted falls apart
// (id refused it: cooldown, the others go back to matchmaking), a game
// being loaded or played goes on with id played by the server (a ranked
// one costs it the last rank's points and a cooldown). It returns the
// cooldown left (SC_23012.time).
func (game *Game) leavePlayRoom(box *playOutbox, id uint32, now time.Time) uint32 {
	pr := &game.playRooms
	room := pr.of[id]
	if room == nil {
		return pr.cooldown(id, now)
	}
	switch {
	case (room.phase == playLoading || room.phase == playPlaying) && !room.seated(id):
		// a spectator of a game
		delete(pr.of, id)
		room.remove(id)
		return pr.cooldown(id, now)
	}
	switch room.phase {
	case playWait:
		delete(pr.of, id)
		room.remove(id)
		if pr.rehost(room) {
			room.tellRoom(box, id)
		}
	case playMatching:
		delete(pr.of, id)
		delete(pr.rooms, room.id)
	case playReady:
		pr.punish(id, now)
		game.breakTable(box, room, map[uint32]bool{id: true}, id)
	case playLoading:
		delete(pr.of, id)
		pr.handOver(box, room, id)
		room.loads[id] = 100
		game.quitRanked(box, room, id, now)
		if !game.anyonePlaying(room) {
			game.closePlayRoom(room)
		} else {
			game.loadProgress(box, room, now)
		}
	case playPlaying:
		delete(pr.of, id)
		pr.handOver(box, room, id)
		if room.game != nil {
			if seat := room.game.seatOf(id); seat != nil && !seat.out {
				room.game.delegate(box, id)
				game.quitRanked(box, room, id, now)
			}
			if seat := room.game.seatOf(id); seat != nil {
				seat.gone = true
			}
		}
		if !game.anyonePlaying(room) {
			game.closePlayRoom(room)
		}
	}
	return pr.cooldown(id, now)
}

// handOver keys a game's room under another commander still in it when its
// host id leaves for good: the room id is the host's (PlayRoomTools
// GetHostID), and the host may open a room of its own, which would take
// the key and leave the game unticked.
func (pr *playRooms) handOver(box *playOutbox, room *playRoom, id uint32) {
	if room.id != id || pr.rooms[room.id] != room {
		return
	}
	for _, other := range room.humans() {
		if other != id && pr.of[other] == room {
			delete(pr.rooms, room.id)
			room.id = other
			pr.rooms[other] = room
			room.tellRoom(box, id)
			return
		}
	}
}

// quitRanked charges a commander leaving a ranked game: the last rank's
// points (bar_tips_game6: eliminated, no points) and a matchmaking
// cooldown, both GUESSes; the new score comes as SC_23117.
func (game *Game) quitRanked(box *playOutbox, room *playRoom, id uint32, now time.Time) {
	if room.kind != playRoomMatch {
		return
	}
	game.playRooms.punish(id, now)
	gameType := room.gameType
	rank := uint32(len(room.players()))
	box.do(func() {
		score, err := game.addPlayRoomScore(context.Background(), id, gameType, rank, true)
		if err != nil {
			logger.WithFields("PlayRoom", logger.FieldValue("commander", id), logger.FieldValue("error", err)).Warn("score not saved")
			return
		}
		box.send(id, &protobuf.SC_23117{CurScore: proto.Uint32(score)}, true)
	})
}

// anyonePlaying tells whether a commander still takes part in the room's
// load or game.
func (game *Game) anyonePlaying(room *playRoom) bool {
	for _, id := range room.humans() {
		if game.playRooms.of[id] == room {
			return true
		}
	}
	return false
}

func (game *Game) closePlayRoom(room *playRoom) {
	pr := &game.playRooms
	for _, id := range room.humans() {
		if pr.of[id] == room {
			delete(pr.of, id)
		}
	}
	if pr.rooms[room.id] == room {
		delete(pr.rooms, room.id)
	}
	room.game = nil
}

// matchTables seats the queued MATCH rooms of each game type at tables, the
// longest waiting first: commanders whose scores are within the gap of the
// first one's wait (match_expand, widened by match_time_expand), bots in
// the empty seats once its barbot_match_waittime is over.
func (game *Game) matchTables(box *playOutbox, now time.Time) {
	pr := &game.playRooms
	var queue []*playRoom
	for _, room := range sortedRooms(pr.rooms) {
		if room.phase == playMatching {
			queue = append(queue, room)
		}
	}
	taken := map[*playRoom]bool{}
	for i, first := range queue {
		if taken[first] {
			continue
		}
		seats := 0
		for _, count := range modeRooms[first.gameType].Count {
			seats += int(count)
		}
		wait := now.Sub(first.matchSince)
		gap := playRoomRules.matchGap
		for _, step := range playRoomRules.gapGrowth {
			if wait >= time.Duration(step[0])*time.Second {
				gap = playRoomRules.matchGap + step[1]
			}
		}
		table := []*playRoom{first}
		for _, other := range queue[i+1:] {
			if len(table) == seats {
				break
			}
			if taken[other] || other.gameType != first.gameType {
				continue
			}
			diff := max(other.score, first.score) - min(other.score, first.score)
			if diff <= gap {
				table = append(table, other)
			}
		}
		if len(table) < seats && wait < first.botAfter {
			continue
		}
		for _, room := range table {
			taken[room] = true
		}
		game.seatTable(box, table, now)
	}
}

// seatTable merges the MATCH rooms of a table into the first one's, with
// bots in the seats left, and asks its commanders to accept (SC_23095).
func (game *Game) seatTable(box *playOutbox, rooms []*playRoom, now time.Time) {
	pr := &game.playRooms
	table := rooms[0]
	for _, room := range rooms[1:] {
		delete(pr.rooms, room.id)
		player := room.members[room.id]
		table.add(player, true)
		pr.of[room.id] = table
	}
	host := table.members[table.id]
	for !table.full() {
		if !table.add(pr.newPlayBot(host.GetLevel()), true) {
			break
		}
	}
	table.phase = playReady
	table.deadline = now.Add(playRoomRules.refuseTime)
	table.accepted = map[uint32]bool{}
	for _, id := range table.players() {
		if isPlayBot(id) {
			table.accepted[id] = true
		}
	}
	table.tellTable(box)
}

// tellTable pushes the table to accept (MatchReadyRoom) to its commanders.
func (room *playRoom) tellTable(box *playOutbox) {
	var accepted []uint32
	for _, id := range room.players() {
		if room.accepted[id] {
			accepted = append(accepted, id)
		}
	}
	msg := &protobuf.SC_23095{
		GameType:   proto.Uint32(room.gameType),
		PlayerList: room.playerList(room.players()),
		TeamList:   room.teamList(),
		ReadyList:  accepted,
		Time:       proto.Uint32(uint32(room.deadline.Unix())),
	}
	for _, id := range room.humans() {
		box.push(id, msg)
	}
}

// dissolveTable ends a table whose time to accept ran out: those who didn't
// accept get the cooldown.
func (game *Game) dissolveTable(box *playOutbox, room *playRoom, now time.Time) {
	refused := map[uint32]bool{}
	for _, id := range room.humans() {
		if !room.accepted[id] {
			refused[id] = true
			game.playRooms.punish(id, now)
		}
	}
	game.breakTable(box, room, refused, 0)
}

// breakTable closes a table: the refusers' rooms close (SC_23096), the
// others go back to matchmaking (reason 3, the client opens and starts a
// new MATCH room); leaving is the commander who left, told by its reply.
func (game *Game) breakTable(box *playOutbox, room *playRoom, refused map[uint32]bool, leaving uint32) {
	for _, id := range room.humans() {
		if id == leaving {
			continue
		}
		reason := uint32(playRoomOutRequeue)
		if refused[id] {
			reason = playRoomOutClosed
		}
		box.push(id, &protobuf.SC_23096{Reason: proto.Uint32(reason)})
	}
	game.closePlayRoom(room)
}

// startLoading has the room's players load the game (SC_23094), bots
// loaded already.
func (game *Game) startLoading(box *playOutbox, room *playRoom, now time.Time) {
	room.phase = playLoading
	room.deadline = now.Add(playLoadTime)
	room.loads = map[uint32]uint32{}
	room.ready = map[uint32]bool{}
	for _, id := range room.players() {
		if isPlayBot(id) {
			room.loads[id] = 100
		} else {
			room.loads[id] = 0
		}
	}
	for _, id := range room.humans() {
		if !room.seated(id) {
			box.push(id, &protobuf.SC_23099{Room: room.proto()})
		}
	}
	game.loadProgress(box, room, now)
}

// loadList is SC_23094: every player's load, 100 for all once over.
func (room *playRoom) loadList(all bool) *protobuf.SC_23094 {
	msg := &protobuf.SC_23094{
		GameType:   proto.Uint32(room.gameType),
		PlayerList: room.playerList(room.players()),
		TeamList:   room.teamList(),
		Time:       proto.Uint32(uint32(room.deadline.Unix())),
	}
	for _, id := range room.players() {
		load := room.loads[id]
		if all {
			load = 100
		}
		msg.LoadList = append(msg.LoadList, &protobuf.PTGC_LOAD{UserId: proto.Uint32(id), Load: proto.Uint32(load)})
	}
	return msg
}

// loadProgress pushes the loads to the seated commanders, and starts the
// game once everyone loaded.
func (game *Game) loadProgress(box *playOutbox, room *playRoom, now time.Time) {
	for _, id := range room.players() {
		if room.loads[id] < 100 {
			msg := room.loadList(false)
			for _, human := range room.players() {
				if game.playRooms.of[human] == room {
					box.push(human, msg)
				}
			}
			return
		}
	}
	game.startPlayGame(box, room, now)
}

// startPlayGame starts the game of a loading room: whoever hasn't loaded
// starts delegated.
func (game *Game) startPlayGame(box *playOutbox, room *playRoom, now time.Time) {
	msg := room.loadList(true)
	for _, id := range room.players() {
		if game.playRooms.of[id] == room {
			box.push(id, msg)
		}
	}
	room.phase = playPlaying
	room.game = newCheatBar(game, room, now)
	room.game.start(box, now)
}

// endPlayGame takes a room back to its lobby once its game is over: the
// bots and whoever left stand up, the others stay until they leave (the
// finish page's Quit sends CS_23011).
func (game *Game) endPlayGame(box *playOutbox, room *playRoom) {
	pr := &game.playRooms
	room.game = nil
	room.phase = playWait
	room.ready = map[uint32]bool{}
	for _, id := range slices.Clone(room.order) {
		if isPlayBot(id) || pr.of[id] != room || !game.isOnline(id) {
			if pr.of[id] == room {
				delete(pr.of, id)
			}
			room.remove(id)
		}
	}
	if pr.rehost(room) {
		room.tellRoom(box, 0)
	}
}

// playRoomSeason is the activity a game type's season runs in
// (mode_room.activity_type, PlayRoomTools.GetPtScrore reads its data1).
func playRoomSeason(gameType uint32, now time.Time) (activityConfig, bool) {
	kind := modeRooms[gameType].ActivityType
	if kind == 0 {
		return activityConfig{}, false
	}
	for _, config := range openActivities(now) {
		if config.Type == kind {
			return config, true
		}
	}
	return activityConfig{}, false
}

// addPlayRoomScore applies a game's rank (1 the winner) to the commander's
// season score when ranked (island_integral_rank of the score's bracket,
// never below 0) and returns the score; the season's activity keeps it in
// data1, the best in data2 (IslandCheaterTavernMonitor's SC_23108).
func (game *Game) addPlayRoomScore(ctx context.Context, commanderID, gameType, rank uint32, ranked bool) (uint32, error) {
	season, open := playRoomSeason(gameType, time.Now())
	var score uint32
	err := game.DB.InTx(ctx, func(q *db.Queries) error {
		row, err := q.GetPlayRoomScore(ctx, db.GetPlayRoomScoreParams{CommanderID: commanderID, GameType: gameType, Season: season.ID})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		score = row.Score
		if !ranked {
			return nil
		}
		score = uint32(max(0, int64(row.Score)+integralDelta(row.Score, rank)))
		best := max(row.MaxScore, score)
		err = q.SetPlayRoomScore(ctx, db.SetPlayRoomScoreParams{CommanderID: commanderID, GameType: gameType, Season: season.ID, Score: score, MaxScore: best})
		if err != nil || !open {
			return err
		}
		state, err := activityState(ctx, q, commanderID, season)
		if err != nil {
			return err
		}
		state.Data1, state.Data2 = score, max(state.Data2, best)
		return q.SaveActivity(ctx, db.SaveActivityParams(state))
	})
	return score, err
}

// handlePlayRoomList answers PlayRoomRefreshRoomCommand (CS_23001): the
// rooms open to join, COMMON and PERSON ones (PlayRoomMainScene filters and
// sorts them itself), by creation.
func (game *Game) handlePlayRoomList(client *connection.Client, req *protobuf.CS_23001) error {
	pr := game.lockPlayRooms(client)
	reply := &protobuf.SC_23002{}
	for _, room := range sortedRooms(pr.rooms) {
		if room.kind != playRoomMatch {
			reply.RoomList = append(reply.RoomList, room.simple())
		}
	}
	pr.mu.Unlock()
	return client.Send(reply)
}

// handlePlayRoomInfo answers PlayRoomRefreshRoomInfoCommand (CS_23003): the
// commander's room, id 0 outside one. A PLAYING room reloads its game
// (PlayRoomEntranceMediator).
func (game *Game) handlePlayRoomInfo(client *connection.Client, req *protobuf.CS_23003) error {
	pr := game.lockPlayRooms(client)
	room := &protobuf.PTGC_ROOM{Id: proto.Uint32(0)}
	if own := pr.of[client.CommanderID]; own != nil {
		room = own.proto()
	}
	pr.mu.Unlock()
	return client.Send(&protobuf.SC_23004{Result: result(nil), Room: room})
}

// handlePlayRoomCreate answers PlayRoomCreateRoomCommand and
// PlayRoomSwitchRoomTypeCommand (CS_23005): a new room of the type, hosted
// by the commander (who leaves the room it was in), or its own lobby room
// switched between COMMON and PERSON. A MATCH room during the matchmaking
// cooldown gets result 19 with when it ends; past game_room_remax rooms the
// request is refused.
func (game *Game) handlePlayRoomCreate(client *connection.Client, req *protobuf.CS_23005) error {
	ctx := context.Background()
	now := time.Now()
	kind, gameType := req.GetType(), req.GetGameType()
	if _, ok := modeRooms[gameType]; !playRoomsOn || !ok || gameType == 0 || kind < playRoomCommon || kind > playRoomMatch ||
		(kind == playRoomMatch && !cheatBarGame(gameType)) {
		return client.Send(&protobuf.SC_23006{Result: result(errRefused)})
	}
	host, err := game.ptgcPlayer(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	id := client.CommanderID
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	reply, err := func() (*protobuf.SC_23006, error) {
		if room := pr.of[id]; room != nil {
			if room.id == id && room.phase == playWait && room.kind != playRoomMatch && kind != playRoomMatch && room.gameType == gameType {
				room.kind = kind
				room.tellRoom(box, id)
				return &protobuf.SC_23006{Result: result(nil), Room: room.proto()}, nil
			}
			if room.phase != playWait && room.phase != playMatching {
				return nil, errRefused
			}
			game.leavePlayRoom(box, id, now)
		}
		if kind == playRoomMatch {
			if cd := pr.cooldown(id, now); cd != 0 {
				return &protobuf.SC_23006{Result: result(errPlayRoomPunished), Cd: proto.Uint32(cd)}, nil
			}
		}
		if len(pr.rooms) >= playRoomRules.roomMax {
			return nil, errRefused
		}
		room := pr.newPlayRoom(kind, gameType, host, now)
		return &protobuf.SC_23006{Result: result(nil), Room: room.proto()}, nil
	}()
	pr.mu.Unlock()
	box.flush()
	if err != nil {
		return client.Send(&protobuf.SC_23006{Result: result(err)})
	}
	return client.Send(reply)
}

// handlePlayRoomJoin answers PlayRoomJoinRoomCommand (CS_23007): into the
// room's first free seat, or among its spectators (mode_room.viewer_count),
// result 6 when both are full. Room 0 is the quick join: the fullest COMMON
// room with a free seat, result 20 without one. A PERSON room takes the
// commanders its players invited, a room in game or a MATCH room nobody.
func (game *Game) handlePlayRoomJoin(client *connection.Client, req *protobuf.CS_23007) error {
	ctx := context.Background()
	now := time.Now()
	player, err := game.ptgcPlayer(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	id := client.CommanderID
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	reply, err := func() (*protobuf.SC_23008, error) {
		var room *playRoom
		if req.GetRoomId() == 0 {
			for _, candidate := range sortedRooms(pr.rooms) {
				if candidate.kind == playRoomCommon && candidate.phase == playWait && !candidate.full() && candidate != pr.of[id] &&
					(room == nil || len(candidate.players()) > len(room.players())) {
					room = candidate
				}
			}
			if room == nil {
				return nil, errPlayRoomNoneFound
			}
		} else {
			room = pr.rooms[req.GetRoomId()]
			if room == nil || room.kind == playRoomMatch || room.phase != playWait || (room.kind == playRoomPerson && !room.invited[id]) {
				return nil, errRefused
			}
		}
		if pr.of[id] == room {
			return &protobuf.SC_23008{Result: result(nil), Room: room.proto()}, nil
		}
		if room.full() && uint32(room.viewers()) >= modeRooms[room.gameType].ViewerCount {
			return nil, errPlayRoomFull
		}
		if own := pr.of[id]; own != nil {
			if own.phase != playWait && own.phase != playMatching {
				return nil, errRefused
			}
			game.leavePlayRoom(box, id, now)
		}
		room.add(player, !room.full())
		pr.of[id] = room
		room.tellRoom(box, id)
		return &protobuf.SC_23008{Result: result(nil), Room: room.proto()}, nil
	}()
	pr.mu.Unlock()
	box.flush()
	if err != nil {
		return client.Send(&protobuf.SC_23008{Result: result(err)})
	}
	return client.Send(reply)
}

// handlePlayRoomSwitchViewer answers PlayRoomSwitchViewerCommand
// (CS_23009): team_id 0 takes a seated player (not the host) to the
// spectators, a team index (1-based, PlayRoomTools.GetUnfullTeamIndex)
// seats a spectator in that team.
func (game *Game) handlePlayRoomSwitchViewer(client *connection.Client, req *protobuf.CS_23009) error {
	id := client.CommanderID
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		room := pr.of[id]
		if room == nil || room.phase != playWait || room.kind == playRoomMatch {
			return errRefused
		}
		team := int(req.GetTeamId())
		if team == 0 {
			if !room.seated(id) || room.id == id || uint32(room.viewers()) >= modeRooms[room.gameType].ViewerCount {
				return errRefused
			}
			room.unseat(id)
		} else if room.seated(id) || team > len(room.teams) || !room.seat(id, team-1) {
			return errRefused
		}
		room.tellRoom(box, 0)
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23010{Result: result(err)})
}

// handlePlayRoomExit answers PlayRoomExitRoomCommand (CS_23011, also the
// stop of a matchmaking): out of the commander's room (leavePlayRoom), time
// the matchmaking cooldown's end (PlayRoomProxy:SetMatchCD). The auction's
// matchmaking goes first.
func (game *Game) handlePlayRoomExit(client *connection.Client, req *protobuf.CS_23011) error {
	if handled, cd := game.auctionExitMatch(client); handled {
		return client.Send(&protobuf.SC_23012{Time: proto.Uint32(cd)})
	}
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	cd := game.leavePlayRoom(box, client.CommanderID, time.Now())
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23012{Time: proto.Uint32(cd)})
}

// handlePlayRoomKick answers PlayRoomKickCommand (CS_23013): the host
// sends a player of its lobby room away (SC_23096 reason 1,
// match_ui_room_out).
func (game *Game) handlePlayRoomKick(client *connection.Client, req *protobuf.CS_23013) error {
	id, target := client.CommanderID, req.GetUserId()
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		room := pr.of[id]
		if room == nil || room.id != id || room.phase != playWait || target == id || pr.of[target] != room {
			return errRefused
		}
		delete(pr.of, target)
		delete(room.invited, target)
		room.remove(target)
		box.push(target, &protobuf.SC_23096{Reason: proto.Uint32(playRoomOutKicked)})
		room.tellRoom(box, 0)
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23014{Result: result(err)})
}

// handlePlayRoomReady answers PlayRoomReadyCommand (CS_23015): a seated
// player other than the host gets ready (arg 1) or not (0).
func (game *Game) handlePlayRoomReady(client *connection.Client, req *protobuf.CS_23015) error {
	id := client.CommanderID
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		room := pr.of[id]
		if room == nil || room.phase != playWait || room.kind == playRoomMatch || room.id == id || !room.seated(id) || req.GetArg() > 1 {
			return errRefused
		}
		room.ready[id] = req.GetArg() == 1
		room.tellRoom(box, 0)
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23016{Result: result(err)})
}

// handlePlayRoomStart answers PlayRoomStartGameCommand (CS_23017), from the
// host: a MATCH room is queued for a table (matchTables), a lobby room
// whose seated players are all ready starts loading its game, bots in the
// empty seats (island_bar_quick_addbot: playing alone is a game against
// three bots).
func (game *Game) handlePlayRoomStart(client *connection.Client, req *protobuf.CS_23017) error {
	ctx := context.Background()
	now := time.Now()
	id := client.CommanderID
	var gameType uint32
	pr := game.lockPlayRooms(client)
	if room := pr.of[id]; room != nil {
		gameType = room.gameType
	}
	pr.mu.Unlock()
	score, err := game.addPlayRoomScore(ctx, id, gameType, 0, false)
	if err != nil {
		return err
	}
	box := game.playOutbox(client)
	pr = game.lockPlayRooms(client)
	err = func() error {
		room := pr.of[id]
		if room == nil || room.id != id || !cheatBarGame(room.gameType) {
			return errRefused
		}
		switch {
		case room.kind == playRoomMatch && room.phase == playMatching:
			return nil
		case room.kind == playRoomMatch && room.phase == playWait && len(room.humans()) == 1:
			room.phase = playMatching
			room.matchSince = now
			room.score = score
			room.botAfter = botWait(score)
			game.matchTables(box, now)
			return nil
		case room.kind != playRoomMatch && room.phase == playWait:
			for _, seated := range room.players() {
				if seated != id && !room.ready[seated] {
					return errRefused
				}
			}
			host := room.members[id]
			for !room.full() {
				room.add(pr.newPlayBot(host.GetLevel()), true)
			}
			game.startLoading(box, room, now)
			return nil
		}
		return errRefused
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23018{Result: result(err)})
}

// botWait is how long a commander of score waits before bots fill its
// table: barbot_match_waittime of its island_integral_rank bracket, at
// random between its two times.
func botWait(score uint32) time.Duration {
	bracket, _ := integralRank(score)
	row := playRoomRules.botWait[len(playRoomRules.botWait)-1]
	for _, candidate := range playRoomRules.botWait {
		if candidate[0] == bracket {
			row = candidate
		}
	}
	seconds := row[1] + uint32(rand.IntN(int(row[2]-row[1])+1))
	return time.Duration(seconds) * time.Second
}

// handlePlayRoomMatchReady answers PlayRoomMatchReadyCommand (CS_23019): a
// commander at a table accepts it (ready 1: everyone sees it in SC_23095,
// and the load starts once all did) or refuses it (0: cooldown, the others
// back to matchmaking). A refusal of a table already gone is fine.
func (game *Game) handlePlayRoomMatchReady(client *connection.Client, req *protobuf.CS_23019) error {
	id := client.CommanderID
	now := time.Now()
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		room := pr.of[id]
		if room == nil || room.phase != playReady {
			if req.GetReady() == 0 {
				return nil
			}
			return errRefused
		}
		if req.GetReady() == 0 {
			pr.punish(id, now)
			game.breakTable(box, room, map[uint32]bool{id: true}, 0)
			return nil
		}
		room.accepted[id] = true
		for _, seated := range room.players() {
			if !room.accepted[seated] {
				room.tellTable(box)
				return nil
			}
		}
		room.tellTable(box)
		game.startLoading(box, room, now)
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23020{Result: result(err)})
}

// handlePlayRoomInvite answers PlayRoomInviteCommand (CS_23021): a friend
// or guildmate online (PlayRoomInfoInviteScene) gets the invitation
// (SC_23097) to the commander's lobby room, and may then join it even
// PERSON. The same commander is invited once per match_refuseCD (the
// client's own limit).
func (game *Game) handlePlayRoomInvite(client *connection.Client, req *protobuf.CS_23021) error {
	id, target := client.CommanderID, req.GetUserId()
	now := time.Now()
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		room := pr.of[id]
		key := [2]uint32{id, target}
		if room == nil || room.phase != playWait || room.kind == playRoomMatch || target == id || isPlayBot(target) ||
			pr.of[target] == room || !game.isOnline(target) {
			return errRefused
		}
		if at, ok := pr.invites[key]; ok && now.Sub(at) < playRoomRules.refuseTime {
			return errRefused
		}
		pr.invites[key] = now
		room.invited[target] = true
		box.push(target, &protobuf.SC_23097{Invitor: room.members[id], Room: room.simple()})
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23022{Result: result(err)})
}

// handlePlayRoomChat answers PlayRoomSendChatCommand's room channel
// (CS_23023, already filtered by the client's wordVer): to everyone in the
// room, the sender included (PlayRoomProxy only shows what SC_50116
// brings), shaped like world chat (the sender's PLAYER_INFO_P50).
func (game *Game) handlePlayRoomChat(client *connection.Client, req *protobuf.CS_23023) error {
	ctx := context.Background()
	if req.GetContent() == "" {
		return client.Send(&protobuf.SC_23024{Result: result(errRefused), Tip: proto.String("")})
	}
	sender, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	msg := &protobuf.SC_50116{
		Player:  game.playerInfoP50(sender),
		Type:    proto.Uint32(req.GetType()),
		Content: proto.String(req.GetContent()),
	}
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	room := pr.of[client.CommanderID]
	if room != nil {
		for _, id := range room.humans() {
			if pr.of[id] == room {
				box.push(id, msg)
			}
		}
	}
	pr.mu.Unlock()
	box.flush()
	if room == nil {
		return client.Send(&protobuf.SC_23024{Result: result(errRefused), Tip: proto.String("")})
	}
	return client.Send(&protobuf.SC_23024{Result: result(nil), Tip: proto.String("")})
}

// handlePlayRoomRank answers PlayRoomRefreshRankCommand (CS_23025): the
// best season scores of the game type (PlayRoomRankScene splits them into
// friends, guild and all itself).
func (game *Game) handlePlayRoomRank(client *connection.Client, req *protobuf.CS_23025) error {
	ctx := context.Background()
	season, _ := playRoomSeason(req.GetGameType(), time.Now())
	rows, err := game.DB.ListPlayRoomRanking(ctx, db.ListPlayRoomRankingParams{GameType: req.GetGameType(), Season: season.ID, Limit: playRankSize})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_23026{}
	for _, row := range rows {
		player, err := game.ptgcPlayer(ctx, row.CommanderID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		reply.RankList = append(reply.RankList, &protobuf.PTGC_PLAYER_RANK{Player: player, Score: proto.Uint32(row.Score)})
	}
	return client.Send(reply)
}

// handlePlayRoomLoad answers PlayRoomSendSceneLoadCompleteCommand
// (CS_23027): the commander's load, pushed to the room (SC_23094); the game
// starts once everyone loaded. Back in a game it left (reloaded from a
// PLAYING room), the load is over at once. The auction's load goes first.
func (game *Game) handlePlayRoomLoad(client *connection.Client, req *protobuf.CS_23027) error {
	if game.auctionSceneLoaded(client) {
		return client.Send(&protobuf.SC_23028{Result: result(nil)})
	}
	id := client.CommanderID
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		room := pr.of[id]
		switch {
		case room == nil:
			return errRefused
		case room.phase == playLoading && room.seated(id):
			room.loads[id] = max(room.loads[id], min(req.GetProgress(), 100))
			game.loadProgress(box, room, time.Now())
		case room.phase == playPlaying && room.seated(id):
			box.push(id, room.loadList(true))
		default:
			return errRefused
		}
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23028{Result: result(err)})
}

// handlePlayRoomView answers IslandChangeViewDressCommand (CS_23029): the
// island character the commander sits as in a game type (type 1 from a
// room, 2 from the entrance), kept and shown to its room (SC_23099; the
// client refreshes its own with CS_23003). Any island_chara_template
// character is taken: belfast has no island characters to check it against
// yet.
func (game *Game) handlePlayRoomView(client *connection.Client, req *protobuf.CS_23029) error {
	ctx := context.Background()
	gameType, ship := req.GetGameType(), req.GetShipId()
	if _, ok := modeRooms[gameType]; !ok || gameType == 0 {
		return client.Send(&protobuf.SC_23030{Result: result(errRefused)})
	}
	if _, ok := islandCharas[ship]; !ok {
		return client.Send(&protobuf.SC_23030{Result: result(errRefused)})
	}
	err := game.DB.SetPlayRoomView(ctx, db.SetPlayRoomViewParams{CommanderID: client.CommanderID, GameType: gameType, ShipID: ship, DressList: db.IDList{}})
	if err != nil {
		return err
	}
	player, err := game.ptgcPlayer(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	if room := pr.of[client.CommanderID]; room != nil && (room.phase == playWait || room.phase == playMatching) {
		room.members[client.CommanderID] = player
		room.tellRoom(box, client.CommanderID)
	}
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23030{Result: result(nil)})
}
