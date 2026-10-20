package game

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Auction matches, run in memory (AuctionGameProxy's pushes, the client's
// AuctionGameMainScene):
//
//  1. CS_23418 (PlayRoomQuickMatchCommand, the entrance's match button,
//     type the house's auction_session.game_type) queues the commander.
//     Four commanders queued for the same house make a match at once;
//     otherwise, after a wait drawn from auction_matching_limit, the queued
//     ones (up to four) are matched and bots fill the seats.
//  2. SC_23400 (match success): the players, sort giving the seat order,
//     and the lot's grid lines. The client takes the house's ticket from its
//     coins (AuctionGameEntranceScene:OnQuickMatchSuccess), so does the
//     server, and opens the auction scene, whose AuctionGameMainReadyMediator
//     reports it loaded (CS_23027, auctionSceneLoaded).
//  3. Once every commander loaded, or auction_match_loading_timeout after
//     SC_23400, each round runs: SC_23401 (the round, its public event's
//     effect, the player's three personal events to pick from, timestamp
//     the end of auction_event_choose_time), CS_23402 picks one (SC_23403
//     its effect, SC_23412 opt_type 1 to everyone); SC_23404 once all
//     picked or at timestamp (the events picked, the effect of the one
//     picked for a commander who didn't, timestamp the end of
//     auction_bid_time); CS_23405 bids (SC_23412 opt_type 2); SC_23407 once
//     all bid or at timestamp (every bid, state 1 the winner's, timestamp
//     the end of auction_publicity_time). After the publicity, SC_23408
//     settles a won lot, or the next round starts; when nobody bid the
//     auction ends without a winner (AuctionGameMainScene:OnNoBid).
//  4. CS_23409 forfeits (SC_23412 opt_type 3: no more bids, the commander
//     watches); SC_23411 lists the commanders removed from the match.
//
// Bots pick an event after auction_bot_time_event seconds and bid after
// auction_bot_time_bid seconds, a bid drawn from the house's
// auction_bot_bid_<tier> range of the round.
//
// The winner (the tutorial, gamecfg/guide/newguide/segments/auction_guide_6:
// "In round 1 to round 4, if the highest bid exceeds the second by the
// required multiplier, the items will be sold to the highest bidder. In
// round 5, the highest bidder will immediately win"): the highest bid wins
// the lot when it is more than one_hit times the second highest (or the
// only bid, GUESS), ties going to who bid first (GUESS). The winner pays
// its bid and gets the lot's value; when that is a loss, every other
// commander still in the match gets auction_divided percent of the
// difference (the tutorial: "If it is negative, the other participants
// will receive Bid Coins equal to 15% of the difference").

type auctionPhase int

const (
	auctionLoading auctionPhase = iota
	auctionEvents
	auctionBids
	auctionPublicity
	auctionOver
)

// SC_23412 opt_type (AuctionGameProxy:UpdatePlayerOptState)
const (
	auctionOptEvent   = 1
	auctionOptBid     = 2
	auctionOptForfeit = 3
)

// SC_23429 state: AuctionGameEntranceMediator stops matching on 3. GUESS: 1
// still queued, 2 in a match.
const (
	auctionQueued   = 1
	auctionPlaying  = 2
	auctionStopped  = 3
	auctionKickCode = 12 // AuctionGameBidCommand / EventSelectedID: kicked
)

// auctionBotBase is where bot user ids start, above any commander id
// (GUESS: how official bots are numbered is unknown).
const auctionBotBase = 4_000_000_000

// auctionInactivityBan is how long matchmaking is closed after the second
// removal for inactivity in a day, and each one after (GUESS).
const auctionInactivityBan = 30 * time.Minute

var errAuctionKicked error = refusal{"removed from the auction", auctionKickCode}

// auctionHall is the matchmaking queues and the matches running; one lock
// guards all of it, timers included.
type auctionHall struct {
	mu      sync.Mutex
	queues  map[uint32][]*auctionSeat // by game_type, in arrival order
	seats   map[uint32]*auctionSeat   // by commander: queued or in a match
	hooked  map[*connection.Client]bool
	nextBot uint32
}

// auctionSeat is a commander queued or playing.
type auctionSeat struct {
	commanderID uint32
	client      *connection.Client
	gameType    uint32
	activityID  uint32
	info        *protobuf.PTGC_PLAYER
	timer       *time.Timer
	match       *auctionMatch
	player      *auctionPlayer
}

type auctionPlayer struct {
	id   uint32
	info *protobuf.PTGC_PLAYER
	seat *auctionSeat // nil for a bot
	gold uint32       // a commander's coins after the ticket

	loaded, forfeited, left, kicked bool
	hideEmoji                       bool
	emojiAt                         time.Time

	// this round
	events    []uint32
	event     uint32
	autoEvent bool
	bid       uint32
	bidden    bool
	order     uint32
	acted     bool
}

func (player *auctionPlayer) bot() bool { return player.seat == nil }

// active bidders take part in the round.
func (player *auctionPlayer) active() bool {
	return !player.left && !player.kicked && !player.forfeited
}

// watching commanders get the match's pushes.
func (player *auctionPlayer) watching() bool { return !player.bot() && !player.left && !player.kicked }

type auctionMatch struct {
	game       *Game
	session    auctionSessionConfig
	activityID uint32
	players    []*auctionPlayer
	lot        auctionLot
	round      uint32
	phase      auctionPhase
	stamp      uint64 // changes with the phase: stale timers do nothing
	bids       uint32
}

// auctionOutbox is what to send once the hall is unlocked: functions run
// on the commanders' dispatchers (connection.Client.Run), in order.
type auctionOutbox []auctionDelivery

type auctionDelivery struct {
	client *connection.Client
	fn     func(*connection.Client) error
}

func (out *auctionOutbox) run(client *connection.Client, fn func(*connection.Client) error) {
	*out = append(*out, auctionDelivery{client, fn})
}

// push sends copies of msgs (FillRequired fills them in place).
func (out *auctionOutbox) push(client *connection.Client, msgs ...proto.Message) {
	copies := make([]proto.Message, len(msgs))
	for i, msg := range msgs {
		copies[i] = proto.Clone(msg)
	}
	out.run(client, func(client *connection.Client) error {
		for _, msg := range copies {
			if err := client.Push(msg); err != nil {
				return err
			}
		}
		return nil
	})
}

func (out auctionOutbox) flush() {
	for _, delivery := range out {
		if err := delivery.client.Run(delivery.fn); err != nil {
			logger.WithFields("Auction", logger.FieldValue("commander", delivery.client.CommanderID), logger.FieldValue("error", err)).Warn("auction push not delivered")
		}
	}
}

// withAuction runs fn on the hall locked, then sends what it queued.
func (game *Game) withAuction(fn func(hall *auctionHall, out *auctionOutbox)) {
	var out auctionOutbox
	hall := &game.auctions
	hall.mu.Lock()
	if hall.seats == nil {
		hall.seats, hall.queues, hall.hooked = map[uint32]*auctionSeat{}, map[uint32][]*auctionSeat{}, map[*connection.Client]bool{}
	}
	fn(hall, &out)
	hall.mu.Unlock()
	out.flush()
}

// after runs fn in d unless the match moved to another phase meanwhile.
func (match *auctionMatch) after(d time.Duration, fn func(out *auctionOutbox)) {
	stamp := match.stamp
	time.AfterFunc(d, func() {
		match.game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
			if match.stamp == stamp && match.phase != auctionOver {
				fn(out)
			}
		})
	})
}

