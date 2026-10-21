package game

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The auction minigame ("Top Bidder", ActivityConst.ACTIVITY_TYPE_AUCTION_GAME,
// protocol module 23 ids 234xx): AuctionGameBaseProxy keeps the commander's
// Bid Coins (the activity's config_client itemID, an
// item_virtual_data_statistics virtual_type 31 item that Drop adds to the
// proxy's gold, capped at auction_currency_ceiling) and statistics, the
// entrance (AuctionGameEntranceScene) orders item boxes, pays relief and
// starts the matchmaking of the auction houses (auction_session rows with a
// game_type); a match (AuctionGameProxy, auction_match.go) is four bidders,
// filled with bots, bidding on a warehouse of auction_collection items over
// up to five rounds (auction_round).
//
// Built from the Lua alone (only 23430/23431 were captured): the commands
// in controller/command/auctiongame, the proxies, the views in
// view/activity/auctiongame. What the client doesn't decide is marked
// GUESS.

const activityTypeAuctionGame = 136

// AuctionGameConst.CELL_COL_CNT, and the grid lines the client always shows
// (AuctionGameProxy:GetMaxLineCnt)
const (
	auctionGridColumns = 10
	auctionGridLines   = 10
)

// auctionPlayers is the bidders of a match: AuctionGameMainRightView and the
// round over layer have four player slots.
const auctionPlayers = 4

// Task sub types of the auction tasks (task_data_template rows of activity
// 970001; the server counts them, their descs say what)
const (
	TaskSubAuctionJoin     = 3001 // "Participate in auctions or order items N times"
	TaskSubAuctionWin      = 3002 // "Win N auctions"
	TaskSubAuctionLot      = 3003 // "Win items worth at least N in a single auction" (absolute)
	TaskSubAuctionSession  = 3004 // "Participate in N auctions", target: game_type
	TaskSubAuctionSpent    = 3005 // "Spend a total of N Bid Coins at auctions or ordering items"
	TaskSubAuctionGold     = 3006 // "Own N Bid Coins" (absolute)
	TaskSubAuctionLotTotal = 3007 // as 3003, the task type 20 ones (absolute)
)

// auction_session row: an auction house (game_type, matched with CS_23418)
// or the ordered item box (game_type 0). rarity_limit is the least items of
// each rarity a lot holds, rarity_probability the weight of each rarity of
// the other items, value_range weighted {{lo, hi}, weight} ranges of the
// lot's total value.
type auctionSessionConfig struct {
	ID                uint32               `json:"id"`
	Name              string               `json:"name"`
	GameType          uint32               `json:"game_type"`
	Threshold         uint32               `json:"threshold"`
	Ticket            uint32               `json:"ticket"`
	BottomPrice       uint32               `json:"bottom_price"`
	RarityLimit       [][2]uint32          `json:"rarity_limit"`
	RarityProbability [][2]uint32          `json:"rarity_probability"`
	ValueRange        [][2]json.RawMessage `json:"value_range"`

	values  []auctionValueRange
	botBids [][2]uint32 // gameset auction_bot_bid_<tier>, by round
}

type auctionValueRange struct{ lo, hi, weight uint32 }

// auction_event row: group 1 the personal events a player picks from,
// group 2 the round's public one; type is AuctionGameConst.EVENT_TYPE.
type auctionEventConfig struct {
	ID         uint32   `json:"id"`
	Type       uint32   `json:"type"`
	Group      uint32   `json:"group"`
	ConfigData []uint32 `json:"config_data"`
}

// auction_collection row: contour is {width, height} in grid cells.
type auctionCollectionConfig struct {
	ID      uint32    `json:"id"`
	Rarity  uint32    `json:"rarity"`
	Value   uint32    `json:"value"`
	Contour [2]uint32 `json:"contour"`
}

func (item auctionCollectionConfig) area() uint32 { return item.Contour[0] * item.Contour[1] }

// auctionBotShip is a gameset auction_bot_ship ship a bot shows.
type auctionBotShip struct {
	id, skin uint32
	name     string
}

var auction struct {
	sessions   map[uint32]auctionSessionConfig
	byGameType map[uint32]uint32 // game_type -> session id
	preorder   uint32            // the game_type 0 session
	oneHit     []float64         // auction_round one_hit, round 1 first
	events     map[uint32]auctionEventConfig
	personal   []uint32 // group 1 events
	public     []uint32 // group 2 events
	collection map[uint32]auctionCollectionConfig
	byRarity   map[uint32][]uint32
	emojis     map[uint32]bool   // emoji_template auction 1
	coins      map[uint32]bool   // virtual_type 31 items
	botShips   []auctionBotShip  // auction_bot_ship
	botFrames  []uint32          // auction_bot_frame
	botEvent   [2]uint32         // auction_bot_time_event, seconds
	botBid     [2]uint32         // auction_bot_time_bid, seconds
	matchWait  [2]uint32         // auction_matching_limit, seconds
	settings   map[string]uint32 // gameset auction_* key_value
}

// auctionSetting is gameset auction_<key>.key_value.
func auctionSetting(key string) uint32 { return auction.settings[key] }

// auctionSettings the handlers read
var auctionSettingKeys = []string{
	"bid_time", "event_choose_time", "publicity_time", "match_loading_timeout",
	"preorder_price", "relief_payment", "relief_payment_count", "relief_payment_limit",
	"currency_ceiling", "divided", "emoji_cd", "maximum",
}

func loadAuctionData() error {
	var rounds map[uint32]struct {
		OneHit string `json:"one_hit"`
	}
	var emojis map[uint32]struct {
		Auction uint32 `json:"auction"`
	}
	var vitems map[uint32]struct {
		VirtualType uint32 `json:"virtual_type"`
	}
	var ships map[uint32]struct {
		Name   string `json:"name"`
		SkinID uint32 `json:"skin_id"`
	}
	var gameset map[string]uint32
	var botShips []uint32
	err := gamedata.LoadAll(
		gamedata.Load("auction_session", &auction.sessions),
		gamedata.Load("auction_round", &rounds),
		gamedata.Load("auction_event", &auction.events),
		gamedata.Load("auction_collection", &auction.collection),
		gamedata.Load("emoji_template", &emojis),
		gamedata.Load("item_virtual_data_statistics", &vitems),
		gamedata.Load("ship_data_statistics", &ships),
		func() (err error) { gameset, err = gamedata.Gameset(); return err },
		func() error { return gamedata.GamesetDescription("auction_bot_ship", &botShips) },
		func() error { return gamedata.GamesetDescription("auction_bot_frame", &auction.botFrames) },
		func() error { return gamedata.GamesetDescription("auction_bot_time_event", &auction.botEvent) },
		func() error { return gamedata.GamesetDescription("auction_bot_time_bid", &auction.botBid) },
		func() error { return gamedata.GamesetDescription("auction_matching_limit", &auction.matchWait) },
	)
	if err != nil {
		return err
	}
	auction.settings = map[string]uint32{}
	for _, key := range auctionSettingKeys {
		value, ok := gameset["auction_"+key]
		if !ok {
			return gamedata.Missing("gameset", "auction_%s", key)
		}
		auction.settings[key] = value
	}
	auction.settings["preorder_open"] = gameset["auction_preorder_open"]

	auction.byGameType, auction.preorder = map[uint32]uint32{}, 0
	for _, id := range slices.Sorted(maps.Keys(auction.sessions)) {
		session := auction.sessions[id]
		for _, entry := range session.ValueRange {
			var bounds [2]uint32
			var weight uint32
			if json.Unmarshal(entry[0], &bounds) != nil || json.Unmarshal(entry[1], &weight) != nil || bounds[1] < bounds[0] {
				return gamedata.Missing("auction_session", "value_range of %d", id)
			}
			session.values = append(session.values, auctionValueRange{bounds[0], bounds[1], weight})
		}
		if len(session.values) == 0 {
			return gamedata.Missing("auction_session", "value_range of %d", id)
		}
		if session.GameType == 0 {
			auction.preorder = id
		} else {
			// the bots' bids of a house: auction_bot_bid_b for the B-Tier...
			tier := strings.ToLower(strings.TrimSpace(session.Name))
			if tier == "" {
				return gamedata.Missing("auction_session", "name of %d", id)
			}
			if err := gamedata.GamesetDescription("auction_bot_bid_"+tier[:1], &session.botBids); err != nil {
				return err
			}
			auction.byGameType[session.GameType] = id
		}
		auction.sessions[id] = session
	}
	if auction.preorder == 0 {
		return gamedata.Missing("auction_session", "the ordered box (game_type 0)")
	}

	auction.oneHit = nil
	for _, id := range slices.Sorted(maps.Keys(rounds)) {
		value, err := strconv.ParseFloat(strings.TrimSpace(rounds[id].OneHit), 64)
		if err != nil {
			return gamedata.Missing("auction_round", "one_hit of %d", id)
		}
		auction.oneHit = append(auction.oneHit, value)
	}
	if len(auction.oneHit) == 0 {
		return gamedata.Missing("auction_round", "rounds")
	}
	for _, session := range auction.sessions {
		if session.GameType != 0 && len(session.botBids) < len(auction.oneHit) {
			return gamedata.Missing("gameset", "auction_bot_bid of every round for %s", session.Name)
		}
	}

	auction.personal, auction.public = nil, nil
	for _, id := range slices.Sorted(maps.Keys(auction.events)) {
		switch auction.events[id].Group {
		case auctionEventPersonal:
			auction.personal = append(auction.personal, id)
		case auctionEventPublic:
			auction.public = append(auction.public, id)
		}
	}
	if len(auction.personal) < auctionPersonalEvents || len(auction.public) == 0 {
		return gamedata.Missing("auction_event", "personal and public events")
	}

	auction.byRarity = map[uint32][]uint32{}
	for _, id := range slices.Sorted(maps.Keys(auction.collection)) {
		item := auction.collection[id]
		if item.area() == 0 || item.Contour[0] > auctionGridColumns {
			return gamedata.Missing("auction_collection", "contour of %d", id)
		}
		auction.byRarity[item.Rarity] = append(auction.byRarity[item.Rarity], id)
	}

	auction.emojis = map[uint32]bool{}
	for id, emoji := range emojis {
		if emoji.Auction == 1 {
			auction.emojis[id] = true
		}
	}
	auction.coins = map[uint32]bool{}
	for id, item := range vitems {
		if item.VirtualType == 31 {
			auction.coins[id] = true
		}
	}
	auction.botShips = nil
	for _, id := range botShips {
		ship, ok := ships[id]
		if !ok {
			return gamedata.Missing("ship_data_statistics", "auction bot ship %d", id)
		}
		auction.botShips = append(auction.botShips, auctionBotShip{id: id, skin: ship.SkinID, name: ship.Name})
	}
	if len(auction.botShips) == 0 || len(auction.botFrames) == 0 {
		return gamedata.Missing("gameset", "auction_bot_ship / auction_bot_frame")
	}
	return nil
}

// auctionActivity is the auction activity running for the commander at now
// (ActivityProxy:getActivityByType(ACTIVITY_TYPE_AUCTION_GAME)) and when it
// stops (zero: never), 0 when none runs.
func auctionActivity(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (uint32, time.Time, error) {
	var own map[uint32]uint32
	ids := slices.Sorted(maps.Keys(activityConfigs))
	for _, id := range ids {
		config := activityConfigs[id]
		if config.Type != activityTypeAuctionGame {
			continue
		}
		if _, open := openActivity(id, now); open {
			return id, config.Time.End, nil
		}
		if own == nil {
			var err error
			if own, err = commanderActivities(ctx, queries, commanderID, now); err != nil {
				return 0, time.Time{}, err
			}
		}
		if stop, running := own[id]; running {
			if stop == 0 {
				return id, time.Time{}, nil
			}
			return id, time.Unix(int64(stop), 0), nil
		}
	}
	return 0, time.Time{}, nil
}

// auctionState is the commander's stored auction state: a new one when
// nothing is stored or it was kept for another auction activity than
// activityID (0: keep whatever is stored), with the daily counts reset at
// midnight, server time.
func auctionState(ctx context.Context, queries *db.Queries, commanderID, activityID uint32, now time.Time) (db.CommanderAuction, error) {
	state, err := queries.GetAuction(ctx, commanderID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		state, err = db.CommanderAuction{CommanderID: commanderID, ActivityID: activityID}, nil
	case err == nil && activityID != 0 && state.ActivityID != 0 && state.ActivityID != activityID:
		state = db.CommanderAuction{CommanderID: commanderID, ActivityID: activityID}
	case err == nil && state.ActivityID == 0:
		state.ActivityID = activityID
	}
	if err != nil {
		return state, err
	}
	if today := startOfDay(now).Unix(); state.ResetDay < today {
		// GUESS: the relief payments are daily (auction_relief_tip_2
		// "Claims remaining today"), so are the inactivity counts here
		state.InactiveNum, state.InactiveState, state.GetReliefNum, state.ResetDay = 0, 0, 0, today
	}
	return state, nil
}

func saveAuctionState(ctx context.Context, queries *db.Queries, state db.CommanderAuction) error {
	return queries.SaveAuction(ctx, db.SaveAuctionParams(state))
}

// auctionAddGold is AuctionGameBaseProxy:AddGold: gold changed by delta,
// capped at auction_currency_ceiling (and never below 0).
func auctionAddGold(gold uint32, delta int64) uint32 {
	value := min(max(int64(gold)+delta, 0), int64(auctionSetting("currency_ceiling")))
	return uint32(value)
}

// addAuctionGold adds Bid Coins (a virtual_type 31 drop, Drop's case 31) to
// the commander's auction state.
func (tx *Tx) addAuctionGold(count uint32) error {
	now := time.Now()
	activityID, _, err := auctionActivity(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return err
	}
	state, err := auctionState(tx.ctx, tx.Queries, tx.commanderID, activityID, now)
	if err != nil {
		return err
	}
	state.Gold = auctionAddGold(state.Gold, int64(count))
	tx.event(TaskEvent{SubType: TaskSubAuctionGold, Count: state.Gold})
	return saveAuctionState(tx.ctx, tx.Queries, state)
}

// runningAuction is the auction activity running for the commander with
// its state, errRefused when none runs.
func runningAuction(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (db.CommanderAuction, time.Time, error) {
	activityID, stop, err := auctionActivity(ctx, queries, commanderID, now)
	if err != nil {
		return db.CommanderAuction{}, stop, err
	}
	if activityID == 0 {
		return db.CommanderAuction{}, stop, errRefused
	}
	state, err := auctionState(ctx, queries, commanderID, activityID, now)
	return state, stop, err
}

// handleAuctionGameInit answers AuctionGameInitCommand (CS_23430, sent at
// every login and when entering the auction screens): AuctionGameBaseProxy's
// state. Answered from what is stored even when no auction runs.
func (game *Game) handleAuctionGameInit(client *connection.Client, req *protobuf.CS_23430) error {
	ctx := context.Background()
	now := time.Now()
	activityID, _, err := auctionActivity(ctx, game.DB.Queries, client.CommanderID, now)
	if err != nil {
		return err
	}
	state, err := auctionState(ctx, game.DB.Queries, client.CommanderID, activityID, now)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_23431{
		Gold:          proto.Uint32(state.Gold),
		BuyNum:        proto.Uint32(state.BuyNum),
		MaxProfit:     proto.Uint32(state.MaxProfit),
		AccProfit:     proto.Int32(int32(max(min(state.AccProfit, 1<<31-1), -1<<31))),
		ItemList:      state.ItemList,
		AccBuyPrice:   proto.Uint32(state.AccBuyPrice),
		PreBuyState:   proto.Uint32(state.PreBuyState),
		PreTimestamp:  proto.Uint32(uint32(state.PreTimestamp)),
		MatchTime:     proto.Uint32(uint32(state.MatchTime)),
		IsForbidden:   proto.Uint32(state.IsForbidden),
		GameNum:       proto.Uint32(state.GameNum),
		InactiveNum:   proto.Uint32(state.InactiveNum),
		InactiveState: proto.Uint32(state.InactiveState),
		BackForbidden: proto.Uint32(state.BackForbidden),
		AccItemPrice:  proto.Uint32(state.AccItemPrice),
		GetReliefNum:  proto.Uint32(state.GetReliefNum),
	})
}

// handleAuctionGamePreorderBox answers AuctionGamePreorderBoxCommand
// (CS_23420, the entrance's order button): the box costs
// auction_preorder_price coins (the client takes them on success) and opens
// from timestamp. Refused while a box waits, without the coins, when
// auction_preorder_open is off, and on the activity's last day (the
// entrance hides the button then, uiPreorderEndGo).
//
// GUESS: the box opens at the next midnight, server time (the entrance
// counts down to it in hours; no order on the last day so that it opens
// before the activity ends). Its items are drawn from the game_type 0
// auction_session row when ordered and kept.
func (game *Game) handleAuctionGamePreorderBox(client *connection.Client, req *protobuf.CS_23420) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_23421{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, stop, err := runningAuction(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		price := auctionSetting("preorder_price")
		if auctionSetting("preorder_open") != 1 || state.PreBuyState != 0 || state.Gold < price || (!stop.IsZero() && sameServerDay(now, stop)) {
			return errRefused
		}
		lot := newAuctionLot(auction.sessions[auction.preorder])
		box, err := json.Marshal(lot.Items)
		if err != nil {
			return err
		}
		state.Gold -= price
		state.PreBuyState, state.PreTimestamp, state.PreBox, state.PreLine = 1, nextDay(now).Unix(), string(box), lot.Lines
		reply.Timestamp = proto.Uint32(uint32(state.PreTimestamp))
		tx.event(
			TaskEvent{SubType: TaskSubAuctionJoin, Count: 1},
			TaskEvent{SubType: TaskSubAuctionSpent, Count: price},
			TaskEvent{SubType: TaskSubAuctionGold, Count: state.Gold},
		)
		return saveAuctionState(ctx, tx.Queries, state)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleAuctionGameOpenBox answers AuctionGameOpenBoxCommand (CS_23422, the
// entrance's open button once the box's timestamp passed): the box's items
// (AuctionGameBaseProxy:UpdateSettlementData, shown by
// AuctionGamePreorderBoxSettlementScene, which adds their value to the
// coins itself) and its grid lines. The commander's coins get their value
// and the box is gone (pre_buy_state 0: another one can be ordered).
//
// GUESS: the items join the item logbook (item_list) like won ones.
func (game *Game) handleAuctionGameOpenBox(client *connection.Client, req *protobuf.CS_23422) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_23423{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, _, err := runningAuction(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if state.PreBuyState != 1 || now.Unix() < state.PreTimestamp {
			return errRefused
		}
		var lot auctionLot
		if err := json.Unmarshal([]byte(state.PreBox), &lot.Items); err != nil {
			return fmt.Errorf("auction box of %d: %w", client.CommanderID, err)
		}
		lot.Lines = max(state.PreLine, auctionGridLines)
		for _, item := range lot.Items {
			reply.ItemList = append(reply.ItemList, item.full())
		}
		reply.Line = proto.Uint32(lot.Lines)
		state.Gold = auctionAddGold(state.Gold, int64(lot.value()))
		state.ItemList = auctionUnlock(state.ItemList, lot)
		state.PreBuyState, state.PreTimestamp, state.PreBox, state.PreLine = 0, 0, "", 0
		tx.event(TaskEvent{SubType: TaskSubAuctionGold, Count: state.Gold})
		return saveAuctionState(ctx, tx.Queries, state)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// auctionUnlock adds the lot's collection ids to the item logbook.
func auctionUnlock(list db.IDList, lot auctionLot) db.IDList {
	for _, item := range lot.Items {
		if !slices.Contains(list, item.ID) {
			list = append(list, item.ID)
		}
	}
	slices.SortFunc(list, cmp.Compare)
	return list
}

// handleAuctionGameShowMatchWarningTip answers
// AuctionGameShowMatchWarningTipCommand (CS_23424): the entrance warned the
// commander once removed from a match (inactive_num 1) that leaving matches
// is punished; it isn't warned again (inactive_state 1).
func (game *Game) handleAuctionGameShowMatchWarningTip(client *connection.Client, req *protobuf.CS_23424) error {
	ctx := context.Background()
	now := time.Now()
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, _, err := runningAuction(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		state.InactiveState = 1
		return saveAuctionState(ctx, tx.Queries, state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_23425{Result: result(err)})
}

// handleAuctionGameGetRelief answers AuctionGameGetReliefCommand (CS_23426,
// the entrance's relief button, shown under auction_relief_payment_limit
// coins while relief payments are left today, AuctionGameActivity:
// GetReliefTip): auction_relief_payment coins, which the client shows as an
// award (it reads the coins back from SC_23431 later).
func (game *Game) handleAuctionGameGetRelief(client *connection.Client, req *protobuf.CS_23426) error {
	ctx := context.Background()
	now := time.Now()
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, _, err := runningAuction(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if state.Gold >= auctionSetting("relief_payment_limit") || state.GetReliefNum >= auctionSetting("relief_payment_count") {
			return errRefused
		}
		state.Gold = auctionAddGold(state.Gold, int64(auctionSetting("relief_payment")))
		state.GetReliefNum++
		tx.event(TaskEvent{SubType: TaskSubAuctionGold, Count: state.Gold})
		return saveAuctionState(ctx, tx.Queries, state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_23427{Result: result(err)})
}
