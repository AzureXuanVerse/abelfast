package game

import (
	"context"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's cheat bar (Cheater Tavern, mode_room 101,
// IslandCheaterTavernMonitor, IslandCheaterTavernAgency): a liar's card
// game at a table of four, played from a play room (playroom.go). Built
// from the Lua and the gameset alone, no official capture has it.
//
// Each round deals barHand cards from the bar_card deck (gameset bar_card:
// 6 of each of the three cards, 2 card_99 jokers) to every player still in
// and draws the round's real card among the three (SC_23101 for the first,
// SC_23102 after). In turn, a player puts 1 to 3 cards face down claiming
// they are all the real card (CS_23103 type 1), or challenges the previous
// player's cards (type 2): a card that is neither the real card nor a joker
// makes the putter the liar. The liar, or the challenger when there was no
// lie, spins the penalty roulette (type 3): bar_punishment_limit chambers,
// one of them (drawn at random for the game) eliminates, so each spin
// survived makes the next likelier to eliminate (wiki, Recent News,
// Tactical Simulation, 9 April 2026). A new round
// follows each spin; the last one standing wins. Every operation goes to
// the table as SC_23105.
//
// Timers (auto_time is the unix time the server acts for a commander): a
// round starts after bar_showcard_time + bar_refreshcard_time (the client's
// round start and real card animations), a put is due bar_card_time later,
// a spin bar_punishment_rotate_time after the challenge's bar_question_time
// reveal; the next round comes bar_punishment_turntable_time after a spin
// (+ bar_punishment_eliminate_time after an elimination, when the
// eliminated commander gets its result). A commander
// running out of time is put on delegate (SC_23115, a GUESS): its puts are
// the first card by key (return_list[1] = 1, which the client mirrors with
// GetMainPlayerAutoPutCard), made barbot_automatic_hosting seconds into its
// turn, until it cancels (CS_23113). Ranks are the order of elimination;
// ranked (MATCH) games score them (island_integral_rank, SC_23108 to each
// commander once out or the winner, then SC_23116 to the table).

// PlayRoomConst.GAME_TYPE.CHEATER_TAVERN
const gameTypeCheatBar = 101

func cheatBarGame(gameType uint32) bool { return gameType == gameTypeCheatBar }

// IslandCheaterTavernConst
const (
	barHand    = 5  // cardNumEveryRound
	barMaxPut  = 3  // putCountMax
	barJoker   = 99 // bar_card 99 (card_99), counts as the real card
	barPutOp   = 1  // PlayerOperateType.PutCard
	barQueryOp = 2  // PlayerOperateType.Query
	barShootOp = 3  // PlayerOperateType.Shoot
)

// SC_23105.next_type: 1 put or challenge, 2 the liar caught spins, 3 the
// challenger mistaken spins (UpdateCurrentBoutDisplay: 1 PutCardOrQuery, 2
// ShootByOther, else ShootByOwn; which spin is which is a GUESS).
const (
	barNextPut         = 1
	barNextShootCaught = 2
	barNextShootOwn    = 3
)

// IslandCheaterTavernConst.PlayerCurrentOperateType, SC_23107.opt_type
const (
	barCurrentPutOrQuery = 2
	barCurrentShoot      = 4
)

var barRules struct {
	bombs         int      // bar_punishment_limit
	deck          []uint32 // bar_card: a card id per card
	realCards     []uint32 // the deck's cards but the joker
	cardTime      time.Duration
	questionTime  time.Duration
	refreshTime   time.Duration
	showTime      time.Duration
	turntableTime time.Duration
	eliminateTime time.Duration
	rotateTime    time.Duration
	hostingTime   time.Duration // barbot_automatic_hosting: GUESS, when a delegated commander's turn is played
	botCardTime   [2]uint32     // barbot_card_time: seconds a bot takes to put, min max
	botShootTime  [2]uint32     // barbot_punishment_rotate_time: to spin
	botWeights    [3]uint32     // barbot_card1..3_weight: GUESS, how many cards a bot puts
	botBase       uint32        // barbot_initial_probability_X: GUESS, % a bot challenges
	botClaim      []uint32      // barbot_increase_probability_M: GUESS, + per cards claimed (1, 2, 3)
	botHold       uint32        // barbot_increase_probability_Y: GUESS, + per real card or joker the bot holds
	botTurn       uint32        // barbot_increase_probability_Z: GUESS, + per put this round
}

func loadCheatBarData() error {
	var gameset map[string]uint32
	var deck [][]uint32
	var cardTime, shootTime, claim []uint32
	err := gamedata.LoadAll(
		func() (err error) { gameset, err = gamedata.Gameset(); return err },
		func() error { return gamedata.GamesetDescription("bar_card", &deck) },
		func() error { return gamedata.GamesetDescription("barbot_card_time", &cardTime) },
		func() error { return gamedata.GamesetDescription("barbot_punishment_rotate_time", &shootTime) },
		func() error { return gamedata.GamesetDescription("barbot_increase_probability_M", &claim) },
	)
	if err != nil {
		return err
	}
	rules := &barRules
	rules.deck, rules.realCards = nil, nil
	for _, row := range deck {
		if len(row) != 2 {
			continue
		}
		for range row[1] {
			rules.deck = append(rules.deck, row[0])
		}
		if row[0] != barJoker {
			rules.realCards = append(rules.realCards, row[0])
		}
	}
	rules.bombs = int(gameset["bar_punishment_limit"])
	if len(rules.realCards) == 0 || len(rules.deck) < barHand*4 || rules.bombs == 0 || len(cardTime) != 2 || len(shootTime) != 2 || len(claim) == 0 {
		return gamedata.Missing("gameset", "cheat bar settings")
	}
	second := func(key string) time.Duration { return time.Duration(gameset[key]) * time.Second }
	rules.cardTime = second("bar_card_time")
	rules.questionTime = second("bar_question_time")
	rules.refreshTime = second("bar_refreshcard_time")
	rules.showTime = second("bar_showcard_time")
	rules.turntableTime = second("bar_punishment_turntable_time")
	rules.eliminateTime = second("bar_punishment_eliminate_time")
	rules.rotateTime = second("bar_punishment_rotate_time")
	rules.hostingTime = second("barbot_automatic_hosting")
	rules.botCardTime = [2]uint32{min(cardTime[0], cardTime[1]), max(cardTime[0], cardTime[1])}
	rules.botShootTime = [2]uint32{min(shootTime[0], shootTime[1]), max(shootTime[0], shootTime[1])}
	rules.botWeights = [3]uint32{gameset["barbot_card1_weight"], gameset["barbot_card2_weight"], gameset["barbot_card3_weight"]}
	rules.botBase = gameset["barbot_initial_probability_X"]
	rules.botClaim = claim
	rules.botHold = gameset["barbot_increase_probability_Y"]
	rules.botTurn = gameset["barbot_increase_probability_Z"]
	return nil
}

func randomSeconds(span [2]uint32) time.Duration {
	return time.Duration(span[0]+uint32(rand.IntN(int(span[1]-span[0])+1))) * time.Second
}

type barCard struct{ key, id uint32 }

type barSeat struct {
	id     uint32
	seat   uint32 // 1-based, the table's order
	bot    bool
	auto   bool // delegated: the server plays it
	gone   bool // left the game
	hand   []barCard
	fired  int // chambers spun
	lethal int // the chamber that eliminates
	out    bool
	rank   uint32
	rounds uint32
}

type barStage int

const (
	barPutting  barStage = iota // the actor puts or challenges
	barShooting                 // the actor spins
	barBetween                  // after a spin, until the next round
	barOver                     // after the last spin, until the end
)

type cheatBar struct {
	game     *Game
	room     *playRoom
	seats    []*barSeat
	real     uint32
	turn     uint32 // rounds dealt
	stage    barStage
	actor    int        // seat index acting
	deadline time.Time  // the actor's auto_time
	actAt    time.Time  // when the server acts for a bot or delegated actor
	nextAt   time.Time  // barBetween, barOver
	table    []barCard  // the last cards put
	putter   int        // who put them
	plays    int        // puts this round
	pending  []*barSeat // eliminated, told their result once the spin's animation is over
}

func newCheatBar(game *Game, room *playRoom, now time.Time) *cheatBar {
	bar := &cheatBar{game: game, room: room, putter: -1}
	for i, id := range room.players() {
		seat := &barSeat{id: id, seat: uint32(i + 1), bot: isPlayBot(id), lethal: 1 + rand.IntN(barRules.bombs)}
		if !seat.bot && (game.playRooms.of[id] != room || !game.isOnline(id)) {
			seat.auto = true
			seat.gone = game.playRooms.of[id] != room
		}
		bar.seats = append(bar.seats, seat)
	}
	return bar
}

func (bar *cheatBar) seatOf(id uint32) *barSeat {
	for _, seat := range bar.seats {
		if seat.id == id {
			return seat
		}
	}
	return nil
}

func (bar *cheatBar) alive() int {
	n := 0
	for _, seat := range bar.seats {
		if !seat.out {
			n++
		}
	}
	return n
}

// tell sends msg to the commanders at the table, behind a requester's
// reply.
func (bar *cheatBar) tell(box *playOutbox, msg proto.Message) {
	for _, seat := range bar.seats {
		if bar.watching(seat) {
			box.after(seat.id, msg)
		}
	}
}

func (bar *cheatBar) watching(seat *barSeat) bool {
	return !seat.bot && !seat.gone && bar.game.playRooms.of[seat.id] == bar.room
}

// deal starts a round: a shuffled deck (keys in dealing order), barHand
// cards to everyone still in, a real card.
func (bar *cheatBar) deal() {
	deck := slices.Clone(barRules.deck)
	rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	next := 0
	for _, seat := range bar.seats {
		seat.hand = nil
		if seat.out {
			continue
		}
		seat.rounds++
		for range barHand {
			if next < len(deck) {
				seat.hand = append(seat.hand, barCard{key: uint32(next + 1), id: deck[next]})
				next++
			}
		}
	}
	bar.real = barRules.realCards[rand.IntN(len(barRules.realCards))]
	bar.table, bar.putter, bar.plays = nil, -1, 0
	bar.turn++
}

// arm sets the actor's timers: lead is what the client shows first, limit
// a commander's time to act after it, botTime a bot's.
func (bar *cheatBar) arm(now time.Time, lead, limit time.Duration, botTime [2]uint32) {
	seat := bar.seats[bar.actor]
	bar.deadline = now.Add(lead + limit)
	switch {
	case seat.bot:
		bar.actAt = now.Add(lead + randomSeconds(botTime))
	case seat.auto:
		bar.actAt = now.Add(lead + min(barRules.hostingTime, limit))
	default:
		bar.actAt = time.Time{}
	}
}

func (bar *cheatBar) cards(seat *barSeat) []*protobuf.PTGC_CHEAT_BAR_CARD {
	hand := slices.Clone(seat.hand)
	slices.SortFunc(hand, func(a, b barCard) int { return int(a.key) - int(b.key) })
	list := make([]*protobuf.PTGC_CHEAT_BAR_CARD, 0, len(hand))
	for _, card := range hand {
		list = append(list, &protobuf.PTGC_CHEAT_BAR_CARD{Key: proto.Uint32(card.key), Id: proto.Uint32(card.id)})
	}
	return list
}

func (bar *cheatBar) player(seat *barSeat) *protobuf.PTGC_CHEAT_BAR_PLAYER {
	bombs := make([]*protobuf.PTGC_CHEAT_BAR_BOMB, barRules.bombs)
	for i := range bombs {
		state := uint32(0)
		if i < seat.fired {
			state = 1
		}
		bombs[i] = &protobuf.PTGC_CHEAT_BAR_BOMB{BombId: proto.Uint32(uint32(i + 1)), State: proto.Uint32(state)}
	}
	auto := uint32(0)
	if seat.auto {
		auto = 1
	}
	return &protobuf.PTGC_CHEAT_BAR_PLAYER{
		UserId:     proto.Uint32(seat.id),
		Seat:       proto.Uint32(seat.seat),
		CardNum:    proto.Uint32(uint32(len(seat.hand))),
		BombList:   bombs,
		IsAuto:     proto.Uint32(auto),
		RoundNum:   proto.Uint32(seat.rounds),
		PlayerInfo: bar.room.members[seat.id],
	}
}

// start deals the first round (SC_23101), a random player first.
func (bar *cheatBar) start(box *playOutbox, now time.Time) {
	bar.deal()
	bar.actor = rand.IntN(len(bar.seats))
	bar.stage = barPutting
	bar.arm(now, barRules.showTime+barRules.refreshTime, barRules.cardTime, barRules.botCardTime)
	players := make([]*protobuf.PTGC_CHEAT_BAR_PLAYER, 0, len(bar.seats))
	for _, seat := range bar.seats {
		players = append(players, bar.player(seat))
	}
	for _, seat := range bar.seats {
		if !bar.watching(seat) {
			continue
		}
		box.after(seat.id, &protobuf.SC_23101{
			PlayerList: players,
			RealCard:   proto.Uint32(bar.real),
			CardList:   bar.cards(seat),
			UserId:     proto.Uint32(bar.seats[bar.actor].id),
			AutoTime:   proto.Uint32(uint32(bar.deadline.Unix())),
			StartTime:  proto.Uint32(uint32(now.Unix())),
			RoomType:   proto.Uint32(bar.room.kind),
		})
	}
}

// nextRound deals again (SC_23102), the spinner first when still in.
func (bar *cheatBar) nextRound(box *playOutbox, now time.Time) {
	bar.deal()
	if bar.seats[bar.actor].out {
		bar.actor = bar.nextAlive(bar.actor)
	}
	bar.stage = barPutting
	bar.arm(now, barRules.showTime+barRules.refreshTime, barRules.cardTime, barRules.botCardTime)
	for _, seat := range bar.seats {
		if !bar.watching(seat) {
			continue
		}
		box.after(seat.id, &protobuf.SC_23102{
			RealCard: proto.Uint32(bar.real),
			CardList: bar.cards(seat),
			UserId:   proto.Uint32(bar.seats[bar.actor].id),
			AutoTime: proto.Uint32(uint32(bar.deadline.Unix())),
			TurnNum:  proto.Uint32(bar.turn),
		})
	}
}

func (bar *cheatBar) nextAlive(from int) int {
	for i := 1; i <= len(bar.seats); i++ {
		if at := (from + i) % len(bar.seats); !bar.seats[at].out {
			return at
		}
	}
	return from
}

// nextActor is who plays after from: the next player still in with cards,
// else the next one still in (who can only challenge).
func (bar *cheatBar) nextActor(from int) int {
	for i := 1; i < len(bar.seats); i++ {
		if at := (from + i) % len(bar.seats); !bar.seats[at].out && len(bar.seats[at].hand) > 0 {
			return at
		}
	}
	return bar.nextAlive(from)
}

// onlyChallenge tells whether the actor can only challenge: nobody else
// still in has cards (IslandCheaterTavernAgency:CheckCanOnlyQurey), or it
// has none itself.
func (bar *cheatBar) onlyChallenge() bool {
	actor := bar.seats[bar.actor]
	if len(actor.hand) == 0 {
		return true
	}
	for i, seat := range bar.seats {
		if i != bar.actor && !seat.out && len(seat.hand) > 0 {
			return false
		}
	}
	return true
}

func (bar *cheatBar) tick(box *playOutbox, now time.Time) {
	switch bar.stage {
	case barPutting, barShooting:
		seat := bar.seats[bar.actor]
		if !bar.actAt.IsZero() && !now.Before(bar.actAt) {
			bar.act(box, now, seat.bot)
			return
		}
		if !now.Before(bar.deadline) {
			if bar.stage == barPutting && !seat.auto {
				bar.setAuto(box, seat, true, now)
			}
			bar.act(box, now, false)
		}
	case barBetween, barOver:
		if now.Before(bar.nextAt) {
			return
		}
		for _, seat := range bar.pending {
			bar.result(box, seat)
		}
		bar.pending = nil
		if bar.stage == barOver {
			bar.finish(box)
		} else {
			bar.nextRound(box, now)
		}
	}
}

// act plays the actor's turn for it: a bot's choice when smart, else the
// first card by key (what the client mirrors for an auto put), a challenge
// when it can do nothing else.
func (bar *cheatBar) act(box *playOutbox, now time.Time, smart bool) {
	if bar.stage == barShooting {
		bar.shoot(box, now)
		return
	}
	seat := bar.seats[bar.actor]
	if bar.onlyChallenge() || (smart && len(bar.table) > 0 && bar.botChallenges(seat)) {
		if len(bar.table) == 0 {
			// nothing to challenge and nothing to put: deal again
			bar.nextRound(box, now)
			return
		}
		bar.query(box, now)
		return
	}
	var cards []barCard
	if smart {
		cards = bar.botCards(seat)
	} else {
		hand := slices.Clone(seat.hand)
		slices.SortFunc(hand, func(a, b barCard) int { return int(a.key) - int(b.key) })
		cards = hand[:1]
	}
	bar.put(box, now, cards, !seat.bot)
}

// botChallenges is a bot's call on the cards just put: GUESS from the
// barbot_* probabilities, the more cards claimed, the more real cards it
// holds itself and the later in the round, the likelier.
func (bar *cheatBar) botChallenges(seat *barSeat) bool {
	chance := barRules.botBase + barRules.botTurn*uint32(bar.plays)
	chance += barRules.botClaim[min(len(bar.table), len(barRules.botClaim))-1]
	for _, card := range seat.hand {
		if card.id == bar.real || card.id == barJoker {
			chance += barRules.botHold
		}
	}
	return rand.IntN(100) < int(chance)
}

// botCards are what a bot puts: 1 to 3 cards (barbot_card*_weight), real
// cards and jokers first.
func (bar *cheatBar) botCards(seat *barSeat) []barCard {
	most := min(barMaxPut, len(seat.hand))
	total := uint32(0)
	for _, weight := range barRules.botWeights[:most] {
		total += weight
	}
	n := 1
	if total > 0 {
		pick := uint32(rand.IntN(int(total)))
		for i, weight := range barRules.botWeights[:most] {
			if pick < weight {
				n = i + 1
				break
			}
			pick -= weight
		}
	}
	hand := slices.Clone(seat.hand)
	rand.Shuffle(len(hand), func(i, j int) { hand[i], hand[j] = hand[j], hand[i] })
	slices.SortStableFunc(hand, func(a, b barCard) int {
		honest := func(card barCard) int {
			if card.id == bar.real || card.id == barJoker {
				return 0
			}
			return 1
		}
		return honest(a) - honest(b)
	})
	return hand[:n]
}

// put lays the actor's cards on the table, the next player's turn.
func (bar *cheatBar) put(box *playOutbox, now time.Time, cards []barCard, auto bool) {
	seat := bar.seats[bar.actor]
	seat.hand = slices.DeleteFunc(seat.hand, func(card barCard) bool { return slices.Contains(cards, card) })
	bar.table, bar.putter = cards, bar.actor
	bar.plays++
	bar.actor = bar.nextActor(bar.actor)
	bar.stage = barPutting
	bar.arm(now, 0, barRules.cardTime, barRules.botCardTime)
	flag := uint32(0)
	if auto {
		flag = 1
	}
	bar.tell(box, &protobuf.SC_23105{
		Type:       proto.Uint32(barPutOp),
		UserId:     proto.Uint32(seat.id),
		ReturnList: []uint32{flag, uint32(len(cards))},
		NextUserId: proto.Uint32(bar.seats[bar.actor].id),
		NextType:   proto.Uint32(barNextPut),
		AutoTime:   proto.Uint32(uint32(bar.deadline.Unix())),
	})
}

// query challenges the cards on the table (return_list: 1 a lie caught, 2
// not, then the cards' ids); whoever was wrong spins.
func (bar *cheatBar) query(box *playOutbox, now time.Time) {
	challenger := bar.seats[bar.actor]
	lie := false
	ids := []uint32{2}
	for _, card := range bar.table {
		ids = append(ids, card.id)
		if card.id != bar.real && card.id != barJoker {
			lie = true
		}
	}
	next := uint32(barNextShootOwn)
	if lie {
		ids[0], next = 1, barNextShootCaught
		bar.actor = bar.putter
	}
	bar.table = nil
	bar.stage = barShooting
	bar.arm(now, barRules.questionTime, barRules.rotateTime, barRules.botShootTime)
	bar.tell(box, &protobuf.SC_23105{
		Type:       proto.Uint32(barQueryOp),
		UserId:     proto.Uint32(challenger.id),
		ReturnList: ids,
		NextUserId: proto.Uint32(bar.seats[bar.actor].id),
		NextType:   proto.Uint32(next),
		AutoTime:   proto.Uint32(uint32(bar.deadline.Unix())),
	})
}

// shoot spins the actor's roulette (return_list: the chamber, 1 when it
// eliminates); the next round, or the end, follows the animation.
func (bar *cheatBar) shoot(box *playOutbox, now time.Time) {
	seat := bar.seats[bar.actor]
	seat.fired++
	dead := uint32(0)
	delay := barRules.turntableTime
	if seat.fired >= seat.lethal {
		dead = 1
		seat.out, seat.hand = true, nil
		seat.rank = uint32(bar.alive() + 1)
		delay += barRules.eliminateTime
	}
	bar.tell(box, &protobuf.SC_23105{
		Type:       proto.Uint32(barShootOp),
		UserId:     proto.Uint32(seat.id),
		ReturnList: []uint32{uint32(seat.fired), dead},
		NextUserId: proto.Uint32(0),
		NextType:   proto.Uint32(0),
		AutoTime:   proto.Uint32(0),
	})
	if dead == 1 {
		bar.pending = append(bar.pending, seat)
	}
	bar.actAt, bar.nextAt = time.Time{}, now.Add(delay)
	bar.stage = barBetween
	if bar.alive() <= 1 {
		bar.stage = barOver
	}
}

// result tells a commander its rank and score (SC_23108), scoring a ranked
// game first; who left was charged already (quitRanked).
func (bar *cheatBar) result(box *playOutbox, seat *barSeat) {
	if seat.bot || seat.gone {
		return
	}
	game, id, rank := bar.game, seat.id, seat.rank
	gameType, ranked := bar.room.gameType, bar.room.kind == playRoomMatch
	box.do(func() {
		score, err := game.addPlayRoomScore(context.Background(), id, gameType, rank, ranked)
		if err != nil {
			logger.WithFields("PlayRoom", logger.FieldValue("commander", id), logger.FieldValue("error", err)).Warn("score not saved")
			return
		}
		box.send(id, &protobuf.SC_23108{Rank: proto.Uint32(rank), CurScore: proto.Uint32(score)}, true)
	})
}

// finish ends the game: the winner's result, the winner to the table
// (SC_23116), the room back to its lobby.
func (bar *cheatBar) finish(box *playOutbox) {
	winner := uint32(0)
	for _, seat := range bar.seats {
		if !seat.out {
			seat.rank = 1
			winner = seat.id
			bar.result(box, seat)
		}
	}
	msg := &protobuf.SC_23116{WinUser: proto.Uint32(winner)}
	for _, seat := range bar.seats {
		if bar.watching(seat) {
			box.after(seat.id, msg)
		}
	}
	bar.game.endPlayGame(box, bar.room)
}

// setAuto turns a commander's delegation on or off (SC_23115 to the table).
func (bar *cheatBar) setAuto(box *playOutbox, seat *barSeat, on bool, now time.Time) {
	if seat.auto == on {
		return
	}
	seat.auto = on
	state := uint32(0)
	if on {
		state = 1
	}
	bar.tell(box, &protobuf.SC_23115{UserId: proto.Uint32(seat.id), State: proto.Uint32(state)})
	if (bar.stage == barPutting || bar.stage == barShooting) && bar.seats[bar.actor] == seat {
		if on {
			bar.actAt = now.Add(min(barRules.hostingTime, max(0, bar.deadline.Sub(now))))
		} else {
			bar.actAt = time.Time{}
		}
	}
}

// delegate has the server play for a commander who left or lost its
// connection.
func (bar *cheatBar) delegate(box *playOutbox, id uint32) {
	if seat := bar.seatOf(id); seat != nil && !seat.out {
		bar.setAuto(box, seat, true, time.Now())
	}
}

// playingSeat is the commander's seat in its room's game, nil outside one.
func (game *Game) playingSeat(id uint32) (*cheatBar, *barSeat) {
	room := game.playRooms.of[id]
	if room == nil || room.phase != playPlaying || room.game == nil {
		return nil, nil
	}
	seat := room.game.seatOf(id)
	if seat == nil || seat.gone {
		return nil, nil
	}
	return room.game, seat
}

// handleCheatBarOperate answers IslandCheaterOperateCommand (CS_23103): the
// commander's turn, a put (type 1, arg_list the cards' keys, 1 to
// putCountMax of its own, not when it can only challenge), a challenge (2,
// cards on the table) or a spin (3, when it must). The table hears of it
// behind the reply (SC_23105).
func (game *Game) handleCheatBarOperate(client *connection.Client, req *protobuf.CS_23103) error {
	now := time.Now()
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := func() error {
		bar, seat := game.playingSeat(client.CommanderID)
		if bar == nil || seat.out || bar.seats[bar.actor] != seat {
			return errRefused
		}
		switch req.GetType() {
		case barPutOp:
			keys := req.GetArgList()
			if bar.stage != barPutting || len(keys) == 0 || len(keys) > barMaxPut || bar.onlyChallenge() {
				return errRefused
			}
			var cards []barCard
			for _, key := range keys {
				at := slices.IndexFunc(seat.hand, func(card barCard) bool { return card.key == key })
				if at < 0 || slices.Contains(cards, seat.hand[at]) {
					return errRefused
				}
				cards = append(cards, seat.hand[at])
			}
			bar.put(box, now, cards, false)
		case barQueryOp:
			if bar.stage != barPutting || len(bar.table) == 0 {
				return errRefused
			}
			bar.query(box, now)
		case barShootOp:
			if bar.stage != barShooting {
				return errRefused
			}
			bar.shoot(box, now)
		default:
			return errRefused
		}
		return nil
	}()
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23104{Result: result(err)})
}