func seconds(n uint32) time.Duration { return time.Duration(n) * time.Second }

// broadcast pushes msg to the commanders watching.
func (match *auctionMatch) broadcast(out *auctionOutbox, msg proto.Message) {
	for _, player := range match.players {
		if player.watching() {
			out.push(player.seat.client, msg)
		}
	}
}

func (match *auctionMatch) opt(out *auctionOutbox, kind uint32, player *auctionPlayer) {
	match.broadcast(out, &protobuf.SC_23412{Round: proto.Uint32(match.round), OptType: proto.Uint32(kind), UserId: proto.Uint32(player.id)})
}

func (match *auctionMatch) watched() bool {
	return slices.ContainsFunc(match.players, (*auctionPlayer).watching)
}

// dequeue takes a queued seat out of its queue.
func (hall *auctionHall) dequeue(seat *auctionSeat) {
	if seat.timer != nil {
		seat.timer.Stop()
		seat.timer = nil
	}
	queue := hall.queues[seat.gameType]
	hall.queues[seat.gameType] = slices.DeleteFunc(queue, func(other *auctionSeat) bool { return other == seat })
}

// handleAuctionQuickMatch answers PlayRoomQuickMatchCommand (CS_23418, the
// entrance's match button once its animation ends): queues the commander
// for the house of game_type type. Refused when no auction runs, under
// the house's threshold (or its ticket), while matchmaking is closed to the
// commander (is_forbidden, back_forbidden, match_time) and while in a
// match.
func (game *Game) handleAuctionQuickMatch(client *connection.Client, req *protobuf.CS_23418) error {
	ctx := context.Background()
	now := time.Now()
	err := game.queueAuction(ctx, client, req.GetType(), now)
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_23419{Result: result(err)})
}

