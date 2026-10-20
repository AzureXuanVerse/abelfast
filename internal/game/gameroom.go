package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// GameRoomProxy.coin_res_id and ticket_res_id: the Arcade's coins (credits)
// and the Game Tickets they earn.
const (
	resourceGameCoin   = 11
	resourceGameTicket = 12
)

// gameRoomSettings are the Arcade's gameset settings.
var gameRoomSettings struct {
	coinInitial uint32 // game_coin_initial: the first visit's coins
	coinMax     uint32 // game_coin_max: the most coins one can hold
	coinWeek    uint32 // game_coin_week: the weekly free coins
	ticketMonth uint32 // game_ticket_month: the most tickets earned a month
	ticketMax   uint32 // game_room_remax: the most tickets one can hold
	// game_coin_gold: [n, gold] the price of each coin bought after the
	// n-th of the week (GameHallExchangePanel:getPriceByCount)
	coinGold [][2]uint32
}

// game_room_template row: an Arcade game.
type gameRoomConfig struct {
	// tickets per coin spent: add_base times the multiplier of the highest
	// add_num [score, multiplier] the score reaches (the games' help:
	// "0-5 points: 180 Game Tickets" for add_base 200 and [0, 0.9])
	AddBase uint32       `json:"add_base"`
	AddNum  [][2]float64 `json:"add_num"`
	AddType uint32       `json:"add_type"` // the player_resource earned
	CoinMax uint32       `json:"coin_max"` // the most coins one game takes
	// [[y,m,d],[h,m,s]] server time: the Arcade lists the game from then
	// (GameHallListPanel)
	UnlockTime json.RawMessage `json:"unlock_time"`
	unlock     time.Time
}

var gameRooms map[uint32]gameRoomConfig

// gameroom_shop_template row: Prize Shop goods (MiniGameGoods).
type miniGameGoodsConfig struct {
	PurchaseLimit uint32 `json:"goods_purchase_limit"`
	// what one purchase gives: num of a drop of drop_type, goods[1] or, for
	// goods_type 2, one of goods picked by the player (MiniGameGoods:Selectable)
	DropType  uint32   `json:"drop_type"`
	Goods     []uint32 `json:"goods"`
	GoodsType uint32   `json:"goods_type"`
	Num       uint32   `json:"num"`
	Price     uint32   `json:"price"` // in Game Tickets, per purchase
	// non-zero: the purchases count again from each refresh
	// (MiniGameShop:setNextTime)
	MonthRe uint32          `json:"month_re"`
	Time    gamedata.Window `json:"time"`
}

var miniGameGoods map[uint32]miniGameGoodsConfig

func loadGameRoomData() error {
	gameset, err := gamedata.Gameset()
	if err != nil {
		return err
	}
	settings := []struct {
		key   string
		value *uint32
	}{
		{"game_coin_initial", &gameRoomSettings.coinInitial},
		{"game_coin_max", &gameRoomSettings.coinMax},
		{"game_coin_week", &gameRoomSettings.coinWeek},
		{"game_ticket_month", &gameRoomSettings.ticketMonth},
		{"game_room_remax", &gameRoomSettings.ticketMax},
	}
	for _, setting := range settings {
		value, ok := gameset[setting.key]
		if !ok {
			return gamedata.Missing("gameset", "%s", setting.key)
		}
		*setting.value = value
	}
	if err := gamedata.GamesetDescription("game_coin_gold", &gameRoomSettings.coinGold); err != nil {
		return err
	}
	rooms, err := gamedata.Table[gameRoomConfig]("game_room_template")
	if err != nil {
		return err
	}
	for id, room := range rooms {
		var unlock [2][3]int
		if json.Unmarshal(room.UnlockTime, &unlock) != nil {
			return gamedata.Missing("game_room_template", "%d unlock_time", id)
		}
		room.unlock = time.Date(unlock[0][0], time.Month(unlock[0][1]), unlock[0][2], unlock[1][0], unlock[1][1], unlock[1][2], 0, gamedata.Zone)
		rooms[id] = room
	}
	goods, err := gamedata.Table[miniGameGoodsConfig]("gameroom_shop_template")
	if err != nil {
		return err
	}
	gameRooms, miniGameGoods = rooms, goods
	return nil
}

// gameRoomState is the commander's Arcade counters as of now: the weekly
// ones start again every Monday 00:00 server time, the monthly ones on the
// first of the month.
type gameRoomState struct {
	firstEnter    uint32
	weeklyFree    bool   // this week's free coins were taken
	payCoinCount  uint32 // coins bought this week
	monthlyTicket uint32 // tickets earned this month
}

