package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The event mini-games (MiniGameProxy, view/minigame): each mini_game
// belongs to a mini_game_hub, which runs while its act_id activity does
// (MiniGameHubData:CheckInTime). A hub holds the commander's tickets
// (available_cnt), the stages completed with them (used_cnt), whether the
// final reward was claimed (ultimate) and the best score of each of its games
// (maxscores).
//
// Tickets: reborn_times a server day from the day the hub's activity
// started, the missed days kept, as the wiki has it ("1 Minigame Ticket is
// given daily. Missed daily login Tickets will be saved", Recent News/EN/2019;
// two a day in Summer Festival, whose hub has reborn_times 2), and never more
// than the reward_need stages altogether (a guess: the stage list of
// MiniGameTemplateView ends at reward_need).

// MiniGameOPCommand.CMD_*
const (
	miniGameComplete     = 1
	miniGameUltimate     = 2
	miniGameSpecial      = 3
	miniGameHighScore    = 4
	miniGamePlay         = 5
	miniGameTrack        = 100
	miniGameSuccessData  = 101
	miniGameRequestFetch = 1 // CMD_SPECIAL_GAME's args1[2]: load the saved data
	miniGameRequestSave  = 2 // CMD_SPECIAL_GAME's args1[2]: save / wish
	miniGameRequestPray  = 3 // a shrine's offering once the wishes are used up
)

// MiniGameConst.MG_TYPE_*: what CMD_SPECIAL_GAME does with a game's data
// (MiniGameDataCreator)
const (
	miniGameTypeStored       = 2 // elements and key-value lists saved as they are
	miniGameTypeShrine       = 3 // commander's wish: {wishes left, server gold}
	miniGameTypeTower        = 4 // elements and key-value lists, every op
	miniGameTypeShrineShips  = 5 // shipgirls' wish: {wishes left}, the cards drawn
	miniGameShrineShipsCards = 1 // key-value list of the cards: card slot -> card
)

// task_data_template sub types the mini-games count (guessed from the
// tasks' texts): stages completed in a hub (target the hub, "Complete the
// 7th day of the event minigame", "Finish the Shipgirl Game minigame 3
// times") and games played (target the game, "Play the "Castle Clash"
// minigame 3 times").
const (
	TaskSubMiniGameStage = 415
	TaskSubMiniGamePlay  = 417
)

// mini_game row
type miniGameConfig struct {
	ID               uint32          `json:"id"`
	HubID            uint32          `json:"hub_id"`
	Type             uint32          `json:"type"`
	ConfigData       json.RawMessage `json:"config_data"`
	SimpleConfigData json.RawMessage `json:"simple_config_data"`

	// simple_config_data.drop_ids or .drop: what each stage gives, day 1
	// first (the stage list of MiniGameTemplateView and the event pages)
	stages [][]uint32
	// a shrine's config_data: {gold an offering costs, wishes, wishes a day}
	shrineCost, shrineWishes uint32
}

// mini_game_hub row
type miniGameHubConfig struct {
	ID            uint32          `json:"id"`
	ActID         uint32          `json:"act_id"`
	RebornTimes   uint32          `json:"reborn_times"`
	Reward        uint32          `json:"reward"`
	RewardNeed    uint32          `json:"reward_need"`
	RewardDisplay json.RawMessage `json:"reward_display"`

	games []uint32 // its mini_game ids
	// reward_display when it's one drop: what the final reward shows
	ultimate [][]uint32
}

var (
	miniGames   map[uint32]miniGameConfig
	miniGameHub map[uint32]miniGameHubConfig
)

func loadMiniGameData() error {
	err := gamedata.LoadAll(
		gamedata.Load("mini_game", &miniGames),
		gamedata.Load("mini_game_hub", &miniGameHub),
	)
	if err != nil {
		return err
	}
	for id, game := range miniGames {
		var simple struct {
			Drop    [][]uint32 `json:"drop"`
			DropIDs [][]uint32 `json:"drop_ids"`
		}
		if json.Unmarshal(game.SimpleConfigData, &simple) == nil {
			game.stages = simple.DropIDs
			if len(game.stages) == 0 {
				game.stages = simple.Drop
			}
		}
		var shrine []json.RawMessage
		if json.Unmarshal(game.ConfigData, &shrine) == nil && len(shrine) >= 3 {
			json.Unmarshal(shrine[0], &game.shrineCost)
			json.Unmarshal(shrine[2], &game.shrineWishes)
		}
		miniGames[id] = game
		if hub, ok := miniGameHub[game.HubID]; ok {
			hub.games = append(hub.games, id)
			miniGameHub[game.HubID] = hub
		}
	}
	for id, hub := range miniGameHub {
		slices.Sort(hub.games)
		var display []uint32
		if json.Unmarshal(hub.RewardDisplay, &display) == nil && len(display) == 3 {
			hub.ultimate = [][]uint32{display}
		}
		miniGameHub[id] = hub
	}
	return nil
}