func (game *Game) queueAuction(ctx context.Context, client *connection.Client, gameType uint32, now time.Time) error {
	sessionID, ok := auction.byGameType[gameType]
	if !ok {
		return errRefused
	}
	session := auction.sessions[sessionID]
	state, _, err := runningAuction(ctx, game.DB.Queries, client.CommanderID, now)
	if err != nil {
		return err
	}
	if state.Gold < max(session.Threshold, session.Ticket) || state.IsForbidden != 0 || state.BackForbidden != 0 || now.Unix() < state.MatchTime {
		return errRefused
	}
	info, err := game.ptgcPlayer(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	id := client.CommanderID
	hook := false
	game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
		if seat := hall.seats[id]; seat != nil {
			if seat.match != nil && seat.match.phase != auctionOver && !seat.player.kicked {
				err = errRefused
				return
			}
			if seat.match == nil {
				hall.dequeue(seat)
			}
			delete(hall.seats, id)
		}
		seat := &auctionSeat{commanderID: id, client: client, gameType: gameType, activityID: state.ActivityID, info: info}
		hall.seats[id] = seat
		hall.queues[gameType] = append(hall.queues[gameType], seat)
		if !hall.hooked[client] {
			hall.hooked[client], hook = true, true
		}
		if len(hall.queues[gameType]) >= auctionPlayers {
			game.startAuction(hall, hall.queues[gameType][:auctionPlayers], out)
			return
		}
		seat.timer = time.AfterFunc(seconds(randomBetween(auction.matchWait)), func() {
			game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
				if hall.seats[id] != seat || seat.match != nil {
					return
				}
				queue := hall.queues[gameType]
				game.startAuction(hall, queue[:min(len(queue), auctionPlayers)], out)
			})
		})
	})
	if hook {
		client.OnClose(func() { game.auctionDisconnect(client) })
	}
	return err
}

// startAuction matches the queued seats, bots taking the seats left.
func (game *Game) startAuction(hall *auctionHall, seats []*auctionSeat, out *auctionOutbox) {
	seats = slices.Clone(seats)
	if len(seats) == 0 {
		return
	}
	for _, seat := range seats {
		hall.dequeue(seat)
	}
	session := auction.sessions[auction.byGameType[seats[0].gameType]]
	match := &auctionMatch{game: game, session: session, activityID: seats[0].activityID, lot: newAuctionLot(session)}
	for _, seat := range seats {
		player := &auctionPlayer{id: seat.commanderID, info: seat.info, seat: seat}
		seat.match, seat.player = match, player
		match.players = append(match.players, player)
	}
	ships := slices.Clone(auction.botShips)
	rand.Shuffle(len(ships), func(i, j int) { ships[i], ships[j] = ships[j], ships[i] })
	for n := 0; len(match.players) < auctionPlayers; n++ {
		hall.nextBot++
		id := auctionBotBase + hall.nextBot
		match.players = append(match.players, &auctionPlayer{id: id, info: auctionBotPlayer(id, ships[n%len(ships)])})
	}
	rand.Shuffle(len(match.players), func(i, j int) { match.players[i], match.players[j] = match.players[j], match.players[i] })
	success := &protobuf.SC_23400{Line: proto.Uint32(match.lot.Lines)}
	for n, player := range match.players {
		success.PlayerList = append(success.PlayerList, &protobuf.PTGC_BID_KING_PLAYER{
			UserId:     proto.Uint32(player.id),
			Sort:       proto.Uint32(uint32(n + 1)),
			PlayerInfo: player.info,
		})
	}
	for _, player := range match.players {
		if player.bot() {
			continue
		}
		msg := proto.Clone(success)
		out.run(player.seat.client, func(client *connection.Client) error {
			gold, err := game.payAuctionTicket(client, match)
			if err != nil {
				return err
			}
			game.withAuction(func(*auctionHall, *auctionOutbox) { player.gold = gold })
			return client.Push(msg)
		})
	}
	match.phase = auctionLoading
	match.after(seconds(auctionSetting("match_loading_timeout")), match.startRound)
}

