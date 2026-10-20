package game

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island's social side (mod/island/command/{friend,trade,agora,signin,
// chat,card}): who may visit (IslandAccessAgency: open flags, white and
// black lists, invite code) and the visitors' log, the plaza
// (Get-Together Island, IslandAgoraAgency) and its themes, the daily
// supplies (IslandSignInAgency), the pearl market (IslandTradegency), the
// chat between the players of an island and the island card
// (IslandCard, IslandSettingsAgency, IslandCardDiyAgency). Built from the
// Lua and the island_* tables alone: no capture enters the island, and the
// wiki (Island Planner) says nothing of these rules.

// IslandConst.OPEN_*: PB_ISLAND_PUBLIC.flag_list, the island's open flags.
// OPEN_ALL is "visits enabled"; OPEN_FRIEND and OPEN_GUILD are set when
// friends / guild mates are NOT let in (IslandFriendSettingPage turns its
// friend and guild toggles on by removing them).
const (
	islandOpenAll    = 1
	islandOpenFriend = 2
	islandOpenGuild  = 3
	islandOpenCode   = 4
	islandOpenSignIn = 5
)

// IslandConst.DL_*: the keys of PB_ISLAND_PRIVATE.daily_list, all social,
// started over every day (BaseIsland.UpdatePerDay).
const (
	islandDailyFetch      = 1 // took a supply on its own island
	islandDailyOtherFetch = 2 // supplies taken on other islands
	islandDailySignIn     = 3 // signed in
	islandDailyInviteCode = 4 // refreshed its invite code
)

// IslandConst.VISITOR_LOG_CMD_*: what a PB_VISITOR says happened.
const (
	islandVisitorEnter = 1
	islandVisitorExit  = 2
)

// EnterIslandCommand's results (SC_21203) that have their own tip.
const (
	islandVisitRefused = 1  // island_visit_tip1 "Cannot visit island."
	islandVisitBadCode = 9  // island_visit_tip3 "Your visit code is invalid."
	islandVisitClosed  = 20 // island_visit_tip2 "The island does not permit visitors."
)

// IslandConst.ACCESS_OP_*: CS_21302.cmd.
const (
	islandAccessSetWhite     = 1
	islandAccessSetBlack     = 2
	islandAccessKick         = 3
	islandAccessKickAndBlack = 4
)

// IslandSettingsAgency.FLAG_TYPES: PB_SET_FLAG types of the card.
const (
	islandFlagShowSocial = 1
	islandFlagShowLabel  = 2
)

// islandVisitorLogSize is how many PB_VISITOR an island keeps. GUESS: the
// client lists any number.
const islandVisitorLogSize = 50

// islandSocial is the social state only the server knows, in
// islandState.extra["social"].
type islandSocial struct {
	// the card (SC_21327): photo, greeting, achievement groups shown
	Picture string   `json:"picture,omitempty"`
	Word    string   `json:"word,omitempty"`
	Groups  []uint32 `json:"groups,omitempty"`
	// counted on the card: visits, likes and rating tags received
	Visits uint32            `json:"visits,omitempty"`
	Likes  uint32            `json:"likes,omitempty"`
	Labels map[uint32]uint32 `json:"labels,omitempty"`
	// who liked today / tagged this week: commander -> when (unix)
	LikedBy   map[uint32]int64 `json:"liked_by,omitempty"`
	LabeledBy map[uint32]int64 `json:"labeled_by,omitempty"`
	// the custom plaza themes (PB_PLACEMENT_THEME, marshaled) by slot
	Themes map[uint32][]byte `json:"themes,omitempty"`
	// the pearl market's week the purchases count in (unix)
	TradeWeek int64 `json:"trade_week,omitempty"`
}

const islandSocialKey = "social"

// social is the island's social state.
func (state *islandState) social() (*islandSocial, error) {
	social := &islandSocial{}
	if _, err := state.getExtra(islandSocialKey, social); err != nil {
		return nil, err
	}
	return social, nil
}

// setSocial stores the island's social state.
func (state *islandState) setSocial(social *islandSocial) error {
	return state.setExtra(islandSocialKey, social)
}

// daily is a daily_list value.
func (state *islandState) daily(key uint32) uint32 {
	for _, entry := range state.private().GetDailyList() {
		if entry.GetKey() == key {
			return entry.GetValue()
		}
	}
	return 0
}