// miniGameHubState is a hub as the commander has it.
type miniGameHubState struct {
	config miniGameHubConfig
	row    db.CommanderMinigameHub
	open   bool
	start  time.Time // when its tickets started
}

// tickets are the hub's available_cnt at now.
func (hub miniGameHubState) tickets(now time.Time) uint32 {
	if !hub.open || hub.start.IsZero() || now.Before(hub.start) {
		return 0
	}
	days := uint32(startOfDay(now).Sub(startOfDay(hub.start)).Hours()/24) + 1
	given := min(days*hub.config.RebornTimes, hub.config.RewardNeed)
	if given <= hub.row.UsedCnt {
		return 0
	}
	return given - hub.row.UsedCnt
}

// info is the hub's MINIGAMEHUB, with the best scores of its games.
func (hub miniGameHubState) info(scores map[uint32]db.CommanderMinigameScore, now time.Time) *protobuf.MINIGAMEHUB {
	info := &protobuf.MINIGAMEHUB{
		Id:           proto.Uint32(hub.config.ID),
		AvailableCnt: proto.Uint32(hub.tickets(now)),
		UsedCnt:      proto.Uint32(hub.row.UsedCnt),
		Ultimate:     proto.Uint32(hub.row.Ultimate),
	}
	for _, game := range hub.config.games {
		if score, ok := scores[game]; ok {
			info.Maxscores = append(info.Maxscores, &protobuf.KVDATA2{
				Key: proto.Uint32(game), Value1: proto.Uint32(score.Score), Value2: proto.Uint32(score.Extra),
			})
		}
	}
	return info
}

// miniGameHubOpen tells whether hub runs at now for the commander, and since
// when: its activity at its activity_template time, or at the commander's
// own (a mini-event replayed, a Capsule Rerun: own) from when the commander
// first saw the hub (openedAt).
func miniGameHubOpen(hub miniGameHubConfig, own map[uint32]uint32, openedAt time.Time, now time.Time) (bool, time.Time) {
	if config, ok := openActivity(hub.ActID, now); ok {
		if config.Time.Start.IsZero() {
			return true, openedAt
		}
		return true, config.Time.Start
	}
	if _, ok := own[hub.ActID]; ok {
		return true, openedAt
	}
	return false, time.Time{}
}