// auctionBotPlayer is a bot's PTGC_PLAYER: one of the auction_bot_ship
// ships as icon, an auction_bot_frame frame, named "Participant<n>" like
// the opponents of the rules' screenshots (gametip auction_help, helpbg
// auction_help_2..6: Participant514, Participant643...).
//
// GUESS: the number is 3 random digits, the level random.
func auctionBotPlayer(id uint32, ship auctionBotShip) *protobuf.PTGC_PLAYER {
	return &protobuf.PTGC_PLAYER{
		Id:        proto.Uint32(id),
		Level:     proto.Uint32(randomBetween([2]uint32{30, 125})),
		Name:      proto.String(fmt.Sprintf("Participant%d", 100+rand.IntN(900))),
		GuildName: proto.String(""),
		Display: &protobuf.DISPLAYINFO{
			Icon:      proto.Uint32(ship.id),
			Skin:      proto.Uint32(ship.skin),
			IconFrame: proto.Uint32(auction.botFrames[rand.IntN(len(auction.botFrames))]),
		},
	}
}

// payAuctionTicket takes the house's ticket from the commander's coins at
// match success, counting the auction attended; it returns the coins left.
func (game *Game) payAuctionTicket(client *connection.Client, match *auctionMatch) (uint32, error) {
	ctx := context.Background()
	var gold uint32
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := auctionState(ctx, tx.Queries, client.CommanderID, match.activityID, time.Now())
		if err != nil {
			return err
		}
		paid := min(match.session.Ticket, state.Gold)
		state.Gold -= paid
		state.GameNum++
		gold = state.Gold
		tx.event(
			TaskEvent{SubType: TaskSubAuctionJoin, Count: 1},
			TaskEvent{SubType: TaskSubAuctionSession, Target: match.session.GameType, Count: 1},
			TaskEvent{SubType: TaskSubAuctionSpent, Count: paid},
			TaskEvent{SubType: TaskSubAuctionGold, Count: state.Gold},
		)
		return saveAuctionState(ctx, tx.Queries, state)
	})
	return gold, err
}

// checkLoaded starts the first round once every commander loaded.
func (match *auctionMatch) checkLoaded(out *auctionOutbox) {
	if match.phase != auctionLoading {
		return
	}
	for _, player := range match.players {
		if player.watching() && !player.loaded {
			return
		}
	}
	match.startRound(out)
}

// startRound starts the next round's event phase (SC_23401).
//
// One group (public) event and three individual events per auction (wiki,
// Recent News, Top Bidder, 16 July 2026). GUESS: the public event is any
// group 2 auction_event, the personal ones three distinct group 1 events,
// drawn uniformly (the wiki doesn't say how they are picked).
func (match *auctionMatch) startRound(out *auctionOutbox) {
	match.round++
	match.phase = auctionEvents
	match.stamp++
	now := time.Now()
	deadline := now.Add(seconds(auctionSetting("event_choose_time")))
	public := match.lot.effect(auction.public[rand.IntN(len(auction.public))])
	for _, player := range match.players {
		player.events, player.event, player.autoEvent = nil, 0, false
		player.bid, player.bidden, player.order, player.acted = 0, false, 0, false
		if player.active() {
			events := slices.Clone(auction.personal)
			rand.Shuffle(len(events), func(i, j int) { events[i], events[j] = events[j], events[i] })
			player.events = events[:auctionPersonalEvents]
		}
	}
	for _, player := range match.players {
		if player.watching() {
			round := &protobuf.SC_23401{
				Round:             proto.Uint32(match.round),
				PublicEventEffect: public,
				EventList:         player.events,
				Timestamp:         proto.Uint32(uint32(deadline.Unix())),
			}
			if match.round == 1 {
				round.StartTime = proto.Uint32(uint32(now.Unix()))
			}
			out.push(player.seat.client, round)
		}
		if player.bot() && player.active() {
			match.after(seconds(randomBetween(auction.botEvent)), func(out *auctionOutbox) {
				if player.event == 0 && player.active() {
					player.event = player.events[rand.IntN(len(player.events))]
					match.opt(out, auctionOptEvent, player)
					match.checkEvents(out)
				}
			})
		}
	}
	match.after(deadline.Sub(now), match.endEvents)
}

func (match *auctionMatch) checkEvents(out *auctionOutbox) {
	if match.phase != auctionEvents {
		return
	}
	for _, player := range match.players {
		if player.active() && player.event == 0 {
			return
		}
	}
	match.endEvents(out)
}