// setDaily sets a daily_list value.
func (state *islandState) setDaily(key, value uint32) {
	private := state.private()
	for _, entry := range private.DailyList {
		if entry.GetKey() == key {
			entry.Value = proto.Uint32(value)
			return
		}
	}
	private.DailyList = append(private.DailyList, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(value)})
}

// saveIsland stores an island changed by the request: the commander's own
// is saved when the request commits, another's (a supply taken from it, a
// like, a visit in its log) right away, in the request's transaction.
func (tx *Tx) saveIsland(state *islandState) error {
	if state.commanderID == tx.commanderID {
		return nil
	}
	return saveIsland(tx.ctx, tx.Queries, state)
}

// islandSocialConfig is what the social side reads from the tables.
var islandSocialConfig struct {
	whiteListMax  uint32
	giftCount     uint32 // supplies dropped by a sign-in (daily_gift_drop_num)
	giftFetchMax  uint32 // supplies taken on other islands a day (daily_gift_get_max)
	giftItem      uint32 // GUESS: the island item a supply holds (daily_gift)
	agoraCost     [][3]uint32
	themeMax      uint32
	labelIDs      map[uint32]islandCardLabelConfig
	photos        map[uint32]islandCardDiyConfig
	defaultPhoto  uint32
	cardAchvMax   uint32
	achievements  map[uint32]islandAchievementCardConfig
	abilities     map[uint32]islandAbilitySocialConfig
	tradeBuy      uint32    // purchase price, % of the selling price
	tradeLimit    [2]uint32 // pearls bought a week, sold to another island a day
	tradeInitial  [2]uint32 // a day's selling price range
	tradePriceCap [2]uint32
}

type islandCardLabelConfig struct {
	ID uint32 `json:"id"`
}

type islandCardDiyConfig struct {
	ID uint32 `json:"id"`
}

// islandAchievementCardConfig is an island_achievement row as the card
// reads it: its group and stage.
type islandAchievementCardConfig struct {
	Group uint32 `json:"group"`
	Stage uint32 `json:"stage"`
}

// islandAbilitySocialConfig is an island_ability_template row: type 21
// (IslandAblityAgency.TYPE_SIGN_GIFT_CNT) adds effect supplies a day.
type islandAbilitySocialConfig struct {
	Type   uint32          `json:"type"`
	Effect json.RawMessage `json:"effect"`
}

const islandAbilitySignGift = 21

