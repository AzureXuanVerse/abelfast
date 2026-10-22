package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// The back-office's view of a commander's activities, shops and archives
// (internal/admin area_life.go). Every write kicks the commander first (it
// keeps its own copy of what it holds) and runs in one transaction.

func lifeRows[T any](rows []T, err error) ([]any, error) {
	if err != nil {
		return nil, err
	}
	list := make([]any, len(rows))
	for i := range rows {
		list[i] = rows[i]
	}
	return list, nil
}

// lifeTable is a table of per-commander activity state: listed whole,
// cleared for one activity id (or whole for the systems kept apart from any
// activity, key ignored).
type lifeTable struct {
	name  string
	list  func(ctx context.Context, q *db.Queries, commanderID uint32) ([]any, error)
	clear func(ctx context.Context, q *db.Queries, commanderID, key uint32) error
}

// lifeActivityTables are the tables keyed by activity_template id
// (activity_id, or act_id), the tasks first.
var lifeActivityTables = []lifeTable{
	{"commander_activity_tasks", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderActivityTasks(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderActivityTasks(ctx, db.LifeDeleteCommanderActivityTasksParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_activities", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderActivities(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderActivities(ctx, db.LifeDeleteCommanderActivitiesParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_activity_key_values", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderActivityKeyValues(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderActivityKeyValues(ctx, db.LifeDeleteCommanderActivityKeyValuesParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_permanent_activities", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderPermanentActivities(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderPermanentActivities(ctx, db.LifeDeleteCommanderPermanentActivitiesParams{CommanderID: c, ActivityID: a})
	}},
	{"activity_fleet_ships", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListActivityFleetShips(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteActivityFleetShips(ctx, db.LifeDeleteActivityFleetShipsParams{CommanderID: c, ActivityID: a})
	}},
	{"activity_fleet_meowfficers", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListActivityFleetMeowfficers(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteActivityFleetMeowfficers(ctx, db.LifeDeleteActivityFleetMeowfficersParams{CommanderID: c, ActivityID: a})
	}},
	{"challenge_runs", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListChallengeRuns(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteChallengeRuns(ctx, db.LifeDeleteChallengeRunsParams{CommanderID: c, ActivityID: a})
	}},
	{"challenge_ships", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListChallengeShips(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteChallengeShips(ctx, db.LifeDeleteChallengeShipsParams{CommanderID: c, ActivityID: a})
	}},
	{"challenge_meowfficers", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListChallengeMeowfficers(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteChallengeMeowfficers(ctx, db.LifeDeleteChallengeMeowfficersParams{CommanderID: c, ActivityID: a})
	}},
	{"challenge_records", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListChallengeRecords(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteChallengeRecords(ctx, db.LifeDeleteChallengeRecordsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_auction", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderAuction(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderAuction(ctx, db.LifeDeleteCommanderAuctionParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_city_rebuild", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderCityRebuild(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderCityRebuild(ctx, db.LifeDeleteCommanderCityRebuildParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_city_rebuild_units", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderCityRebuildUnits(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderCityRebuildUnits(ctx, db.LifeDeleteCommanderCityRebuildUnitsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_city_rebuild_buffs", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderCityRebuildBuffs(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderCityRebuildBuffs(ctx, db.LifeDeleteCommanderCityRebuildBuffsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_coloring_cells", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderColoringCells(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderColoringCells(ctx, db.LifeDeleteCommanderColoringCellsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_coloring_awards", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderColoringAwards(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderColoringAwards(ctx, db.LifeDeleteCommanderColoringAwardsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_feasts", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderFeasts(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderFeasts(ctx, db.LifeDeleteCommanderFeastsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_feast_ships", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderFeastShips(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderFeastShips(ctx, db.LifeDeleteCommanderFeastShipsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_feast_invitations", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderFeastInvitations(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderFeastInvitations(ctx, db.LifeDeleteCommanderFeastInvitationsParams{CommanderID: c, ActivityID: a})
	}},
	{"commander_island_nodes", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderIslandNodes(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteCommanderIslandNodes(ctx, db.LifeDeleteCommanderIslandNodesParams{CommanderID: c, ActivityID: a})
	}},
	{"remaster_awards", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListRemasterAwards(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteRemasterAwards(ctx, db.LifeDeleteRemasterAwardsParams{CommanderID: c, ActivityID: a})
	}},
	{"new_server_shop_goods", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListNewServerShopGoods(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteNewServerShopGoods(ctx, db.LifeDeleteNewServerShopGoodsParams{CommanderID: c, ActivityID: a})
	}},
	{"new_server_shop_sub_goods", func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListNewServerShopSubGoods(ctx, c))
	}, func(ctx context.Context, q *db.Queries, c, a uint32) error {
		return q.LifeDeleteNewServerShopSubGoods(ctx, db.LifeDeleteNewServerShopSubGoodsParams{CommanderID: c, ActivityID: a})
	}},
}

// lifeWhole is a lifeTable of a system kept apart from activity ids.
func lifeWhole[T any](name string, list func(*db.Queries, context.Context, uint32) ([]T, error), clear func(*db.Queries, context.Context, uint32) error) lifeTable {
	return lifeTable{name,
		func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) { return lifeRows(list(q, ctx, c)) },
		func(ctx context.Context, q *db.Queries, c, _ uint32) error { return clear(q, ctx, c) },
	}
}

// lifeSystems are the systems whose state isn't kept by activity id, by
// the key the back-office names them with.
var lifeSystems = map[string][]lifeTable{
	"atelier": {
		lifeWhole("commander_atelier_items", (*db.Queries).LifeListCommanderAtelierItems, (*db.Queries).LifeDeleteCommanderAtelierItems),
		lifeWhole("commander_atelier_recipes", (*db.Queries).LifeListCommanderAtelierRecipes, (*db.Queries).LifeDeleteCommanderAtelierRecipes),
		lifeWhole("commander_atelier_slots", (*db.Queries).LifeListCommanderAtelierSlots, (*db.Queries).LifeDeleteCommanderAtelierSlots),
	},
	"juus": {
		lifeWhole("commander_juus_posts", (*db.Queries).LifeListCommanderJuusPosts, (*db.Queries).LifeDeleteCommanderJuusPosts),
		lifeWhole("commander_juus_topics", (*db.Queries).LifeListCommanderJuusTopics, (*db.Queries).LifeDeleteCommanderJuusTopics),
		lifeWhole("commander_juus_replies", (*db.Queries).LifeListCommanderJuusReplies, (*db.Queries).LifeDeleteCommanderJuusReplies),
		lifeWhole("commander_juus_groups", (*db.Queries).LifeListCommanderJuusGroups, (*db.Queries).LifeDeleteCommanderJuusGroups),
	},
	"apartment": {
		lifeWhole("commander_apartment", (*db.Queries).LifeListCommanderApartment, (*db.Queries).LifeDeleteCommanderApartment),
		lifeWhole("apartment_ships", (*db.Queries).LifeListApartmentShips, (*db.Queries).LifeDeleteApartmentShips),
		lifeWhole("apartment_rooms", (*db.Queries).LifeListApartmentRooms, (*db.Queries).LifeDeleteApartmentRooms),
		lifeWhole("apartment_furniture", (*db.Queries).LifeListApartmentFurniture, (*db.Queries).LifeDeleteApartmentFurniture),
		lifeWhole("apartment_gifts", (*db.Queries).LifeListApartmentGifts, (*db.Queries).LifeDeleteApartmentGifts),
		lifeWhole("apartment_hidden_parts", (*db.Queries).LifeListApartmentHiddenParts, (*db.Queries).LifeDeleteApartmentHiddenParts),
		lifeWhole("commander_apartment_chats", (*db.Queries).LifeListCommanderApartmentChats, (*db.Queries).LifeDeleteCommanderApartmentChats),
		lifeWhole("commander_apartment_events", (*db.Queries).LifeListCommanderApartmentEvents, (*db.Queries).LifeDeleteCommanderApartmentEvents),
		lifeWhole("commander_apartment_ins_contents", (*db.Queries).LifeListCommanderApartmentInsContents, (*db.Queries).LifeDeleteCommanderApartmentInsContents),
		lifeWhole("commander_apartment_ins_replies", (*db.Queries).LifeListCommanderApartmentInsReplies, (*db.Queries).LifeDeleteCommanderApartmentInsReplies),
	},
	"loveletters": {
		lifeWhole("commander_love_letters", (*db.Queries).LifeListCommanderLoveLetters, (*db.Queries).LifeDeleteCommanderLoveLetters),
		lifeWhole("commander_love_letter_medals", (*db.Queries).LifeListCommanderLoveLetterMedals, (*db.Queries).LifeDeleteCommanderLoveLetterMedals),
		lifeWhole("commander_love_letter_gifts", (*db.Queries).LifeListCommanderLoveLetterGifts, (*db.Queries).LifeDeleteCommanderLoveLetterGifts),
		lifeWhole("commander_love_letter_rewards", (*db.Queries).LifeListCommanderLoveLetterRewards, (*db.Queries).LifeDeleteCommanderLoveLetterRewards),
	},
	"educate": {
		lifeWhole("commander_educate", (*db.Queries).LifeListCommanderEducate, (*db.Queries).LifeDeleteCommanderEducate),
	},
	"reflux": {
		lifeWhole("commander_reflux", (*db.Queries).LifeListCommanderReflux, (*db.Queries).LifeDeleteCommanderReflux),
	},
	"remaster": {
		lifeWhole("commander_remasters", (*db.Queries).LifeListCommanderRemasters, (*db.Queries).LifeDeleteCommanderRemasters),
		lifeWhole("remaster_states", (*db.Queries).LifeListRemasterStates, (*db.Queries).LifeDeleteRemasterStates),
	},
	"minigames": {
		lifeWhole("commander_minigame_hubs", (*db.Queries).LifeListCommanderMinigameHubs, (*db.Queries).LifeDeleteCommanderMinigameHubs),
		lifeWhole("commander_minigame_data", (*db.Queries).LifeListCommanderMinigameData, (*db.Queries).LifeDeleteCommanderMinigameData),
		lifeWhole("commander_minigame_scores", (*db.Queries).LifeListCommanderMinigameScores, (*db.Queries).LifeDeleteCommanderMinigameScores),
		lifeWhole("commander_minigame_values", (*db.Queries).LifeListCommanderMinigameValues, (*db.Queries).LifeDeleteCommanderMinigameValues),
	},
	"arcade": {
		lifeWhole("commander_game_room", (*db.Queries).LifeListCommanderGameRoom, (*db.Queries).LifeDeleteCommanderGameRoom),
		lifeWhole("commander_game_room_scores", (*db.Queries).LifeListCommanderGameRoomScores, (*db.Queries).LifeDeleteCommanderGameRoomScores),
	},
	"playroom": {
		lifeWhole("commander_play_room_views", (*db.Queries).LifeListCommanderPlayRoomViews, (*db.Queries).LifeDeleteCommanderPlayRoomViews),
		lifeWhole("commander_play_room_scores", (*db.Queries).LifeListCommanderPlayRoomScores, (*db.Queries).LifeDeleteCommanderPlayRoomScores),
	},
	"island": {
		lifeWhole("commander_islands", (*db.Queries).LifeListCommanderIslands, (*db.Queries).LifeDeleteCommanderIslands),
	},
}

// lifeNewEducateTable is the new Project Identity, one state per character.
var lifeNewEducateTable = lifeTable{"commander_new_educate",
	func(ctx context.Context, q *db.Queries, c uint32) ([]any, error) {
		return lifeRows(q.LifeListCommanderNewEducate(ctx, c))
	},
	func(ctx context.Context, q *db.Queries, c, character uint32) error {
		return q.LifeDeleteCommanderNewEducate(ctx, db.LifeDeleteCommanderNewEducateParams{CommanderID: c, CharacterID: character})
	},
}

// The states of AdminState.Status.
const (
	AdminStateRunning   = "running"
	AdminStatePermanent = "permanent"
	AdminStateFinished  = "finished"
)

// AdminStateTable is a table's rows of one state (db model values).
type AdminStateTable struct {
	Table string
	Rows  []any
}

// AdminState is what a commander keeps of one activity or system: Key is
// "activity:<id>", "neweducate:<character>" or a lifeSystems key.
type AdminState struct {
	Key        string
	Activity   uint32 // the activity_template id, 0 for a system
	Type       uint32 // its type (ActivityConst)
	Character  uint32 // neweducate: the child2_data id
	Status     string
	Start, End int64 // the activity's window, 0 when it has none
	Tables     []AdminStateTable
}

// lifeField is a db row's uint32 field of that name, false without one.
func lifeField(row any, names ...string) (uint32, bool) {
	value := reflect.ValueOf(row)
	for _, name := range names {
		if field := value.FieldByName(name); field.IsValid() && field.Kind() == reflect.Uint32 {
			return uint32(field.Uint()), true
		}
	}
	return 0, false
}

func lifeUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// AdminStates lists every activity and system the commander keeps state of.
func (game *Game) AdminStates(ctx context.Context, commanderID uint32) ([]AdminState, error) {
	now := time.Now()
	q := game.DB.Queries
	byActivity := map[uint32]*AdminState{}
	for _, table := range lifeActivityTables {
		rows, err := table.list(ctx, q, commanderID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			id, _ := lifeField(row, "ActivityID", "ActID")
			state := byActivity[id]
			if state == nil {
				state = &AdminState{Key: "activity:" + strconv.FormatUint(uint64(id), 10), Activity: id}
				byActivity[id] = state
			}
			if n := len(state.Tables); n == 0 || state.Tables[n-1].Table != table.name {
				state.Tables = append(state.Tables, AdminStateTable{Table: table.name})
			}
			last := &state.Tables[len(state.Tables)-1]
			last.Rows = append(last.Rows, row)
		}
	}
	own, err := commanderActivities(ctx, q, commanderID, now)
	if err != nil {
		return nil, err
	}
	var states []AdminState
	for _, id := range slices.Sorted(maps.Keys(byActivity)) {
		state := byActivity[id]
		config, known := activityConfigs[id]
		if known {
			state.Type = config.Type
			state.Start, state.End = lifeUnix(config.Time.Start), lifeUnix(config.Time.End)
		}
		_, running := own[id]
		_, open := openActivity(id, now)
		permanent := slices.ContainsFunc(state.Tables, func(t AdminStateTable) bool { return t.Table == "commander_permanent_activities" })
		switch {
		case open || running:
			state.Status = AdminStateRunning
		case permanent || (known && config.Time.Always):
			state.Status = AdminStatePermanent
		default:
			state.Status = AdminStateFinished
		}
		states = append(states, *state)
	}
	for _, key := range slices.Sorted(maps.Keys(lifeSystems)) {
		state := AdminState{Key: key, Status: AdminStatePermanent}
		for _, table := range lifeSystems[key] {
			rows, err := table.list(ctx, q, commanderID)
			if err != nil {
				return nil, err
			}
			if len(rows) > 0 {
				state.Tables = append(state.Tables, AdminStateTable{Table: table.name, Rows: rows})
			}
		}
		if len(state.Tables) > 0 {
			states = append(states, state)
		}
	}
	children, err := lifeNewEducateTable.list(ctx, q, commanderID)
	if err != nil {
		return nil, err
	}
	for _, row := range children {
		character, _ := lifeField(row, "CharacterID")
		states = append(states, AdminState{
			Key: "neweducate:" + strconv.FormatUint(uint64(character), 10), Character: character, Status: AdminStatePermanent,
			Tables: []AdminStateTable{{Table: lifeNewEducateTable.name, Rows: []any{row}}},
		})
	}
	return states, nil
}

// AdminClearState deletes the commander's rows of one AdminState key, or
// only its task progress (commander_activity_tasks) when tasksOnly, in one
// transaction once the commander is kicked. Missing rows are no error: the
// game starts each activity again from no rows (newActivityState).
func (game *Game) AdminClearState(ctx context.Context, commanderID uint32, key string, tasksOnly bool) (bool, error) {
	kind, arg, _ := strings.Cut(key, ":")
	id, _ := strconv.ParseUint(arg, 10, 32)
	var tables []lifeTable
	switch {
	case kind == "activity" && id != 0 && tasksOnly:
		tables = lifeActivityTables[:1]
	case kind == "activity" && id != 0:
		tables = lifeActivityTables
	case kind == "neweducate" && id != 0 && !tasksOnly:
		tables = []lifeTable{lifeNewEducateTable}
	case lifeSystems[kind] != nil && arg == "" && !tasksOnly:
		tables = lifeSystems[kind]
	default:
		return false, adminRefused("state %q: unknown", key)
	}
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		for _, table := range tables {
			if err := table.clear(ctx, q, commanderID, uint32(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- shops

// The shops AdminShops lists, AdminShop.Kind.
const (
	AdminShopStreet    = "street"    // shop_template shopping_street, the general shop
	AdminShopArena     = "arena"     // shop_template arena_shop, the merit shop
	AdminShopOil       = "oil"       // shop_template buy_oil
	AdminShopPacks     = "packs"     // shop_template gift_package / new_server_shop and the groups
	AdminShopOther     = "other"     // every other shop_template purchase
	AdminShopMedal     = "medal"     // honormedal_goods_list, the medal exchange
	AdminShopCore      = "core"      // the Core Data exchange (month_shop_template)
	AdminShopPrototype = "prototype" // the Prototype shop (month_shop_template)
	AdminShopGuild     = "guild"     // guild_store
	AdminShopPrize     = "prize"     // gameroom_shop_template, the Arcade's Prize Shop
	AdminShopNewServer = "newserver" // newserver_shop_template / blackfriday_shop_template
)

// AdminGoods is one goods of a shop: ID is what resets it (the shop_template
// id, group id, table row, guild shop slot), Type/Item/Num what a purchase
// gives (Effect for a shop_template extension), Price* what it costs.
type AdminGoods struct {
	ID        uint32
	Group     bool // a shop_template group
	Type      uint32
	Item      uint32
	Num       uint32
	Effect    string
	PriceType uint32 // a drop type: 1 resource, 2 item
	PriceID   uint32
	Price     uint32
	Bought    uint32
	Limit     uint32 // 0 no limit
	Never     bool   // the purchases never reset (the Prototype shop's limited goods)
}

// AdminShop is what the commander bought in one shop.
type AdminShop struct {
	Kind      string
	Activity  uint32 // newserver: the activity
	ActType   uint32
	ResetAt   int64 // when the purchases count again from 0, 0 never
	Refreshes uint32
	InGuild   bool
	Goods     []AdminGoods
}

func lifeShopDrop(goods shopGoods) (kind, item uint32) {
	switch {
	case goods.Type == 0:
		return 0, 0
	case goods.Type == shopTypeSkin:
		return dropTypeSkin, goods.target
	}
	return goods.Type, goods.target
}

func lifeShopKind(genre string) string {
	switch genre {
	case genreShoppingStreet:
		return AdminShopStreet
	case genreArenaShop:
		return AdminShopArena
	case genreBuyOil:
		return AdminShopOil
	case genreGiftPackage, genreNewServerShop:
		return AdminShopPacks
	}
	return AdminShopOther
}

// AdminShops lists the shops' purchases that count today: every
// shop_template purchase still counted, and the stock of the medal
// exchange, the monthly shops, the guild shop, the Prize Shop and the new
// commander shops.
func (game *Game) AdminShops(ctx context.Context, commanderID uint32) ([]AdminShop, error) {
	now := time.Now()
	q := game.DB.Queries
	shops := map[string]*AdminShop{}
	order := []string{AdminShopStreet, AdminShopArena, AdminShopOil, AdminShopPacks, AdminShopOther}
	for _, kind := range order {
		shops[kind] = &AdminShop{Kind: kind}
	}
	shops[AdminShopStreet].ResetAt = nextExerciseReset(now).Unix()
	shops[AdminShopArena].ResetAt = nextDay(now).Unix()
	shops[AdminShopOil].ResetAt = nextDay(now).Unix()
	purchases, err := q.LifeListShopPurchases(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	bought := map[uint32]uint32{}
	for _, row := range purchases {
		if row.ExpiresAt == 0 || row.ExpiresAt > now.Unix() {
			bought[row.ShopID] += row.Count
		}
	}
	for _, id := range slices.Sorted(maps.Keys(bought)) {
		goods := shopTemplate[id]
		kind, item := lifeShopDrop(goods)
		shop := shops[lifeShopKind(goods.Genre)]
		shop.Goods = append(shop.Goods, AdminGoods{
			ID: id, Type: kind, Item: item, Num: uint32(max(goods.Num, 0)), Effect: goods.effect,
			PriceType: dropTypeResource, PriceID: goods.ResourceType, Price: uint32(max(goods.ResourceNum, 0)),
			Bought: bought[id], Limit: goods.times,
		})
	}
	groups, err := q.LifeListShopGroupPurchases(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	groupBought := map[uint32]uint32{}
	for _, row := range groups {
		if row.ExpiresAt == 0 || row.ExpiresAt > now.Unix() {
			groupBought[row.GroupID] += row.Count
		}
	}
	for _, group := range slices.Sorted(maps.Keys(groupBought)) {
		// named after the group's first goods
		var head shopGoods
		for _, id := range slices.Sorted(maps.Keys(shopTemplate)) {
			if shopTemplate[id].Group == group {
				head = shopTemplate[id]
				break
			}
		}
		kind, item := lifeShopDrop(head)
		shops[AdminShopPacks].Goods = append(shops[AdminShopPacks].Goods, AdminGoods{
			ID: group, Group: true, Type: kind, Item: item, Num: uint32(max(head.Num, 0)),
			Bought: groupBought[group], Limit: head.GroupLimit,
		})
	}
	list := make([]AdminShop, 0, len(order)+6)
	for _, kind := range order {
		list = append(list, *shops[kind])
	}

	// the medal exchange, this month's stock
	next := nextMonth(now)
	medalBought, err := medalShopPurchases(ctx, q, commanderID, next)
	if err != nil {
		return nil, err
	}
	medal := AdminShop{Kind: AdminShopMedal, ResetAt: next.Unix()}
	for _, id := range medalShopGoods(commanderID, next) {
		goods := medalGoods[id]
		medal.Goods = append(medal.Goods, AdminGoods{
			ID: id, Type: goods.Type, Item: lifeFirst(goods.Goods), Num: goods.Num,
			PriceType: dropTypeItem, PriceID: itemHonorMedal, Price: goods.Price,
			Bought: medalBought[id], Limit: goods.PurchaseLimit,
		})
	}
	list = append(list, medal)

	// the monthly shops
	monthRows, err := q.ListMonthShopPurchases(ctx, db.ListMonthShopPurchasesParams{CommanderID: commanderID, Month: monthKey(now)})
	if err != nil {
		return nil, err
	}
	monthBought := map[uint32]uint32{} // the limited goods' purchases are month 0, the others this month's
	for _, row := range monthRows {
		monthBought[row.GoodsID] += row.Count
	}
	month := monthShops[uint32(now.In(gamedata.Zone()).Month())]
	monthGoods := func(id, key uint32) AdminGoods {
		goods := activityShop[id]
		priceType := uint32(dropTypeResource)
		if goods.ResourceCategory == dropTypeItem {
			priceType = dropTypeItem
		}
		return AdminGoods{
			ID: id, Type: goods.CommodityType, Item: goods.CommodityID, Num: goods.Num,
			PriceType: priceType, PriceID: goods.ResourceType, Price: goods.ResourceNum,
			Bought: monthBought[id], Limit: goods.NumLimit, Never: key == 0,
		}
	}
	core := AdminShop{Kind: AdminShopCore, ResetAt: next.Unix()}
	for _, id := range month.Core {
		core.Goods = append(core.Goods, monthGoods(id, monthKey(now)))
	}
	prototype := AdminShop{Kind: AdminShopPrototype, ResetAt: next.Unix()}
	for _, id := range month.Blueprint {
		prototype.Goods = append(prototype.Goods, monthGoods(id, monthKey(now)))
	}
	for _, id := range month.BlueprintLimit {
		prototype.Goods = append(prototype.Goods, monthGoods(id, 0))
	}
	list = append(list, core, prototype)

	// the guild shop: its stock as stored, or the next one
	_, inGuild, err := guildMembership(ctx, q, commanderID)
	if err != nil {
		return nil, err
	}
	guild := AdminShop{Kind: AdminShopGuild, InGuild: inGuild, ResetAt: nextGuildShopRefresh(now).Unix()}
	if shop, err := q.GetGuildShop(ctx, commanderID); err == nil {
		guild.ResetAt, guild.Refreshes = shop.NextRefreshTime, shop.RefreshCount
		slots, err := q.ListGuildShopPurchases(ctx, commanderID)
		if err != nil {
			return nil, err
		}
		slotBought := map[uint32]uint32{}
		for _, row := range slots {
			slotBought[row.Slot] = row.Count
		}
		for i, id := range guildShopGoods(commanderID, shop.NextRefreshTime, shop.RefreshCount) {
			goods := guildGoods[id]
			guild.Goods = append(guild.Goods, AdminGoods{
				ID: uint32(i + 1), Type: goods.Type, Item: lifeFirst(goods.Goods), Num: goods.Num,
				PriceType: dropTypeResource, PriceID: ResourceGuildCoin, Price: goods.Price,
				Bought: slotBought[uint32(i+1)], Limit: goods.PurchaseLimit,
			})
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	list = append(list, guild)

	// the Prize Shop
	prizeBought, err := miniGameShopPurchases(ctx, q, commanderID, now)
	if err != nil {
		return nil, err
	}
	prize := AdminShop{Kind: AdminShopPrize, ResetAt: next.Unix()}
	for _, id := range slices.Sorted(maps.Keys(miniGameGoods)) {
		goods := miniGameGoods[id]
		if !goods.Time.Always && !goods.Time.Start.IsZero() && (now.Before(goods.Time.Start) || !now.Before(goods.Time.End)) && prizeBought[id] == 0 {
			continue
		}
		prize.Goods = append(prize.Goods, AdminGoods{
			ID: id, Type: goods.DropType, Item: lifeFirst(goods.Goods), Num: goods.Num,
			PriceType: dropTypeResource, PriceID: resourceGameTicket, Price: goods.Price,
			Bought: prizeBought[id], Limit: goods.PurchaseLimit, Never: goods.MonthRe == 0,
		})
	}
	list = append(list, prize)

	// the new commander and Black Friday shops bought from, or running
	newServer, err := q.LifeListNewServerShopGoods(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	activities := map[uint32]map[uint32]uint32{}
	for _, row := range newServer {
		if activities[row.ActivityID] == nil {
			activities[row.ActivityID] = map[uint32]uint32{}
		}
		activities[row.ActivityID][row.GoodsID] = row.Bought
	}
	own, err := newUserActivities(ctx, q, commanderID, now)
	if err != nil {
		return nil, err
	}
	for id, config := range activityConfigs {
		if config.Type != activityTypeNewServerShop && config.Type != activityTypeBlackFridayShop || activities[id] != nil {
			continue
		}
		_, open := openActivity(id, now)
		if _, mine := own[id]; open || mine {
			activities[id] = map[uint32]uint32{}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(activities)) {
		config := activityConfigs[id]
		shop := AdminShop{Kind: AdminShopNewServer, Activity: id, ActType: config.Type}
		if _, stop, err := shopWindow(ctx, q, commanderID, config); err != nil {
			return nil, err
		} else {
			shop.ResetAt = lifeUnix(stop)
		}
		var ids []uint32
		json.Unmarshal(config.ConfigData, &ids)
		for goodsID := range activities[id] {
			if !slices.Contains(ids, goodsID) {
				ids = append(ids, goodsID)
			}
		}
		table := newServerShopGoods[config.Type]
		for _, goodsID := range ids {
			goods := table[goodsID]
			priceType := uint32(dropTypeResource)
			if goods.ResourceCategory == dropTypeItem {
				priceType = dropTypeItem
			}
			shop.Goods = append(shop.Goods, AdminGoods{
				ID: goodsID, Type: goods.Type, Item: lifeFirst(goods.Goods), Num: goods.Num,
				PriceType: priceType, PriceID: goods.ResourceType, Price: goods.ResourceNum - min(goods.Discount, goods.ResourceNum),
				Bought: activities[id][goodsID], Limit: goods.GoodsPurchaseLimit,
			})
		}
		list = append(list, shop)
	}
	return list, nil
}

func lifeFirst(ids []uint32) uint32 {
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

// AdminResetShop forgets what the commander bought in a shop: one goods
// (its AdminGoods.ID, with group for a shop_template group), or all of the
// shop's when id is 0. The guild shop's refreshes go back to 0 with refreshes
// (its stock is drawn again, as when it refreshes).
func (game *Game) AdminResetShop(ctx context.Context, commanderID uint32, kind string, activity, id uint32, group, refreshes bool) (bool, error) {
	shops, err := game.AdminShops(ctx, commanderID)
	if err != nil {
		return false, err
	}
	index := slices.IndexFunc(shops, func(shop AdminShop) bool {
		return shop.Kind == kind && (kind != AdminShopNewServer || shop.Activity == activity)
	})
	if index < 0 {
		return false, adminRefused("shop %s: unknown", kind)
	}
	shop := shops[index]
	goods := shop.Goods
	if id != 0 {
		goods = slices.DeleteFunc(slices.Clone(goods), func(g AdminGoods) bool { return g.ID != id || g.Group != group })
		if len(goods) == 0 {
			return false, adminRefused("goods %d: unknown", id)
		}
	}
	now := time.Now()
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		if kind == AdminShopGuild && refreshes {
			row, err := q.GetGuildShop(ctx, commanderID)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			} else if err != nil {
				return err
			}
			row.RefreshCount = 0
			return newTx(ctx, q, commanderID).resetGuildShop(row)
		}
		for _, g := range goods {
			var err error
			switch kind {
			case AdminShopStreet, AdminShopArena, AdminShopOil, AdminShopPacks, AdminShopOther:
				if g.Group {
					err = q.LifeDeleteShopGroupPurchases(ctx, db.LifeDeleteShopGroupPurchasesParams{CommanderID: commanderID, GroupID: g.ID})
				} else {
					err = q.LifeDeleteShopPurchases(ctx, db.LifeDeleteShopPurchasesParams{CommanderID: commanderID, ShopID: g.ID})
				}
			case AdminShopMedal:
				err = q.LifeDeleteMedalShopPurchase(ctx, db.LifeDeleteMedalShopPurchaseParams{CommanderID: commanderID, GoodsID: g.ID, FlashTime: shop.ResetAt})
			case AdminShopCore, AdminShopPrototype:
				month := monthKey(now)
				if g.Never {
					month = 0
				}
				err = q.LifeDeleteMonthShopPurchase(ctx, db.LifeDeleteMonthShopPurchaseParams{CommanderID: commanderID, GoodsID: g.ID, Month: month})
			case AdminShopGuild:
				err = q.LifeDeleteGuildShopPurchase(ctx, db.LifeDeleteGuildShopPurchaseParams{CommanderID: commanderID, Slot: g.ID})
			case AdminShopPrize:
				err = q.LifeDeleteMiniGameShopPurchases(ctx, db.LifeDeleteMiniGameShopPurchasesParams{CommanderID: commanderID, GoodsID: g.ID})
			case AdminShopNewServer:
				err = q.LifeDeleteNewServerShopGoodsItem(ctx, db.LifeDeleteNewServerShopGoodsItemParams{CommanderID: commanderID, ActivityID: activity, GoodsID: g.ID})
				if err == nil {
					err = q.LifeDeleteNewServerShopSubGoodsItem(ctx, db.LifeDeleteNewServerShopSubGoodsItemParams{CommanderID: commanderID, ActivityID: activity, GoodsID: g.ID})
				}
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// AdminRequisitions is how many ships were requisitioned this month, the
// monthly limit and when the count resets.
func (game *Game) AdminRequisitions(ctx context.Context, commanderID uint32) (count, limit uint32, resetAt int64, err error) {
	next := nextMonth(time.Now())
	count, err = game.DB.GetRequisitionCount(ctx, db.GetRequisitionCountParams{CommanderID: commanderID, Month: next.Unix()})
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return count, requisitionMonthly, next.Unix(), err
}

// AdminResetRequisitions forgets this month's requisitions.
func (game *Game) AdminResetRequisitions(ctx context.Context, commanderID uint32) (bool, error) {
	next := nextMonth(time.Now())
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		return q.LifeDeleteRequisitions(ctx, db.LifeDeleteRequisitionsParams{CommanderID: commanderID, Month: next.Unix()})
	})
}

// AdminAkashi is Akashi's affection (charge_exp): the exp, the taps counted
// today and the last tap (unix, 0 never). The taps of an earlier day are 0.
func (game *Game) AdminAkashi(ctx context.Context, commanderID uint32) (db.ChargeExp, uint32, error) {
	row, err := game.DB.GetChargeExp(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.ChargeExp{CommanderID: commanderID}, mingShiDailyTaps, nil
	}
	if !sameServerDay(time.Unix(row.TappedAt, 0), time.Now()) {
		row.Taps = 0
	}
	return row, mingShiDailyTaps, err
}

// AdminSetAkashi sets Akashi's affection exp and today's taps (at most the
// daily taps); reset forgets the row.
func (game *Game) AdminSetAkashi(ctx context.Context, commanderID, exp, taps uint32, reset bool) (bool, error) {
	if taps > mingShiDailyTaps {
		return false, adminRefused("taps: at most %d a day: refused", mingShiDailyTaps)
	}
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		if reset {
			return q.LifeDeleteChargeExp(ctx, commanderID)
		}
		row, err := q.GetChargeExp(ctx, commanderID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		tapped := row.TappedAt
		if taps > 0 && !sameServerDay(time.Unix(tapped, 0), time.Now()) {
			// the taps count on the day of tapped_at
			tapped = time.Now().Unix()
		}
		return q.SaveChargeExp(ctx, db.SaveChargeExpParams{CommanderID: commanderID, Exp: exp, Taps: taps, TappedAt: tapped})
	})
}

// --- archives

// AdminShipGroup is an archive entry with its stars as the client counts
// them (archiveStar).
type AdminShipGroup struct {
	db.CommanderShipGroup
	Stars    uint32
	Template uint32 // the group's base template
}

// AdminShipGroups lists the commander's archive entries.
func (game *Game) AdminShipGroups(ctx context.Context, commanderID uint32) ([]AdminShipGroup, error) {
	groups, err := game.DB.ListShipGroups(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	list := make([]AdminShipGroup, 0, len(groups))
	for _, group := range groups {
		list = append(list, AdminShipGroup{CommanderShipGroup: group, Stars: archiveStar(group), Template: group.GroupID*10 + 1})
	}
	return list, nil
}

// AdminMedal is a medal (its first tier) with its tiers.
type AdminMedal struct {
	ID       uint32
	Tiers    []uint32 // the tiers' ids, in order
	Targets  []uint32 // each tier's target_num
	Progress uint32   // the progress of the tier shown (the first not claimed)
	Claimed  int      // tiers claimed
	Hidden   bool
}

// AdminMedals lists the medals the commander has progress or claims on.
func (game *Game) AdminMedals(ctx context.Context, commanderID uint32) ([]AdminMedal, error) {
	state, err := loadTrophyState(ctx, game.DB.Queries, commanderID)
	if err != nil {
		return nil, err
	}
	var list []AdminMedal
	for _, id := range archiveTrophies {
		if medalTemplates[id].Start != 0 {
			continue
		}
		medal := AdminMedal{ID: id, Hidden: medalTemplates[id].Hide == medalAlwaysHidden}
		shown := uint32(0)
		for tier := id; tier != 0; tier = medalTemplates[tier].Next {
			config, ok := medalTemplates[tier]
			if !ok || slices.Contains(medal.Tiers, tier) {
				break
			}
			if config.TargetType == medalTargetComingSoon {
				break
			}
			medal.Tiers = append(medal.Tiers, tier)
			medal.Targets = append(medal.Targets, config.TargetNum)
			if state.claimed[tier] != 0 {
				medal.Claimed++
			} else if shown == 0 {
				shown = tier
			}
		}
		if len(medal.Tiers) == 0 {
			continue
		}
		if shown == 0 {
			shown = medal.Tiers[len(medal.Tiers)-1]
		}
		medal.Progress = state.info(shown).GetProgress()
		if medal.Progress == 0 && medal.Claimed == 0 {
			continue
		}
		list = append(list, medal)
	}
	return list, nil
}

// AdminResetMedal forgets the tiers claimed of a medal (its first tier id)
// and of their sub-medals, and takes them off the profile. The progress is
// kept: it counts what the commander did.
func (game *Game) AdminResetMedal(ctx context.Context, commanderID, id uint32) (bool, error) {
	if config, ok := medalTemplates[id]; !ok || config.Start != 0 {
		return false, adminRefused("medal %d: unknown", id)
	}
	var ids []uint32
	for tier := id; tier != 0; tier = medalTemplates[tier].Next {
		if _, ok := medalTemplates[tier]; !ok || slices.Contains(ids, tier) {
			break
		}
		ids = append(ids, tier)
		ids = append(ids, trophySubMedals(tier)...)
	}
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		for _, tier := range ids {
			if err := q.LifeDeleteTrophyClaim(ctx, db.LifeDeleteTrophyClaimParams{CommanderID: commanderID, TrophyID: tier}); err != nil {
				return err
			}
			if err := q.LifeDeleteDisplayMedal(ctx, db.LifeDeleteDisplayMedalParams{CommanderID: commanderID, MedalID: tier}); err != nil {
				return err
			}
		}
		return nil
	})
}

// The archive lists AdminResetSeen clears.
const (
	AdminSeenStories = "stories"
	AdminSeenFlags   = "flags"
	AdminSeenGallery = "gallery"
	AdminSeenMusic   = "music"
	AdminSeenManga   = "manga"
)

// AdminSeen counts what the commander saw and unlocked: the stories, the
// client's one-time flags, the gallery pictures, songs and manga chapters
// (unlocked or read, then liked).
func (game *Game) AdminSeen(ctx context.Context, commanderID uint32) (map[string][2]int64, error) {
	q := game.DB.Queries
	seen := map[string][2]int64{}
	stories, err := q.LifeCountStories(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	seen[AdminSeenStories] = [2]int64{stories}
	flags, err := q.ListCommonFlags(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	seen[AdminSeenFlags] = [2]int64{int64(len(flags))}
	rows, err := q.ListAppreciation(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	kinds := map[uint32][2]any{
		appreciationGalleryUnlocked: {AdminSeenGallery, 0}, appreciationGalleryLiked: {AdminSeenGallery, 1},
		appreciationMusicUnlocked: {AdminSeenMusic, 0}, appreciationMusicLiked: {AdminSeenMusic, 1},
		appreciationMangaRead: {AdminSeenManga, 0}, appreciationMangaLiked: {AdminSeenManga, 1},
	}
	for _, key := range []string{AdminSeenGallery, AdminSeenMusic, AdminSeenManga} {
		seen[key] = [2]int64{}
	}
	for _, row := range rows {
		kind, ok := kinds[row.Kind]
		if !ok {
			continue
		}
		counts := seen[kind[0].(string)]
		counts[kind[1].(int)]++
		seen[kind[0].(string)] = counts
	}
	return seen, nil
}

// AdminResetSeen clears one AdminSeen list; the music also stops the main
// scene's player (its album may no longer be unlocked).
func (game *Game) AdminResetSeen(ctx context.Context, commanderID uint32, what string) (bool, error) {
	kinds := map[string][]uint32{
		AdminSeenGallery: {appreciationGalleryUnlocked, appreciationGalleryLiked},
		AdminSeenMusic:   {appreciationMusicUnlocked, appreciationMusicLiked},
		AdminSeenManga:   {appreciationMangaRead, appreciationMangaLiked},
	}
	if what != AdminSeenStories && what != AdminSeenFlags && kinds[what] == nil {
		return false, adminRefused("%q: unknown", what)
	}
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		switch what {
		case AdminSeenStories:
			return q.LifeDeleteStories(ctx, commanderID)
		case AdminSeenFlags:
			return q.LifeDeleteCommonFlags(ctx, commanderID)
		}
		for _, kind := range kinds[what] {
			if err := q.LifeDeleteAppreciation(ctx, db.LifeDeleteAppreciationParams{CommanderID: commanderID, Kind: kind}); err != nil {
				return err
			}
		}
		if what == AdminSeenMusic {
			return q.LifeDeleteMusicPlayer(ctx, commanderID)
		}
		return nil
	})
}

// AdminLook is what the commander shows: its secretaries (ship ids, in
// order), the random secretary (mode 0 unset, enabled), the oath ship shown
// (a ship id, 0 none), the special secretary (secretary_special_ship, 0
// none), the icon and chat frames and the port wallpaper (0 the defaults).
type AdminLook struct {
	Secretaries     []uint32
	RandomMode      uint32
	RandomEnabled   bool
	MarryShip       uint32
	ChildDisplay    uint32
	IconFrame       uint32
	ChatFrame       uint32
	Cover           uint32
	MaxSecretaries  int
	SpecialOptions  []uint32 // the secretary_special_ship ids
	IconFrames      []uint32 // owned (0 always)
	ChatFrames      []uint32
	Covers          []uint32
	MusicNo, MusicM uint32
}

// AdminGetLook reads the commander's AdminLook with what it may pick from.
func (game *Game) AdminGetLook(ctx context.Context, commanderID uint32) (AdminLook, error) {
	q := game.DB.Queries
	var look AdminLook
	commander, err := q.GetCommander(ctx, commanderID)
	if err != nil {
		return look, err
	}
	look.MarryShip, look.ChildDisplay = commander.MarryShip, commander.ChildDisplay
	secretaries, err := q.ListSecretaries(ctx, commanderID)
	if err != nil {
		return look, err
	}
	for _, secretary := range secretaries {
		look.Secretaries = append(look.Secretaries, secretary.ShipID)
	}
	random, err := q.LifeGetRandomFlagship(ctx, commanderID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return look, err
	}
	look.RandomMode, look.RandomEnabled = random.Mode, random.Enabled
	shown, err := appearance(ctx, q, commanderID)
	if err != nil {
		return look, err
	}
	look.IconFrame, look.ChatFrame, look.Cover = shown.IconFrame, shown.ChatFrame, shown.Cover
	if look.MaxSecretaries, err = maxSecretaries(); err != nil {
		return look, err
	}
	look.SpecialOptions = slices.Sorted(maps.Keys(educateSecretaries))
	look.IconFrames, look.ChatFrames, look.Covers = []uint32{defaultAppearance}, []uint32{defaultAppearance}, []uint32{defaultAppearance}
	frames, err := q.ListAttireFrames(ctx, commanderID)
	if err != nil {
		return look, err
	}
	for _, frame := range frames {
		switch frame.DropType {
		case dropTypeIconFrame:
			look.IconFrames = append(look.IconFrames, frame.FrameID)
		case dropTypeChatFrame:
			look.ChatFrames = append(look.ChatFrames, frame.FrameID)
		case dropTypeCover:
			look.Covers = append(look.Covers, frame.FrameID)
		}
	}
	player, err := q.GetMusicPlayer(ctx, commanderID)
	if err == nil {
		look.MusicNo, look.MusicM = player.MusicNo, player.MusicMode
	} else if !errors.Is(err, sql.ErrNoRows) {
		return look, err
	}
	return look, nil
}

// AdminSetLook saves the commander's look, checked as the game checks each
// part when the client changes it: the secretaries are 1 to maxSecretaries
// distinct ships of its own (CS_11011, shown as themselves, no skin
// shadow), the random mode one of the panel's, the oath ship an oathed ship
// of its own (CS_12032), the special secretary a secretary_special_ship
// (CS_27041), the frames and wallpaper ones it owns (CS_11005, CS_11030).
func (game *Game) AdminSetLook(ctx context.Context, commanderID uint32, look AdminLook) (bool, error) {
	limit, err := maxSecretaries()
	if err != nil {
		return false, err
	}
	if len(look.Secretaries) == 0 || len(look.Secretaries) > limit {
		return false, adminRefused("secretaries: 1 to %d: refused", limit)
	}
	if look.RandomMode > randomShipCustom {
		return false, adminRefused("random mode %d: refused", look.RandomMode)
	}
	if _, ok := educateSecretaries[look.ChildDisplay]; !ok && look.ChildDisplay != 0 {
		return false, adminRefused("special secretary %d: refused", look.ChildDisplay)
	}
	known := map[uint32]map[uint32]struct{}{dropTypeIconFrame: iconFrames, dropTypeChatFrame: chatFrames, dropTypeCover: covers}
	parts := [][2]uint32{{dropTypeIconFrame, look.IconFrame}, {dropTypeChatFrame, look.ChatFrame}, {dropTypeCover, look.Cover}}
	return game.EditCommanderRows(ctx, commanderID, func(q *db.Queries) error {
		seen := map[uint32]bool{}
		for _, ship := range look.Secretaries {
			if seen[ship] {
				return adminRefused("secretary %d twice: refused", ship)
			}
			seen[ship] = true
			if _, err := q.GetShip(ctx, db.GetShipParams{ID: ship, CommanderID: commanderID}); errors.Is(err, sql.ErrNoRows) {
				return adminRefused("ship %d: refused", ship)
			} else if err != nil {
				return err
			}
		}
		if look.MarryShip != 0 {
			ship, err := q.GetShip(ctx, db.GetShipParams{ID: look.MarryShip, CommanderID: commanderID})
			if errors.Is(err, sql.ErrNoRows) || (err == nil && ship.Propose == 0) {
				return adminRefused("oath ship %d: refused", look.MarryShip)
			} else if err != nil {
				return err
			}
		}
		for _, part := range parts {
			if _, ok := known[part[0]][part[1]]; !ok && part[1] != defaultAppearance {
				return adminRefused("appearance %d: refused", part[1])
			}
			if owned, err := ownsAttire(ctx, q, commanderID, part[0], part[1]); err != nil {
				return err
			} else if !owned {
				return adminRefused("appearance %d not owned: refused", part[1])
			}
		}
		if err := q.DeleteSecretaries(ctx, commanderID); err != nil {
			return err
		}
		for i, ship := range look.Secretaries {
			if err := q.SetSecretary(ctx, db.SetSecretaryParams{CommanderID: commanderID, Position: uint32(i + 1), ShipID: ship}); err != nil {
				return err
			}
		}
		if err := q.SetRandomFlagshipMode(ctx, db.SetRandomFlagshipModeParams{CommanderID: commanderID, Mode: look.RandomMode}); err != nil {
			return err
		}
		if err := q.SetRandomFlagshipEnabled(ctx, db.SetRandomFlagshipEnabledParams{CommanderID: commanderID, Enabled: look.RandomEnabled}); err != nil {
			return err
		}
		if err := q.SetMarryShip(ctx, db.SetMarryShipParams{MarryShip: look.MarryShip, ID: commanderID}); err != nil {
			return err
		}
		if err := q.SetChildDisplay(ctx, db.SetChildDisplayParams{ChildDisplay: look.ChildDisplay, ID: commanderID}); err != nil {
			return err
		}
		if err := q.SetIconFrame(ctx, db.SetIconFrameParams{CommanderID: commanderID, IconFrame: look.IconFrame}); err != nil {
			return err
		}
		if err := q.SetChatFrame(ctx, db.SetChatFrameParams{CommanderID: commanderID, ChatFrame: look.ChatFrame}); err != nil {
			return err
		}
		return q.SetCover(ctx, db.SetCoverParams{CommanderID: commanderID, Cover: look.Cover})
	})
}

// AdminHandbookPage is a commander manual page: its tasks finished and the
// pt rewards claimed, of the totals.
type AdminHandbookPage struct {
	ID                  uint32
	Done, Tasks         int
	Awards, AwardsTotal uint32
}

// AdminHandbook lists the commander's manual pages.
func (game *Game) AdminHandbook(ctx context.Context, commanderID uint32) ([]AdminHandbookPage, error) {
	pages, err := game.DB.ListHandbookPages(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	finished, err := game.DB.ListHandbookFinished(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	var list []AdminHandbookPage
	for _, page := range pages {
		config := handbookPageConfigs[page.PageID]
		row := AdminHandbookPage{ID: page.PageID, Awards: page.Award, AwardsTotal: uint32(len(config.Target))}
		for _, pair := range config.TaskList {
			if len(pair) > 0 {
				row.Tasks += len(pair[0])
			}
		}
		for _, done := range finished {
			if done.PageID == page.PageID {
				row.Done++
			}
		}
		list = append(list, row)
	}
	return list, nil
}