// endEvents starts the bid phase (SC_23404): the bidders who didn't pick
// an event get one of theirs at random.
func (match *auctionMatch) endEvents(out *auctionOutbox) {
	match.phase = auctionBids
	match.stamp++
	now := time.Now()
	deadline := now.Add(seconds(auctionSetting("bid_time")))
	var picked []*protobuf.PTGC_BID_KING_PLAYER_EVENT
	for _, player := range match.players {
		if player.active() && player.event == 0 && len(player.events) > 0 {
			player.event, player.autoEvent = player.events[rand.IntN(len(player.events))], true
		}
		// event 0 would break AuctionGameMainRoundOverPlayer (no row)
		if player.event != 0 {
			picked = append(picked, &protobuf.PTGC_BID_KING_PLAYER_EVENT{UserId: proto.Uint32(player.id), EventId: proto.Uint32(player.event)})
		}
	}
	for _, player := range match.players {
		if player.watching() {
			bids := &protobuf.SC_23404{Timestamp: proto.Uint32(uint32(deadline.Unix())), PlayerEvents: picked}
			if player.autoEvent {
				bids.EventEffect = match.lot.effect(player.event)
			}
			out.push(player.seat.client, bids)
		}
		if player.bot() && player.active() {
			match.after(seconds(randomBetween(auction.botBid)), func(out *auctionOutbox) {
				if !player.bidden && player.active() {
					match.placeBid(player, randomBetween(match.session.botBids[match.round-1]))
					match.opt(out, auctionOptBid, player)
					match.checkBids(out)
				}
			})
		}
	}
	match.after(deadline.Sub(now), match.endBids)
}

func (match *auctionMatch) placeBid(player *auctionPlayer, price uint32) {
	match.bids++
	player.bid, player.bidden, player.order, player.acted = price, true, match.bids, true
}

func (match *auctionMatch) checkBids(out *auctionOutbox) {
	if match.phase != auctionBids {
		return
	}
	for _, player := range match.players {
		if player.active() && !player.bidden {
			return
		}
	}
	match.endBids(out)
}

// endBids ends the round (SC_23407): commanders who neither picked an event
// nor bid are removed (SC_23411, GUESS: "The auction has ended due to
// extended inactivity"), the winner if any is told, and after the
// publicity the lot is settled, the next round starts or, nobody having
// bid, the auction ends.
func (match *auctionMatch) endBids(out *auctionOutbox) {
	match.phase = auctionPublicity
	match.stamp++
	now := time.Now()
	var kicked []uint32
	for _, player := range match.players {
		if !player.bot() && player.active() && !player.acted {
			player.kicked, player.bid = true, 0
			kicked = append(kicked, player.id)
			client := player.seat.client
			out.run(client, func(client *connection.Client) error {
				return match.game.recordAuctionInactivity(client, match.activityID)
			})
		}
	}
	if len(kicked) > 0 {
		removed := &protobuf.SC_23411{UserIds: kicked}
		for _, player := range match.players {
			if !player.bot() && !player.left {
				out.push(player.seat.client, removed)
			}
		}
	}
	var bidders []*auctionPlayer
	for _, player := range match.players {
		if player.bid > 0 && !player.left && !player.kicked {
			bidders = append(bidders, player)
		}
	}
	slices.SortFunc(bidders, func(a, b *auctionPlayer) int {
		return cmp.Or(cmp.Compare(b.bid, a.bid), cmp.Compare(a.order, b.order))
	})
	var winner *auctionPlayer
	if len(bidders) > 0 {
		last := int(match.round) >= len(auction.oneHit)
		if last || len(bidders) == 1 || float64(bidders[0].bid) > float64(bidders[1].bid)*auction.oneHit[match.round-1] {
			winner = bidders[0]
		}
	}
	results := &protobuf.SC_23407{Timestamp: proto.Uint32(uint32(now.Add(seconds(auctionSetting("publicity_time"))).Unix()))}
	for _, player := range match.players {
		state := uint32(0)
		if player == winner {
			state = 1
		}
		results.BidList = append(results.BidList, &protobuf.PTGC_BID_KING_PLAYER_BID{UserId: proto.Uint32(player.id), Price: proto.Uint32(player.bid), State: proto.Uint32(state)})
	}
	match.broadcast(out, results)
	if !match.watched() {
		match.end()
		return
	}
	match.after(seconds(auctionSetting("publicity_time")), func(out *auctionOutbox) {
		switch {
		case winner != nil:
			match.settle(winner, out)
		case len(bidders) == 0:
			match.end()
		default:
			match.startRound(out)
		}
	})
}