// miniGameHubStates are the commander's hubs, every one when all is set,
// else the running ones and the ones the commander played; the running ones
// are stored on the way so their tickets count from now on when their
// activity has no start.
func miniGameHubStates(ctx context.Context, queries *db.Queries, commanderID uint32, only uint32, now time.Time) (map[uint32]miniGameHubState, error) {
	own, err := commanderActivities(ctx, queries, commanderID, now)
	if err != nil {
		return nil, err
	}
	rows, err := queries.ListMiniGameHubs(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	stored := make(map[uint32]db.CommanderMinigameHub, len(rows))
	for _, row := range rows {
		stored[row.HubID] = row
	}
	states := map[uint32]miniGameHubState{}
	for id, config := range miniGameHub {
		if only != 0 && id != only {
			continue
		}
		row, played := stored[id]
		openedAt := now
		if played {
			openedAt = time.Unix(row.OpenedAt, 0)
		}
		open, start := miniGameHubOpen(config, own, openedAt, now)
		if open && !played {
			row = db.CommanderMinigameHub{CommanderID: commanderID, HubID: id, OpenedAt: now.Unix()}
			if err := queries.OpenMiniGameHub(ctx, db.OpenMiniGameHubParams{CommanderID: commanderID, HubID: id, OpenedAt: row.OpenedAt}); err != nil {
				return nil, err
			}
		}
		if open || played {
			states[id] = miniGameHubState{config: config, row: row, open: open, start: start}
		}
	}
	return states, nil
}

// miniGameScores are the commander's best scores, by game.
func miniGameScores(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32]db.CommanderMinigameScore, error) {
	rows, err := queries.ListMiniGameScores(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	scores := make(map[uint32]db.CommanderMinigameScore, len(rows))
	for _, row := range rows {
		scores[row.GameID] = row
	}
	return scores, nil
}

// handleMiniGameHubs answers MiniGameRequestCommand (at login and every
// server day, MiniGameProxy's DayCall): the hubs running, and the ones the
// commander played or scored in, with their tickets and best scores. A new
// commander with no hub running gets none, as captured.
func (game *Game) handleMiniGameHubs(client *connection.Client, req *protobuf.CS_26101) error {
	ctx := context.Background()
	now := time.Now()
	reply := &protobuf.SC_26102{}
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		states, err := miniGameHubStates(ctx, queries, client.CommanderID, 0, now)
		if err != nil {
			return err
		}
		scores, err := miniGameScores(ctx, queries, client.CommanderID)
		if err != nil {
			return err
		}
		for _, id := range slices.Sorted(maps.Keys(states)) {
			reply.Hubs = append(reply.Hubs, states[id].info(scores, now))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleMiniGameOperation answers MiniGameOPCommand: SC_26104 carries the
// hub as it now is and the game's data (CMD_SPECIAL_GAME), which
// MiniGameProxy and MiniGameDataCreator take as they are. Any refused
// operation gets result 1 (the client shows "mini game Error : 1").
//   - CMD_COMPLETE {score, 0, game}: a ticket completes the next stage, which
//     gives that day's drop of the game's stage list (the event pages and the
//     wiki's "Play the minigame for 7 days" tables).
//   - CMD_ULTIMATE: once reward_need stages are done, the final reward:
//     reward_display when the hub has a reward (the reward id itself names
//     nothing the client has; a guess).
//   - CMD_HIGH_SCORE {game, score, second value}: keeps the best score.
//   - CMD_SPECIAL_GAME {game, op, ...}: the game's saved data, by type
//     (MiniGameDataCreator): elements saved and given back (types 2 and 4),
//     the shrines' wishes (types 3 and 5).
//   - CMD_PLAY {game, completed}, CMD_SPECIAL_TRACK, CMD_SUCCESS_DATA: the
//     client's tracking, only acknowledged (CMD_PLAY counts for the "play N
//     times" tasks).
func (game *Game) handleMiniGameOperation(client *connection.Client, req *protobuf.CS_26103) error {
	ctx := context.Background()
	now := time.Now()
	hubID, args := req.GetHubid(), req.GetArgs1()
	config, ok := miniGameHub[hubID]
	reply := &protobuf.SC_26104{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		if !ok {
			return errRefused
		}
		states, err := miniGameHubStates(ctx, tx.Queries, client.CommanderID, hubID, now)
		if err != nil {
			return err
		}
		hub, known := states[hubID]
		if !known {
			hub = miniGameHubState{config: config, row: db.CommanderMinigameHub{CommanderID: client.CommanderID, HubID: hubID}}
		}
		switch req.GetCmd() {
		case miniGameComplete:
			if hub.tickets(now) == 0 {
				return errRefused
			}
			n, err := tx.CompleteMiniGameStage(ctx, db.CompleteMiniGameStageParams{CommanderID: client.CommanderID, HubID: hubID, UsedCnt: hub.row.UsedCnt})
			if err != nil {
				return err
			}
			if n == 0 {
				return errRefused
			}
			stages := miniGameStages(config, args)
			if int(hub.row.UsedCnt) < len(stages) {
				reply.AwardList = configDrops(stages[hub.row.UsedCnt : hub.row.UsedCnt+1])
			}
			hub.row.UsedCnt++
			tx.event(TaskEvent{SubType: TaskSubMiniGameStage, Target: hubID, Count: 1})
		case miniGameUltimate:
			if !hub.open || hub.row.Ultimate != 0 || hub.row.UsedCnt < config.RewardNeed {
				return errRefused
			}
			n, err := tx.ClaimMiniGameUltimate(ctx, db.ClaimMiniGameUltimateParams{CommanderID: client.CommanderID, HubID: hubID})
			if err != nil {
				return err
			}
			if n == 0 {
				return errRefused
			}
			if config.Reward != 0 {
				reply.AwardList = configDrops(config.ultimate)
			}
			hub.row.Ultimate = 1
		case miniGameHighScore:
			if len(args) < 2 || miniGames[args[0]].HubID != hubID {
				return errRefused
			}
			if err := saveMiniGameScore(tx, args[0], args[1], argAt(args, 2)); err != nil {
				return err
			}
		case miniGameSpecial:
			if len(args) < 2 || miniGames[args[0]].HubID != hubID {
				return errRefused
			}
			data, err := miniGameSpecialOp(tx, miniGames[args[0]], hub.open, args, now)
			if err != nil {
				return err
			}
			reply.Data = data
		case miniGamePlay:
			if len(args) > 0 {
				tx.event(TaskEvent{SubType: TaskSubMiniGamePlay, Target: args[0], Count: 1})
			}
		case miniGameTrack, miniGameSuccessData:
		default:
			return errRefused
		}
		if err := tx.grantDrops(reply.AwardList); err != nil {
			return err
		}
		scores, err := miniGameScores(ctx, tx.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		reply.Hub = hub.info(scores, now)
		return nil
	})
	if refused(err) {
		return client.Send(&protobuf.SC_26104{Result: result(err)})
	}
	if err != nil {
		return err
	}
	if reply.Data == nil {
		// MiniGameOPCommand reads data.id: 0 leaves the game's data alone
		reply.Data = &protobuf.MINIGAMEDATA{Id: proto.Uint32(0)}
	}
	return client.Send(reply)
}

// argAt is args[i], 0 past its end.
func argAt(args []uint32, i int) uint32 {
	if i < len(args) {
		return args[i]
	}
	return 0
}

// miniGameStages is the stage list of a CMD_COMPLETE: the game's
// (args1[3], MiniGameOPCommand puts the game there), else the first of the
// hub's games that has one.
func miniGameStages(hub miniGameHubConfig, args []uint32) [][]uint32 {
	if game, ok := miniGames[argAt(args, 2)]; ok && game.HubID == hub.ID && len(game.stages) > 0 {
		return game.stages
	}
	for _, id := range hub.games {
		if stages := miniGames[id].stages; len(stages) > 0 {
			return stages
		}
	}
	return nil
}

// saveMiniGameScore keeps the best of the stored score and this one, as
// MiniGameProxy:UpdataHighScore does, ties going to the higher second value.
func saveMiniGameScore(tx *Tx, gameID, score, extra uint32) error {
	scores, err := miniGameScores(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	if best, ok := scores[gameID]; ok && (best.Score > score || best.Score == score && best.Extra >= extra) {
		return nil
	}
	return tx.SaveMiniGameScore(tx.ctx, db.SaveMiniGameScoreParams{CommanderID: tx.commanderID, GameID: gameID, Score: score, Extra: extra})
}

// miniGameSpecialOp runs a CMD_SPECIAL_GAME {game, op, ...} and returns the
// game's MINIGAMEDATA as MiniGameDataCreator reads it for the game's type:
//   - stored games (type 2): op 1 gives back the saved elements, op 2
//     (BaseMiniGameView:StoreDataToServer) saves args1[3:];
//   - tower climbing (type 4): the same, the data read after every op;
//   - commander's shrine (type 3): op 1 gives {wishes left, gold offered on
//     the server}, op 2 is a wish (a wish and the config_data gold), op 3 an
//     offering without a wish once they're used up, both answering {gold
//     offered on the server};
//   - shipgirls' shrine (type 5): op 1 gives {wishes left} and the cards
//     drawn, op 2 {game, 2, ship, slot, card} draws card into slot.
//
// The wishes come back every server day (config_data's third value, a
// guess). The buff a commander's wish grants is not given: the player's
// buff_list isn't kept yet.
func miniGameSpecialOp(tx *Tx, config miniGameConfig, open bool, args []uint32, now time.Time) (*protobuf.MINIGAMEDATA, error) {
	ctx := tx.ctx
	data, err := tx.GetMiniGameData(ctx, db.GetMiniGameDataParams{CommanderID: tx.commanderID, GameID: config.ID})
	if errors.Is(err, sql.ErrNoRows) {
		data, err = db.CommanderMinigameDatum{CommanderID: tx.commanderID, GameID: config.ID}, nil
	}
	if err != nil {
		return nil, err
	}
	shrine := config.Type == miniGameTypeShrine || config.Type == miniGameTypeShrineShips
	if today := startOfDay(now).Unix(); shrine && data.CountDay != today {
		data.Count, data.CountDay = config.shrineWishes, today
	}
	op := args[1]
	reply := &protobuf.MINIGAMEDATA{Id: proto.Uint32(config.ID)}
	switch config.Type {
	case miniGameTypeStored, miniGameTypeTower:
		if op == miniGameRequestSave {
			data.Datas = slices.Clone(args[2:])
		}
		reply.Datas = data.Datas
	case miniGameTypeShrine:
		switch op {
		case miniGameRequestFetch:
		case miniGameRequestSave, miniGameRequestPray:
			if !open || op == miniGameRequestSave && data.Count == 0 {
				return nil, errRefused
			}
			if err := tx.SpendResource(ResourceGold, config.shrineCost); err != nil {
				return nil, err
			}
			if op == miniGameRequestSave {
				data.Count--
			}
			data.Gold += config.shrineCost
		default:
			return nil, errRefused
		}
		if err := tx.SaveMiniGameData(ctx, db.SaveMiniGameDataParams(data)); err != nil {
			return nil, err
		}
		gold, err := tx.SumMiniGameGold(ctx, config.ID)
		if err != nil {
			return nil, err
		}
		if op == miniGameRequestFetch {
			reply.Datas = []uint32{data.Count}
		}
		reply.Datas = append(reply.Datas, uint32(gold))
		return reply, nil
	case miniGameTypeShrineShips:
		switch op {
		case miniGameRequestFetch:
			reply.Datas = []uint32{data.Count}
		case miniGameRequestSave:
			if !open || data.Count == 0 || len(args) < 5 {
				return nil, errRefused
			}
			if err := tx.SpendResource(ResourceGold, config.shrineCost); err != nil {
				return nil, err
			}
			data.Count--
			err := tx.SaveMiniGameValue(ctx, db.SaveMiniGameValueParams{
				CommanderID: tx.commanderID, GameID: config.ID, ListKey: miniGameShrineShipsCards, Key: args[3], Value: args[4],
			})
			if err != nil {
				return nil, err
			}
		default:
			return nil, errRefused
		}
	}
	if err := tx.SaveMiniGameData(ctx, db.SaveMiniGameDataParams(data)); err != nil {
		return nil, err
	}
	values, err := tx.ListMiniGameValues(ctx, db.ListMiniGameValuesParams{CommanderID: tx.commanderID, GameID: config.ID})
	if err != nil {
		return nil, err
	}
	lists := map[uint32]*protobuf.KEYVALUELIST_P26{}
	if config.Type == miniGameTypeShrineShips {
		// Shrine2022View reads kvpElements[1] even before any card is drawn
		lists[miniGameShrineShipsCards] = &protobuf.KEYVALUELIST_P26{Key: proto.Uint32(miniGameShrineShipsCards)}
	}
	for _, value := range values {
		list, ok := lists[value.ListKey]
		if !ok {
			list = &protobuf.KEYVALUELIST_P26{Key: proto.Uint32(value.ListKey)}
			lists[value.ListKey] = list
		}
		list.ValueList = append(list.ValueList, &protobuf.KEYVALUE_P26{
			Key: proto.Uint32(value.Key), Value: proto.Uint32(value.Value), Value2: proto.Uint32(value.Value2),
		})
	}
	for _, key := range slices.Sorted(maps.Keys(lists)) {
		reply.Date1KeyValueList = append(reply.Date1KeyValueList, lists[key])
	}
	return reply, nil
}

// handleMiniGameFriendRank answers MiniGameFriendRankCommand: the best
// scores at a game of the commander and their friends (the ranking views
// pick the commander's own line out of it, MusicBeatGamePopUI), time_data
// being the score's second value. Who has no score isn't listed.
func (game *Game) handleMiniGameFriendRank(client *connection.Client, req *protobuf.CS_26111) error {
	ctx := context.Background()
	friends, err := game.DB.ListFriends(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	self, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	commanders := map[uint32]db.Commander{self.ID: self}
	for _, friend := range friends {
		commanders[friend.Commander.ID] = friend.Commander
	}
	scores, err := game.DB.ListMiniGameRanking(ctx, db.ListMiniGameRankingParams{
		GameID: req.GetGameid(), CommanderIds: slices.Sorted(maps.Keys(commanders)),
	})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_26112{}
	for _, score := range scores {
		commander := commanders[score.CommanderID]
		reply.Ranks = append(reply.Ranks, &protobuf.FRIENDSCORE{
			Id:       proto.Uint32(commander.ID),
			Name:     proto.String(commander.Name),
			Score:    proto.Uint32(score.Score),
			Display:  game.displayInfo(commander),
			TimeData: proto.Uint32(score.Extra),
		})
	}
	return client.Send(reply)
}