func loadGameRoomState(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (gameRoomState, error) {
	row, err := queries.GetGameRoom(ctx, commanderID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return gameRoomState{}, err
	}
	state := gameRoomState{firstEnter: row.FirstEnter}
	thisWeek := periodStart(now, week).Unix()
	state.weeklyFree = row.WeeklyFreeWeek >= thisWeek
	if row.PayWeek == thisWeek {
		state.payCoinCount = row.PayCoinCount
	}
	if row.TicketMonth == startOfMonth(now).Unix() {
		state.monthlyTicket = row.MonthlyTicket
	}
	return state, nil
}

// gameRoomInfo is SC_26120, GameRoomProxy's Arcade state: whether the first
// visit's coins and this week's free ones were taken, the tickets earned
// this month, the coins bought this week and each game's best score. The
// client never resets the counters itself: they are only right again at
// the next login.
func gameRoomInfo(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.SC_26120, error) {
	state, err := loadGameRoomState(ctx, queries, commanderID, now)
	if err != nil {
		return nil, err
	}
	scores, err := queries.ListGameRoomScores(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	weekly := uint32(0)
	if state.weeklyFree {
		weekly = 1
	}
	info := &protobuf.SC_26120{
		WeeklyFree:    proto.Uint32(weekly),
		MonthlyTicket: proto.Uint32(state.monthlyTicket),
		PayCoinCount:  proto.Uint32(state.payCoinCount),
		FirstEnter:    proto.Uint32(state.firstEnter),
	}
	for _, score := range scores {
		info.Rooms = append(info.Rooms, &protobuf.GAMEROOM{Roomid: proto.Uint32(score.RoomID), MaxScore: proto.Uint32(score.MaxScore)})
	}
	return info, nil
}

// resourceAmount is how much of a player_resource the commander has.
func (tx *Tx) resourceAmount(resourceID uint32) (uint32, error) {
	resources, err := tx.ListResources(tx.ctx, tx.commanderID)
	if err != nil {
		return 0, err
	}
	for _, row := range resources {
		if row.ResourceID == resourceID {
			return row.Amount, nil
		}
	}
	return 0, nil
}

// addGameCoins gives up to n coins, as many as fit under game_coin_max (the
// client adds as many: the rest is lost).
func (tx *Tx) addGameCoins(n uint32) error {
	owned, err := tx.resourceAmount(resourceGameCoin)
	if err != nil {
		return err
	}
	return tx.AddResource(resourceGameCoin, min(n, gameRoomSettings.coinMax-min(owned, gameRoomSettings.coinMax)))
}

// handleGameRoomFirstCoin answers GameRoomFirstCoinCommand
// (controller/command/gameroom/gameroomfirstcoincommand.lua), sent when the
// Arcade opens while SC_26120.first_enter is 0: on result 0 the client
// marks the visit and adds game_coin_initial coins itself, fewer when that
// would go past game_coin_max; the server gives as many, once. No official
// capture has it: the reply is built from the Lua alone.
func (game *Game) handleGameRoomFirstCoin(client *connection.Client, req *protobuf.CS_26128) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		set, err := tx.SetGameRoomFirstEnter(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if set == 0 {
			return errRefused
		}
		return tx.addGameCoins(gameRoomSettings.coinInitial)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26129{Result: result(err)})
}

// handleGameRoomWeekCoin answers GameRoomWeekCoinCommand
// (controller/command/gameroom/gameroomweekcoincommand.lua), the Arcade's
// free coins button, shown while SC_26120.weekly_free is 0: on result 0 the
// client marks them taken and adds coins up to game_coin_max. It adds
// game_coin_initial; the server gives game_coin_week (both 10). Once a
// week, from Monday 00:00 server time (wiki: Arcade, "Every week", the
// rates resetting "every week with the server reset"; Missions, the weekly
// reset is "Monday midnight (local server time)").
// No official capture has it: the reply is built from the Lua alone.
func (game *Game) handleGameRoomWeekCoin(client *connection.Client, req *protobuf.CS_26122) error {
	thisWeek := periodStart(time.Now(), week).Unix()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		set, err := tx.SetGameRoomWeeklyFree(tx.ctx, db.SetGameRoomWeeklyFreeParams{CommanderID: tx.commanderID, WeeklyFreeWeek: thisWeek})
		if err != nil {
			return err
		}
		if set == 0 {
			return errRefused
		}
		return tx.addGameCoins(gameRoomSettings.coinWeek)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26123{Result: result(err)})
}

// gameCoinPrice is the gold price of the n-th coin bought in a week
// (GameHallExchangePanel:getPriceByCount, game_coin_gold).
func gameCoinPrice(n uint32) uint32 {
	for i := len(gameRoomSettings.coinGold) - 1; i >= 0; i-- {
		if gameRoomSettings.coinGold[i][0] < n {
			return gameRoomSettings.coinGold[i][1]
		}
	}
	return 0
}