// settle hands the lot to winner (SC_23408): change_gold is what each
// commander's coins change by, the lot's value minus the bid for the
// winner, the dividend for the others when the winner lost coins.
func (match *auctionMatch) settle(winner *auctionPlayer, out *auctionOutbox) {
	value := match.lot.value()
	dividend := int64(0)
	if loss := int64(winner.bid) - int64(value); loss > 0 {
		dividend = loss * int64(auctionSetting("divided")) / 100
	}
	var items []*protobuf.PTGC_BID_KING_ITEM
	for _, item := range match.lot.Items {
		items = append(items, item.full())
	}
	for _, player := range match.players {
		if !player.watching() {
			continue
		}
		won := player == winner
		change := dividend
		if won {
			change = int64(value) - int64(winner.bid)
		}
		settlement := proto.Clone(&protobuf.SC_23408{
			BidUserId:  proto.Uint32(winner.id),
			BidPrice:   proto.Uint32(winner.bid),
			ItemList:   items,
			ChangeGold: proto.Int32(int32(max(min(change, 1<<31-1), -1<<31))),
		})
		bid := winner.bid
		out.run(player.seat.client, func(client *connection.Client) error {
			if err := match.game.recordAuctionSettlement(client, match, won, bid, change); err != nil {
				return err
			}
			return client.Push(settlement)
		})
	}
	match.end()
}

// end closes the match: its commanders can match again.
func (match *auctionMatch) end() {
	match.phase = auctionOver
	match.stamp++
	hall := &match.game.auctions
	for _, player := range match.players {
		if player.seat != nil && hall.seats[player.id] == player.seat {
			delete(hall.seats, player.id)
		}
	}
}

// recordAuctionSettlement stores a settled auction: the coins changed by
// change, the earnings, and for the winner the auction won, its bid, the
// lot's value and items (the item logbook).
func (game *Game) recordAuctionSettlement(client *connection.Client, match *auctionMatch, won bool, bid uint32, change int64) error {
	ctx := context.Background()
	return game.tx(ctx, client, func(tx *Tx) error {
		state, err := auctionState(ctx, tx.Queries, client.CommanderID, match.activityID, time.Now())
		if err != nil {
			return err
		}
		state.Gold = auctionAddGold(state.Gold, change)
		state.AccProfit += change
		if change > 0 {
			state.MaxProfit = max(state.MaxProfit, uint32(min(change, 1<<32-1)))
		}
		if won {
			value := match.lot.value()
			state.BuyNum++
			state.AccBuyPrice += bid
			state.AccItemPrice += value
			state.MaxLotValue = max(state.MaxLotValue, value)
			state.ItemList = auctionUnlock(state.ItemList, match.lot)
			tx.event(
				TaskEvent{SubType: TaskSubAuctionWin, Count: 1},
				TaskEvent{SubType: TaskSubAuctionLot, Count: state.MaxLotValue},
				TaskEvent{SubType: TaskSubAuctionLotTotal, Count: state.MaxLotValue},
				TaskEvent{SubType: TaskSubAuctionSpent, Count: bid},
			)
		}
		tx.event(TaskEvent{SubType: TaskSubAuctionGold, Count: state.Gold})
		return saveAuctionState(ctx, tx.Queries, state)
	})
}

// recordAuctionInactivity counts a removal for inactivity (inactive_num,
// which the entrance warns about once, then auction_game_punishment);
// from the second one in a day matchmaking closes for a while (GUESS:
// auctionInactivityBan per removal past the first).
func (game *Game) recordAuctionInactivity(client *connection.Client, activityID uint32) error {
	ctx := context.Background()
	now := time.Now()
	return game.tx(ctx, client, func(tx *Tx) error {
		state, err := auctionState(ctx, tx.Queries, client.CommanderID, activityID, now)
		if err != nil {
			return err
		}
		state.InactiveNum++
		if state.InactiveNum >= 2 {
			state.MatchTime = now.Add(time.Duration(state.InactiveNum-1) * auctionInactivityBan).Unix()
		}
		return saveAuctionState(ctx, tx.Queries, state)
	})
}

// leave takes a commander out of its match (it exited, or its connection
// closed): it bids no more and the others see it gone (SC_23411, GUESS).
// GUESS: leaving isn't punished, only inactivity is.
func (match *auctionMatch) leave(player *auctionPlayer, out *auctionOutbox) {
	if match.phase == auctionOver || player.left {
		return
	}
	wasKicked := player.kicked
	player.left, player.bid = true, 0
	if !wasKicked {
		match.broadcast(out, &protobuf.SC_23411{UserIds: []uint32{player.id}})
	}
	if !match.watched() {
		match.end()
		return
	}
	switch match.phase {
	case auctionLoading:
		match.checkLoaded(out)
	case auctionEvents:
		match.checkEvents(out)
	case auctionBids:
		match.checkBids(out)
	}
}