// handleCheatBarReconnect answers IslandCheaterReconectCommand (CS_23106):
// the game the commander comes back to, its cards and whose turn it is.
func (game *Game) handleCheatBarReconnect(client *connection.Client, req *protobuf.CS_23106) error {
	pr := game.lockPlayRooms(client)
	reply := &protobuf.SC_23107{Result: result(errRefused)}
	if bar, seat := game.playingSeat(client.CommanderID); bar != nil {
		reply = &protobuf.SC_23107{
			Result:   result(nil),
			RealCard: proto.Uint32(bar.real),
			CardList: bar.cards(seat),
			CardNum:  proto.Uint32(uint32(len(bar.table))),
			UserId:   proto.Uint32(bar.seats[bar.actor].id),
			OptType:  proto.Uint32(barCurrentPutOrQuery),
			AutoTime: proto.Uint32(uint32(bar.deadline.Unix())),
			TurnNum:  proto.Uint32(bar.turn),
			RoomType: proto.Uint32(bar.room.kind),
		}
		if bar.stage == barShooting {
			reply.OptType = proto.Uint32(barCurrentShoot)
		}
		for _, other := range bar.seats {
			if other.out {
				reply.OutPlayerList = append(reply.OutPlayerList, bar.player(other))
			} else {
				reply.PlayerList = append(reply.PlayerList, bar.player(other))
			}
		}
	}
	pr.mu.Unlock()
	return client.Send(reply)
}

// handleCheatBarCancelDelegate answers IslandCheaterCancelDelegateCommand
// (CS_23113): the commander plays its turns again (SC_23115 to the table).
func (game *Game) handleCheatBarCancelDelegate(client *connection.Client, req *protobuf.CS_23113) error {
	box := game.playOutbox(client)
	pr := game.lockPlayRooms(client)
	err := errRefused
	if bar, seat := game.playingSeat(client.CommanderID); bar != nil && !seat.out {
		bar.setAuto(box, seat, false, time.Now())
		err = nil
	}
	pr.mu.Unlock()
	box.flush()
	return client.Send(&protobuf.SC_23114{Result: result(err)})
}