func loadIslandSocialData() error {
	config := &islandSocialConfig
	err := gamedata.LoadAll(
		gamedata.Load("island_card_label", &config.labelIDs),
		gamedata.Load("island_card_diy", &config.photos),
		gamedata.Load("island_achievement", &config.achievements),
		gamedata.Load("island_ability_template", &config.abilities),
	)
	if err != nil {
		return err
	}
	config.whiteListMax = islandSetInt("whit_list_max_cnt")
	config.giftCount = islandSetInt("daily_gift_drop_num")
	config.giftFetchMax = islandSetInt("daily_gift_get_max")
	config.giftItem = islandSetInt("daily_gift")
	config.themeMax = islandSetInt("build_self_theme_num")
	config.defaultPhoto = islandSetInt("island_card_photo_default")
	config.cardAchvMax = islandSetInt("island_card_achievement_limit")
	config.tradeBuy = islandSetInt("treasure_price_buy")
	// [[level, [type, id, count], capacity], ...]: the cost of each plaza
	// upgrade from level (IslandAgoraAgency.consumeList)
	var expansion [][]json.RawMessage
	if err := islandSetList("island_build_expansion", &expansion); err != nil {
		return err
	}
	for _, row := range expansion {
		var cost [3]uint32
		if len(row) < 2 || json.Unmarshal(row[1], &cost) != nil {
			return gamedata.Missing("island_set", "island_build_expansion cost")
		}
		config.agoraCost = append(config.agoraCost, cost)
	}
	for key, value := range map[string]*[2]uint32{
		"treasure_week_limit":    &config.tradeLimit,
		"treasure_price_initial": &config.tradeInitial,
		"treasure_price_limit":   &config.tradePriceCap,
	} {
		if err := islandSetList(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (game *Game) registerIslandSocial() {
	packets.Handle(game.handleIslandOpenFlags)
	packets.Handle(game.handleIslandInviteCode)
	packets.Handle(game.handleIslandOpenFlag)
	packets.Handle(game.handleIslandAccessOp)
	packets.Handle(game.handleIslandAgoraUpgrade)
	packets.Handle(game.handleIslandAgoraSave)
	packets.Handle(game.handleIslandAgoraSaveTheme)
	packets.Handle(game.handleIslandAgoraThemes)
	packets.Handle(game.handleIslandChat)
	packets.Handle(game.handleIslandSignIn)
	packets.Handle(game.handleIslandSignInInvite)
	packets.Handle(game.handleIslandGiftTags)
	packets.Handle(game.handleIslandTrade)
	packets.Handle(game.handleIslandTradePrice)
	packets.Handle(game.handleIslandTradeInvite)
	packets.Handle(game.handleIslandCard)
	packets.Handle(game.handleIslandCardPhoto)
	packets.Handle(game.handleIslandCardWord)
	packets.Handle(game.handleIslandSettingFlags)
	packets.Handle(game.handleIslandCardLike)
	packets.Handle(game.handleIslandCardLabel)
	packets.Handle(game.handleIslandCardAchievements)
}

// islandSocialRefresh brings an island's social state to now when it is
// read: a new day (midnight, server time, BaseIsland.UpdatePerDay) starts
// the daily_list over and forgets who was invited to the supplies; a new
// pearl market day (03:00, IslandTradegency.UpdatePerHour, the price
// trend's dates) sets the day's price and forgets the day's sales and
// invitations, a new market week (Monday 03:00) the pearls bought. The card
// flags start shown. GUESS: the official server's first card flags.
func islandSocialRefresh(state *islandState, now time.Time) {
	private, public := state.private(), state.public()
	today := uint32(startOfDay(now).Unix())
	if private.GetDailyTimestamp() < today {
		private.DailyTimestamp = proto.Uint32(uint32(now.Unix()))
		private.DailyList = nil
		public.TreeGiftInvited = nil
	}
	for _, kind := range []uint32{islandFlagShowSocial, islandFlagShowLabel} {
		if !slices.ContainsFunc(private.FlagList, func(flag *protobuf.PB_SET_FLAG) bool { return flag.GetType() == kind }) {
			private.FlagList = append(private.FlagList, &protobuf.PB_SET_FLAG{Type: proto.Uint32(kind), Flag: proto.Uint32(1)})
		}
	}
	refreshIslandTrade(state, now)
}

// islandVisitAllowed is the EnterIslandCommand result (CS_21202) for client
// visiting island owner: 0 lets it in. Its blacklist never lets in
// (island_visit_balcklist_tip), visits disabled (no OPEN_ALL) let nobody in
// (island_visit_set_help: "No guests may visit the island"), else its
// whitelist (island_visit_whitelist_tip: "always accepted"), anyone with
// the code when OPEN_CODE is set, friends unless OPEN_FRIEND, guild mates
// unless OPEN_GUILD, and the friends it invited to its supplies or its
// pearl market. A visit counts on the owner's card and goes in its
// visitors' log, pushed to it when it is online (SC_21216). GUESS: the
// invited friends' way in, and the codes: 1 for a closed door, 20 for
// visits disabled.
func (game *Game) islandVisitAllowed(tx *Tx, client *connection.Client, owner *islandState, byCode bool) (uint32, error) {
	id := client.CommanderID
	public, private := owner.public(), owner.private()
	switch {
	case slices.Contains(private.BlackList, id):
		return islandVisitRefused, nil
	case !slices.Contains(public.FlagList, islandOpenAll):
		return islandVisitClosed, nil
	case byCode && !slices.Contains(public.FlagList, islandOpenCode):
		return islandVisitBadCode, nil
	}
	allowed := byCode || slices.Contains(private.WhiteList, id) ||
		slices.Contains(public.TreeGiftInvited, id) || slices.Contains(public.Treasure.GetInviteList(), id)
	if !allowed && !slices.Contains(public.FlagList, islandOpenFriend) {
		friends, err := tx.CountFriend(tx.ctx, db.CountFriendParams{CommanderID: owner.commanderID, FriendID: id})
		if err != nil {
			return 0, err
		}
		allowed = friends > 0
	}
	if !allowed && !slices.Contains(public.FlagList, islandOpenGuild) {
		mine, err := tx.GetGuildMember(tx.ctx, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		if err == nil {
			theirs, err := tx.GetGuildMember(tx.ctx, owner.commanderID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return 0, err
			}
			allowed = err == nil && theirs.GuildID == mine.GuildID
		}
	}
	if !allowed {
		return islandVisitRefused, nil
	}
	return 0, game.logIslandVisitor(tx, client, owner, islandVisitorEnter)
}

// logIslandVisitor puts a visitor's entry or exit in the owner's log
// (PB_ISLAND_PRIVATE.visitor_history, the newest last) and pushes it to the
// owner if it is online (SC_21216: IslandProxy adds it and shows it). An
// entry counts as a visit on the owner's card.
func (game *Game) logIslandVisitor(tx *Tx, client *connection.Client, owner *islandState, cmd uint32) error {
	commander, err := tx.GetCommander(tx.ctx, client.CommanderID)
	if err != nil {
		return err
	}
	entry := &protobuf.PB_VISITOR{
		Id: proto.Uint32(client.CommanderID), Name: proto.String(commander.Name),
		Time: proto.Uint32(uint32(serverNow().Unix())), Cmd: proto.Uint32(cmd),
	}
	private := owner.private()
	private.VisitorHistory = append(private.VisitorHistory, entry)
	if extra := len(private.VisitorHistory) - islandVisitorLogSize; extra > 0 {
		private.VisitorHistory = slices.Delete(private.VisitorHistory, 0, extra)
	}
	if cmd == islandVisitorEnter {
		social, err := owner.social()
		if err != nil {
			return err
		}
		social.Visits++
		if err := owner.setSocial(social); err != nil {
			return err
		}
	}
	if err := tx.saveIsland(owner); err != nil {
		return err
	}
	game.deliver(client, owner.commanderID, &protobuf.SC_21216{VisitorList: []*protobuf.PB_VISITOR{entry}})
	return nil
}

// handleIslandOpenFlags answers IslandSetAccessAuthorityCommand and
// IslandShareSignInCommand (CS_21002): the open flags to add (open_flag)
// and remove (close_flag), IslandConst.OPEN_* (1..5). The client applies
// them on result 0. Taking OPEN_ALL away doesn't send the visitors home.
func (game *Game) handleIslandOpenFlags(client *connection.Client, req *protobuf.CS_21002) error {
	reply := &protobuf.SC_21003{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		for _, flag := range append(slices.Clone(req.GetOpenFlag()), req.GetCloseFlag()...) {
			if flag < islandOpenAll || flag > islandOpenSignIn {
				reply.Result = result(errRefused)
				return nil
			}
		}
		state, err := tx.island()
		if err != nil {
			return err
		}
		public := state.public()
		public.FlagList = slices.DeleteFunc(public.FlagList, func(flag uint32) bool { return slices.Contains(req.GetCloseFlag(), flag) })
		for _, flag := range req.GetOpenFlag() {
			if !slices.Contains(public.FlagList, flag) {
				public.FlagList = append(public.FlagList, flag)
			}
		}
		reply.Result = result(nil)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandInviteCode answers IslandRefreshInviteCodeCommand (CS_21008):
// a new invite code for visits by code (CS_21202 code). The first one
// (asked for when OPEN_CODE is first set, auto) is free; a refresh is once a
// day (daily_list DL_INVITE_CODE_FLAG, island_inviteCode_refresh), which
// the client marks itself. GUESS: the code's form, 8 letters and digits.
func (game *Game) handleIslandInviteCode(client *connection.Client, req *protobuf.CS_21008) error {
	reply := &protobuf.SC_21009{InviteCode: proto.String("")}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		private := state.private()
		refresh := private.GetInviteCode() != ""
		if refresh && state.daily(islandDailyInviteCode) != 0 {
			reply.Result = result(errRefused)
			return nil
		}
		code, err := newIslandInviteCode(tx)
		if err != nil {
			return err
		}
		private.InviteCode = proto.String(code)
		if refresh {
			state.setDaily(islandDailyInviteCode, 1)
		}
		reply.Result = result(nil)
		reply.InviteCode = proto.String(code)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// newIslandInviteCode is an invite code no island has.
func newIslandInviteCode(tx *Tx) (string, error) {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for {
		code := make([]byte, 8)
		for i := range code {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
			if err != nil {
				return "", err
			}
			code[i] = letters[n.Int64()]
		}
		_, err := tx.GetIslandByInviteCode(tx.ctx, string(code))
		if errors.Is(err, sql.ErrNoRows) {
			return string(code), nil
		}
		if err != nil {
			return "", err
		}
	}
}

// handleIslandOpenFlag answers the older IslandSetAccessAuthorityCommand
// (mod/island/command, no longer registered: CS_21300), which set a single
// access type (IslandConst.ACCESS_TYPE_CLOSE 0 / OPEN 1): kept in
// PB_ISLAND_PRIVATE.open_flag, which the client doesn't read.
func (game *Game) handleIslandOpenFlag(client *connection.Client, req *protobuf.CS_21300) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		state.private().OpenFlag = proto.Uint32(req.GetOpenFlag())
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21301{Result: result(nil)})
}

// handleIslandAccessOp answers IslandAccessOpCommand (CS_21302): set the
// whitelist (cmd 1) or the blacklist (2) whole, at most whit_list_max_cnt
// players each (island_white_list_full / island_black_list_full); kick
// players off the commander's island (3); kick them and add them to the
// blacklist (4). A kicked visitor gets SC_21304 and goes back to its own
// island (island_visit_tip4). The client sends a second request to take
// the players out of the other list.
func (game *Game) handleIslandAccessOp(client *connection.Client, req *protobuf.CS_21302) error {
	ids := req.GetUserIdList()
	reply := &protobuf.SC_21303{}
	var kick []uint32
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		private := state.private()
		limit := int(islandSocialConfig.whiteListMax)
		switch req.GetCmd() {
		case islandAccessSetWhite, islandAccessSetBlack:
			if len(ids) > limit || slices.Contains(ids, client.CommanderID) {
				reply.Result = result(errRefused)
				return nil
			}
			list := slices.Compact(slices.Clone(ids))
			if req.GetCmd() == islandAccessSetWhite {
				private.WhiteList = list
			} else {
				private.BlackList = list
			}
		case islandAccessKick:
			kick = ids
		case islandAccessKickAndBlack:
			kick = ids
			for _, id := range ids {
				if id != client.CommanderID && !slices.Contains(private.BlackList, id) {
					private.BlackList = append(private.BlackList, id)
				}
			}
			if len(private.BlackList) > limit {
				reply.Result = result(errRefused)
				kick = nil
				return errIslandUndo
			}
		default:
			reply.Result = result(errRefused)
			return nil
		}
		reply.Result = result(nil)
		return nil
	})
	if err != nil && !errors.Is(err, errIslandUndo) {
		return err
	}
	game.kickIslandVisitors(client, kick)
	return client.Send(reply)
}

// errIslandUndo rolls a request's changes back after its reply was set.
var errIslandUndo = errors.New("island change undone")

// kickIslandVisitors sends players on the client's island home (SC_21304).
func (game *Game) kickIslandVisitors(client *connection.Client, ids []uint32) {
	if len(ids) == 0 {
		return
	}
	islandID := client.CommanderID
	world := &game.islands
	world.mu.Lock()
	var targets []*connection.Client
	if room := world.islands[islandID]; room != nil {
		for id, presence := range room.players {
			if id != islandID && slices.Contains(ids, id) {
				targets = append(targets, presence.client)
			}
		}
	}
	world.mu.Unlock()
	kick := &protobuf.SC_21304{Timestamp: proto.Uint32(uint32(serverNow().Unix()))}
	for _, target := range targets {
		game.deliverTo(target, kick)
	}
}

// deliverTo pushes msg to another connection.
func (game *Game) deliverTo(target *connection.Client, msg proto.Message) {
	if err := target.Run(func(target *connection.Client) error { return target.Push(msg) }); err != nil {
		logger.WithFields("Island", logger.FieldValue("commander", target.CommanderID), logger.FieldValue("error", err)).Warn("push not delivered")
	}
}