// release takes the commander's seat away: out of the queue, or out of its
// match. It tells whether the commander was queued or playing.
func (hall *auctionHall) release(id uint32, client *connection.Client, out *auctionOutbox) bool {
	seat := hall.seats[id]
	if seat == nil || (client != nil && seat.client != client) {
		return false
	}
	delete(hall.seats, id)
	if seat.match == nil {
		hall.dequeue(seat)
		return true
	}
	seat.match.leave(seat.player, out)
	return true
}

// auctionExitMatch is the auction's part of PlayRoomExitRoomCommand
// (CS_23011, sent when matching is cancelled, when the auction scene is
// closed and whenever the entrance opens): it leaves the queue or the
// match. handled is false when the commander was in neither; cd is the
// SC_23012 time, always 0: the auction's own cooldown is SC_23431's
// match_time.
func (game *Game) auctionExitMatch(client *connection.Client) (handled bool, cd uint32) {
	game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
		handled = hall.release(client.CommanderID, nil, out)
	})
	return handled, 0
}

// auctionSceneLoaded is the auction's part of
// PlayRoomSendSceneLoadCompleteCommand (CS_23027, sent by
// AuctionGameMainReadyMediator once the auction scene loaded): the first
// round starts once every commander of the match loaded. handled is false
// when the commander isn't in a match.
func (game *Game) auctionSceneLoaded(client *connection.Client) (handled bool) {
	game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
		seat := hall.seats[client.CommanderID]
		if seat == nil || seat.match == nil {
			return
		}
		handled = true
		seat.player.loaded = true
		seat.match.checkLoaded(out)
	})
	return handled
}

// auctionDisconnect leaves the queue or the match when the connection
// queued closes (the auction scene says "Connection lost. You have left the
// match." on reconnecting).
func (game *Game) auctionDisconnect(client *connection.Client) {
	game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
		delete(hall.hooked, client)
		hall.release(client.CommanderID, client, out)
	})
}

// inMatch runs fn on the commander's match and player, errRefused when it
// is in none, errAuctionKicked once removed.
func (game *Game) inMatch(client *connection.Client, fn func(match *auctionMatch, player *auctionPlayer, out *auctionOutbox) error) error {
	err := errRefused
	game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
		seat := hall.seats[client.CommanderID]
		if seat == nil || seat.match == nil || seat.match.phase == auctionOver {
			return
		}
		if seat.player.kicked {
			err = errAuctionKicked
			return
		}
		err = fn(seat.match, seat.player, out)
	})
	return err
}

// handleAuctionEventSelected answers AuctionGameEventSelectedIDCommand
// (CS_23402, AuctionGameMainEventLayer's confirm): the personal event
// picked this round, one of SC_23401's event_list, and its effect for the
// commander alone; the others see it picked (SC_23412).
func (game *Game) handleAuctionEventSelected(client *connection.Client, req *protobuf.CS_23402) error {
	var effect *protobuf.PTGC_BID_KING_EVENT_EFFECT
	err := game.inMatch(client, func(match *auctionMatch, player *auctionPlayer, out *auctionOutbox) error {
		if match.phase != auctionEvents || !player.active() || player.event != 0 || !slices.Contains(player.events, req.GetEventId()) {
			return errRefused
		}
		player.event, player.acted = req.GetEventId(), true
		effect = match.lot.effect(player.event)
		match.opt(out, auctionOptEvent, player)
		match.checkEvents(out)
		return nil
	})
	reply := &protobuf.SC_23403{Result: result(err), PublicEventEffect: effect}
	if effect == nil {
		reply.PublicEventEffect = &protobuf.PTGC_BID_KING_EVENT_EFFECT{}
	}
	return client.Send(reply)
}

// handleAuctionBid answers AuctionGameBidCommand (CS_23405,
// AuctionGameMainBidLayer): the commander's bid this round, at most its
// coins and at least the house's bottom_price, once per round; the others
// see it bid (SC_23412) but not how much until SC_23407.
//
// GUESS: a bid may be lower than the commander's previous one.
func (game *Game) handleAuctionBid(client *connection.Client, req *protobuf.CS_23405) error {
	err := game.inMatch(client, func(match *auctionMatch, player *auctionPlayer, out *auctionOutbox) error {
		price := req.GetPrice()
		if match.phase != auctionBids || !player.active() || player.bidden || price > player.gold || price < match.session.BottomPrice {
			return errRefused
		}
		match.placeBid(player, price)
		match.opt(out, auctionOptBid, player)
		match.checkBids(out)
		return nil
	})
	return client.Send(&protobuf.SC_23406{Result: result(err)})
}