// handleGameRoomExchangeCoin answers GameRoomExchangeCoinCommand
// (controller/command/gameroom/gameroomexchangecoincommand.lua), the
// Arcade's coin exchange (GameHallExchangePanel): times coins for gold, each
// priced by how many were bought this week (game_coin_gold), never past
// game_coin_max. On result 0 the client adds the coins (no more than fit),
// counts them in pay_coin_count and takes off the gold the panel showed;
// the server charges the same. The count starts again every week (the
// wiki's Arcade page: "These conversion rates reset every week with the
// server reset"). No official capture has it: the reply is built from the
// Lua alone.
func (game *Game) handleGameRoomExchangeCoin(client *connection.Client, req *protobuf.CS_26124) error {
	now := time.Now()
	thisWeek := periodStart(now, week).Unix()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := loadGameRoomState(tx.ctx, tx.Queries, tx.commanderID, now)
		if err != nil {
			return err
		}
		owned, err := tx.resourceAmount(resourceGameCoin)
		if err != nil {
			return err
		}
		times := req.GetTimes()
		if times == 0 || owned+times > gameRoomSettings.coinMax {
			return errRefused
		}
		gold := uint32(0)
		for n := range times {
			gold += gameCoinPrice(state.payCoinCount + n + 1)
		}
		if err := tx.SpendResource(ResourceGold, gold); err != nil {
			return err
		}
		if err := tx.AddResource(resourceGameCoin, times); err != nil {
			return err
		}
		return tx.AddGameRoomPayCoins(tx.ctx, db.AddGameRoomPayCoinsParams{CommanderID: tx.commanderID, PayWeek: thisWeek, PayCoinCount: times})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26125{Result: result(err)})
}

// gameRoomTickets is what a game of room scoring score earns per coin.
func (room gameRoomConfig) gameRoomTickets(score uint32) uint32 {
	multiplier := 0.0
	for _, step := range room.AddNum {
		if float64(score) >= step[0] && step[0] >= 0 {
			multiplier = max(multiplier, step[1])
		}
	}
	return uint32(math.Round(float64(room.AddBase) * multiplier))
}

// handleGameRoomSuccess answers GameRoomSuccessCommand
// (controller/command/gameroom/gameroomsuccesscommand.lua), sent when an
// Arcade game ends (MiniHubMediator:OnMiniGameSuccess) after the player
// put times coins in (GameRoomCoinLayer, at most coin_max and the coins
// owned; never 0, nor once the month's or the holding's ticket cap is
// reached). On result 0 the client keeps the best score, takes the coins
// off and adds drop_list (its first entry is the tickets); it shows fewer
// when past game_ticket_month or game_room_remax, so the server gives no
// more: add_base times the score's add_num multiplier per coin (the games'
// help text), capped by both. The ticket drop's shape (a resource drop of
// add_type) is a guess. No official capture has it: the reply is built
// from the Lua alone.
func (game *Game) handleGameRoomSuccess(client *connection.Client, req *protobuf.CS_26126) error {
	now := time.Now()
	var drops []*protobuf.DROPINFO
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		room, ok := gameRooms[req.GetRoomid()]
		times := req.GetTimes()
		if !ok || now.Before(room.unlock) || times == 0 || times > room.CoinMax {
			return errRefused
		}
		state, err := loadGameRoomState(tx.ctx, tx.Queries, tx.commanderID, now)
		if err != nil {
			return err
		}
		owned, err := tx.resourceAmount(room.AddType)
		if err != nil {
			return err
		}
		monthLeft := gameRoomSettings.ticketMonth - min(state.monthlyTicket, gameRoomSettings.ticketMonth)
		holdLeft := gameRoomSettings.ticketMax - min(owned, gameRoomSettings.ticketMax)
		if monthLeft == 0 || holdLeft == 0 {
			return errRefused
		}
		if err := tx.SpendResource(resourceGameCoin, times); err != nil {
			return err
		}
		if err := tx.SetGameRoomScore(tx.ctx, db.SetGameRoomScoreParams{CommanderID: tx.commanderID, RoomID: req.GetRoomid(), MaxScore: req.GetScore()}); err != nil {
			return err
		}
		tickets := min(room.gameRoomTickets(req.GetScore())*times, monthLeft, holdLeft)
		drops = []*protobuf.DROPINFO{dropInfo(dropTypeResource, room.AddType, tickets)}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		return tx.AddGameRoomMonthlyTickets(tx.ctx, db.AddGameRoomMonthlyTicketsParams{
			CommanderID: tx.commanderID, TicketMonth: startOfMonth(now).Unix(), MonthlyTicket: tickets,
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_26127{Result: result(err), DropList: drops})
}