// handleAuctionForfeit answers AuctionGameForfeitCommand (CS_23409, the
// forfeit button, confirmed): no more bids this auction (refused once the
// round's bid is in, auction_game_cannot_forfeit); the commander keeps
// watching and can leave (CS_23011).
func (game *Game) handleAuctionForfeit(client *connection.Client, req *protobuf.CS_23409) error {
	inMatch := false
	err := game.inMatch(client, func(match *auctionMatch, player *auctionPlayer, out *auctionOutbox) error {
		inMatch = true
		if (match.phase != auctionEvents && match.phase != auctionBids) || player.forfeited || player.bidden {
			return errRefused
		}
		player.forfeited, player.acted, player.bid = true, true, 0
		match.opt(out, auctionOptForfeit, player)
		switch match.phase {
		case auctionEvents:
			match.checkEvents(out)
		case auctionBids:
			match.checkBids(out)
		}
		return nil
	})
	if !inMatch && errors.Is(err, errRefused) {
		// the tutorial (AuctionGameMainGuideScene, which shares
		// AuctionGameMainMediator) withdraws for real outside any match,
		// then waits for the reply to show its close button (GUESS: the
		// official server accepts it, there is nothing to withdraw from)
		err = nil
	}
	return client.Send(&protobuf.SC_23410{Result: result(err)})
}

// handleAuctionEmoji answers AuctionGameEmojiCommand (CS_23413, the emoji
// layer): an emoji_template emoji of the auction's (auction 1), at most
// every auction_emoji_cd seconds, shown above the commander's portrait to
// the match's commanders (SC_23415).
//
// GUESS: the sender gets it too: the client shows its own emoji only from
// SC_23415 (AuctionGameMainRightPlayerInfo:ShowEmoji).
func (game *Game) handleAuctionEmoji(client *connection.Client, req *protobuf.CS_23413) error {
	err := game.inMatch(client, func(match *auctionMatch, player *auctionPlayer, out *auctionOutbox) error {
		now := time.Now()
		if !auction.emojis[req.GetExpressionId()] || now.Before(player.emojiAt.Add(seconds(auctionSetting("emoji_cd")))) {
			return errRefused
		}
		player.emojiAt = now
		emoji := &protobuf.SC_23415{UserId: proto.Uint32(player.id), ExpressionId: proto.Uint32(req.GetExpressionId())}
		for _, other := range match.players {
			if other.watching() && !other.hideEmoji {
				out.push(other.seat.client, emoji)
			}
		}
		return nil
	})
	return client.Send(&protobuf.SC_23414{Result: result(err)})
}

// handleAuctionSwitchEmoji answers AuctionGameSwitchEmojiCommand
// (CS_23416, "Hide Emojis"): switch 1 hides the others' emojis (the client
// also drops SC_23415 then), 0 shows them again. The switch only lasts the
// match (AuctionGameProxy:InitGameData resets it), so nothing is saved.
//
//belfast:static
func (game *Game) handleAuctionSwitchEmoji(client *connection.Client, req *protobuf.CS_23416) error {
	err := game.inMatch(client, func(match *auctionMatch, player *auctionPlayer, out *auctionOutbox) error {
		player.hideEmoji = req.GetSwitch() == 1
		return nil
	})
	return client.Send(&protobuf.SC_23417{Result: result(err)})
}

// handleAuctionMatchingReconnect answers AuctionGameMatchingReconnectCommand
// (CS_23428, sent by the entrance after a reconnection while it was
// matching): whether the commander is still queued (the queue then follows
// the new connection), in a match, or neither (3: the entrance stops
// matching).
func (game *Game) handleAuctionMatchingReconnect(client *connection.Client, req *protobuf.CS_23428) error {
	state := uint32(auctionStopped)
	hook := false
	game.withAuction(func(hall *auctionHall, out *auctionOutbox) {
		seat := hall.seats[client.CommanderID]
		switch {
		case seat == nil:
		case seat.match == nil:
			state, seat.client = auctionQueued, client
			if !hall.hooked[client] {
				hall.hooked[client], hook = true, true
			}
		case seat.match.phase != auctionOver && !seat.player.kicked && !seat.player.left:
			state = auctionPlaying
		}
	})
	if hook {
		client.OnClose(func() { game.auctionDisconnect(client) })
	}
	return client.Send(&protobuf.SC_23429{State: proto.Uint32(state)})
}
