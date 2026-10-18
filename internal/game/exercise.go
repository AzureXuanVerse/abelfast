package game

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// SeasonInfo (model/vo/seasoninfo.lua): MAX_FIGHTCOUNT, the attempts a
// refill tops up to, and RECOVER_UP_COUNT, what a refill adds.
const (
	exerciseFightCount   = 10
	exerciseRefillCount  = 5
	exerciseRivals       = 4 // TARGETINFO a list holds (capture)
	exerciseRivalReplace = 5 // const.lua MAX_REPLACE_RIVAL_COUNT, per day
	exerciseRankListMax  = 100
)

// task_data_template sub_type 27 (TaskSubExercise) target_id: 1 counts the
// exercises won ("obtain 3 victories in exercises"), 0 all of them
const exerciseTaskWin = 1

// arena_data_rank row: an exercise rank (SeasonInfo.getMilitaryRank). point
// is the score it takes, order (when not 0) the standing it also takes;
// battle_award the merit a win gives (a defeat half), award_list what
// reaching it pays, k_value how much a win moves the score.
type arenaRank struct {
	ID          uint32      `json:"id"`
	Point       uint32      `json:"point"`
	Order       uint32      `json:"order"`
	BattleAward uint32      `json:"battle_award"`
	KValue      uint32      `json:"k_value"`
	AwardList   [][3]uint32 `json:"award_list"`
}

// arenaRanks are the arena_data_rank rows, by id.
var arenaRanks []arenaRank

func loadArenaRanks() error {
	rows, err := gamedata.Table[arenaRank]("arena_data_rank")
	if err != nil {
		return err
	}
	arenaRanks = arenaRanks[:0]
	for _, row := range rows {
		arenaRanks = append(arenaRanks, row)
	}
	slices.SortFunc(arenaRanks, func(a, b arenaRank) int { return int(a.ID) - int(b.ID) })
	if len(arenaRanks) == 0 {
		return gamedata.Missing("arena_data_rank", "ranks")
	}
	return nil
}

// militaryRank is SeasonInfo.getMilitaryRank: the highest rank whose point
// the score reaches (and whose order the standing does, when it has one).
func militaryRank(score, standing uint32) arenaRank {
	for _, row := range slices.Backward(arenaRanks) {
		if row.Point <= score && (row.Order == 0 || standing <= row.Order) {
			return row
		}
	}
	return arenaRanks[0]
}

// exerciseGain is the score a win against a rival scoring rivalScore adds:
// an Elo update with the attacker's rank's k_value (equal scores give half
// of it, the wiki's "base seasonal score gain"; a stronger rival more, a
// weaker one less). A defeat costs nothing.
func exerciseGain(k, score, rivalScore uint32) uint32 {
	expected := 1 / (1 + math.Pow(10, (float64(rivalScore)-float64(score))/400))
	return uint32(math.Round(float64(k) * (1 - expected)))
}

// exerciseShipExp is the exp a ship gets from an exercise (wiki): 1.5 x the
// rival fleet's levels + 171, x2 for the MVP, x1.5 for the flagship (x3 for
// both), halved on a defeat. No table has it, the client adds what it gets.
func exerciseShipExp(rivalLevels uint32, win, flagship, mvp bool) uint32 {
	exp := math.Floor(1.5*float64(rivalLevels)) + 171
	switch {
	case flagship && mvp:
		exp *= 3
	case mvp:
		exp *= 2
	case flagship:
		exp *= 1.5
	}
	if !win {
		exp /= 2
	}
	return uint32(exp)
}

// lastExerciseReset returns the last attempt refill at or before now.
func lastExerciseReset(now time.Time) time.Time {
	last := startOfDay(now)
	for next := nextExerciseReset(last); !next.After(now); next = nextExerciseReset(next) {
		last = next
	}
	return last
}

// loadExercise is the commander's exercise state at now: a new commander
// starts with full attempts (capture), a new season from zero with
// RECOVER_UP_COUNT attempts (wiki); the refills since the last visit are
// added, the day's rival and shop refreshes start over.
func loadExercise(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (db.CommanderExercise, error) {
	season := periodStart(now, exerciseSeasonLength).Unix()
	state, err := queries.GetExercise(ctx, commanderID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		state = db.CommanderExercise{CommanderID: commanderID, Season: season, BestRank: 1, FightCount: exerciseFightCount, RefilledAt: lastExerciseReset(now).Unix()}
	case err != nil:
		return state, err
	case state.Season != season:
		state.Season, state.Score, state.BestRank, state.Rivals = season, 0, 1, nil
		state.FightCount, state.RefilledAt = exerciseRefillCount, lastExerciseReset(now).Unix()
	}
	for next := nextExerciseReset(time.Unix(state.RefilledAt, 0)); !next.After(now); next = nextExerciseReset(next) {
		state.FightCount = min(state.FightCount+exerciseRefillCount, exerciseFightCount)
		state.RefilledAt = next.Unix()
	}
	if today := startOfDay(now).Unix(); state.FlashDay != today {
		state.FlashCount, state.FlashDay = 0, today
	}
	if today := startOfDay(now).Unix(); state.ShopDay != today {
		state.ShopRefreshes, state.ShopDay = 0, today
	}
	return state, nil
}

func saveExercise(ctx context.Context, queries *db.Queries, state db.CommanderExercise) error {
	return queries.SaveExercise(ctx, db.SaveExerciseParams(state))
}

// exerciseStandings is every commander of the server with their season
// score, best first: a commander's place in it is their exercise rank.
type exerciseStandings []db.ListExerciseStandingsRow

func listExerciseStandings(ctx context.Context, queries *db.Queries, now time.Time) (exerciseStandings, error) {
	return queries.ListExerciseStandings(ctx, periodStart(now, exerciseSeasonLength).Unix())
}

// rank is the commander's place, 1 for the best (0 when unknown).
func (standings exerciseStandings) rank(commanderID uint32) uint32 {
	return uint32(slices.IndexFunc(standings, func(row db.ListExerciseStandingsRow) bool { return row.ID == commanderID }) + 1)
}

// defenseFleet is the commander's exercise defense fleet, the ships that
// still exist. Until one is set it is fleet 1 split by team, as the official
// server sets up a new commander's (the starter in the vanguard, the
// prologue's carrier in the main fleet) and as the client sends it back when
// it resets it (UpdateExerciseFleetCommand).
func defenseFleet(ctx context.Context, queries *db.Queries, state db.CommanderExercise) (vanguard, main []db.Ship, err error) {
	set := len(state.Vanguard)+len(state.Main) > 0
	ids := append(slices.Clone(state.Vanguard), state.Main...)
	if !set {
		rows, err := queries.ListFleetShips(ctx, state.CommanderID)
		if err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			if row.FleetID == fleetRegular {
				ids = append(ids, row.ShipID)
			}
		}
	}
	byID, err := shipsByID(ctx, queries, state.CommanderID, ids)
	if err != nil {
		return nil, nil, err
	}
	for i, id := range ids {
		ship, ok := byID[id]
		switch {
		case !ok:
		case set && i < len(state.Vanguard), !set && shipTeam(ship) == teamVanguard:
			vanguard = append(vanguard, ship)
		case set, shipTeam(ship) == teamMain:
			main = append(main, ship)
		}
	}
	return vanguard, main, nil
}

// shipTeam is the ship's TeamType (Ship:getTeamType): its type's team_type.
func shipTeam(ship db.Ship) string {
	return shipTeams[shipTemplates[ship.TemplateID/10*10+1].Type]
}

// shipIDs are the ids of ships.
func shipIDs(ships []db.Ship) []uint32 {
	ids := make([]uint32, len(ships))
	for i, ship := range ships {
		ids[i] = ship.ID
	}
	return ids
}

// targetInfo is a commander as an exercise rival (Rival.New): its standing,
// portrait and defense fleet with gear and skills (RivalShip). ok is false
// when it has no fleet to fight.
func (game *Game) targetInfo(ctx context.Context, queries *db.Queries, standings exerciseStandings, commanderID uint32, now time.Time) (*protobuf.TARGETINFO, uint32, bool, error) {
	commander, err := queries.GetCommander(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	state, err := loadExercise(ctx, queries, commanderID, now)
	if err != nil {
		return nil, 0, false, err
	}
	vanguard, main, err := defenseFleet(ctx, queries, state)
	if err != nil || len(vanguard)+len(main) == 0 {
		return nil, 0, false, err
	}
	equips, err := queries.ListShipEquipmentsByCommander(ctx, commanderID)
	if err != nil {
		return nil, 0, false, err
	}
	skills, err := queries.ListShipSkillsByCommander(ctx, commanderID)
	if err != nil {
		return nil, 0, false, err
	}
	full := func(ship db.Ship) *protobuf.SHIPINFO {
		var shipEquips []db.ShipEquipment
		for _, equip := range equips {
			if equip.ShipID == ship.ID {
				shipEquips = append(shipEquips, db.ShipEquipment(equip))
			}
		}
		var shipSkills []db.ShipSkill
		for _, skill := range skills {
			if skill.ShipID == ship.ID {
				shipSkills = append(shipSkills, db.ShipSkill(skill))
			}
		}
		return fullShipInfo(ship, shipEquips, shipSkills)
	}
	info := &protobuf.TARGETINFO{
		Id:      proto.Uint32(commander.ID),
		Level:   proto.Uint32(commander.Level),
		Name:    proto.String(commander.Name),
		Score:   proto.Uint32(state.Score),
		Rank:    proto.Uint32(standings.rank(commander.ID)),
		Display: game.displayInfo(commander),
	}
	levels := uint32(0)
	for _, ship := range vanguard {
		info.VanguardShipList = append(info.VanguardShipList, full(ship))
		levels += ship.Level
	}
	for _, ship := range main {
		info.MainShipList = append(info.MainShipList, full(ship))
		levels += ship.Level
	}
	return info, levels, true, nil
}

// pickRivals draws the commander's opponents: the commanders standing just
// above them (the captured four were 60 to 180 places higher), then just
// below when there aren't enough, among those with a fleet.
func (game *Game) pickRivals(ctx context.Context, queries *db.Queries, standings exerciseStandings, commanderID uint32, now time.Time) ([]*protobuf.TARGETINFO, error) {
	place := int(standings.rank(commanderID)) - 1
	var pool []uint32
	for i := place - 1; i >= 0 && len(pool) < 2*exerciseRivals; i-- {
		pool = append(pool, standings[i].ID)
	}
	for i := place + 1; i < len(standings) && len(pool) < 2*exerciseRivals; i++ {
		pool = append(pool, standings[i].ID)
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	var rivals []*protobuf.TARGETINFO
	for _, id := range pool {
		if len(rivals) == exerciseRivals {
			break
		}
		info, _, ok, err := game.targetInfo(ctx, queries, standings, id, now)
		if err != nil {
			return nil, err
		}
		if ok {
			rivals = append(rivals, info)
		}
	}
	slices.SortFunc(rivals, func(a, b *protobuf.TARGETINFO) int { return int(a.GetRank()) - int(b.GetRank()) })
	return rivals, nil
}

// rivalList is the commander's current opponents, drawn again when they
// have none (a new commander or season) or when redraw.
func (game *Game) rivalList(ctx context.Context, queries *db.Queries, state *db.CommanderExercise, standings exerciseStandings, redraw bool, now time.Time) ([]*protobuf.TARGETINFO, error) {
	if !redraw && len(state.Rivals) > 0 {
		var rivals []*protobuf.TARGETINFO
		for _, id := range state.Rivals {
			info, _, ok, err := game.targetInfo(ctx, queries, standings, id, now)
			if err != nil {
				return nil, err
			}
			if ok {
				rivals = append(rivals, info)
			}
		}
		if len(rivals) > 0 {
			return rivals, nil
		}
	}
	rivals, err := game.pickRivals(ctx, queries, standings, state.CommanderID, now)
	if err != nil {
		return nil, err
	}
	state.Rivals = make(db.IDList, len(rivals))
	for i, rival := range rivals {
		state.Rivals[i] = rival.GetId()
	}
	return rivals, nil
}

// handleSeasonInfo answers the exercise (PvP) season (GetSeasonInfoCommand):
// the season's score and standing, the attempts left and when they refill,
// the rivals replaced today, the defense fleet and the rivals.
func (game *Game) handleSeasonInfo(client *connection.Client, req *protobuf.CS_18001) error {
	ctx, now := context.Background(), time.Now()
	var reply *protobuf.SC_18002
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		state, err := loadExercise(ctx, queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		standings, err := listExerciseStandings(ctx, queries, now)
		if err != nil {
			return err
		}
		rivals, err := game.rivalList(ctx, queries, &state, standings, false, now)
		if err != nil {
			return err
		}
		vanguard, main, err := defenseFleet(ctx, queries, state)
		if err != nil {
			return err
		}
		reply = &protobuf.SC_18002{
			Score:               proto.Uint32(state.Score),
			Rank:                proto.Uint32(standings.rank(client.CommanderID)),
			FightCount:          proto.Uint32(state.FightCount),
			FightCountResetTime: proto.Uint32(uint32(nextExerciseReset(now).Unix())),
			FlashTargetCount:    proto.Uint32(state.FlashCount),
			VanguardShipIdList:  shipIDs(vanguard),
			MainShipIdList:      shipIDs(main),
			TargetList:          rivals,
		}
		return saveExercise(ctx, queries, state)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleReplaceRivals draws new opponents (ReplaceRivalsCommand), five times
// a day (MAX_REPLACE_RIVAL_COUNT); arena_data_rank's refresh_price is free
// at every rank.
func (game *Game) handleReplaceRivals(client *connection.Client, req *protobuf.CS_18003) error {
	ctx, now := context.Background(), time.Now()
	reply := &protobuf.SC_18004{}
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		state, err := loadExercise(ctx, queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if state.FlashCount >= exerciseRivalReplace {
			return errRefused
		}
		standings, err := listExerciseStandings(ctx, queries, now)
		if err != nil {
			return err
		}
		if reply.TargetList, err = game.rivalList(ctx, queries, &state, standings, true, now); err != nil {
			return err
		}
		state.FlashCount++
		return saveExercise(ctx, queries, state)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleArenaRank answers the exercise leaderboard (GetPowerRankCommand,
// TYPE_MILITARY_RANK): the best commanders of the season in order (the
// client numbers them and works out their rank emblems).
func (game *Game) handleArenaRank(client *connection.Client, req *protobuf.CS_18006) error {
	ctx := context.Background()
	standings, err := listExerciseStandings(ctx, game.DB.Queries, time.Now())
	if err != nil {
		return err
	}
	reply := &protobuf.SC_18007{}
	for _, row := range standings[:min(len(standings), exerciseRankListMax)] {
		commander, err := game.DB.GetCommander(ctx, row.ID)
		if err != nil {
			return err
		}
		reply.ArenaRankLsit = append(reply.ArenaRankLsit, &protobuf.ARENARANK{
			Id:      proto.Uint32(row.ID),
			Level:   proto.Uint32(row.Level),
			Name:    proto.String(row.Name),
			Score:   proto.Uint32(uint32(row.Score)),
			Display: game.displayInfo(commander),
		})
	}
	return client.Send(reply)
}

// handleExerciseFleet sets the defense fleet (UpdateExerciseFleetCommand):
// up to three of the commander's ships of the right team on each side, both
// sides manned (the client sends fleet 1 instead of an empty one).
func (game *Game) handleExerciseFleet(client *connection.Client, req *protobuf.CS_18008) error {
	ctx, now := context.Background(), time.Now()
	vanguard, main := req.GetVanguardShipIdList(), req.GetMainShipIdList()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		if len(vanguard) == 0 || len(main) == 0 || len(vanguard) > teamShipMax || len(main) > teamShipMax {
			return errRefused
		}
		ids := append(slices.Clone(vanguard), main...)
		byID, err := shipsByID(ctx, queries, client.CommanderID, ids)
		if err != nil {
			return err
		}
		if len(byID) != len(ids) {
			return errRefused
		}
		for _, id := range vanguard {
			if shipTeam(byID[id]) != teamVanguard {
				return errRefused
			}
		}
		for _, id := range main {
			if shipTeam(byID[id]) != teamMain {
				return errRefused
			}
		}
		state, err := loadExercise(ctx, queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		state.Vanguard, state.Main = vanguard, main
		return saveExercise(ctx, queries, state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_18009{Result: result(err)})
}

// handleRivalInfo answers a commander's exercise card (GetRivalInfoCommand):
// an id 0 when there is no such commander or they have no fleet (the client
// shows common_get_player_info_erro).
func (game *Game) handleRivalInfo(client *connection.Client, req *protobuf.CS_18104) error {
	ctx, now := context.Background(), time.Now()
	standings, err := listExerciseStandings(ctx, game.DB.Queries, now)
	if err != nil {
		return err
	}
	info, _, ok, err := game.targetInfo(ctx, game.DB.Queries, standings, req.GetId(), now)
	if err != nil {
		return err
	}
	if !ok {
		info = &protobuf.TARGETINFO{Id: proto.Uint32(0), Level: proto.Uint32(0), Name: proto.String(""), Score: proto.Uint32(0), Rank: proto.Uint32(0)}
	}
	return client.Send(&protobuf.SC_18105{Info: info})
}

// beginDuel starts an exercise battle (BattleGateDuel.Entrance): data is the
// rival, one of the commander's current ones, and an attempt must be left.
// battle_cost_template[SYSTEM_DUEL] costs no oil nor morale.
func (game *Game) beginDuel(client *connection.Client, req *protobuf.CS_40001) error {
	ctx := context.Background()
	state, err := loadExercise(ctx, game.DB.Queries, client.CommanderID, time.Now())
	if err != nil {
		return err
	}
	if state.FightCount == 0 || !slices.Contains(state.Rivals, req.GetData()) {
		return client.Send(&protobuf.SC_40002{Result: result(errRefused)})
	}
	return client.Send(&protobuf.SC_40002{Result: result(nil), Key: proto.Uint32(rand.Uint32N(10000))})
}

// finishDuel settles an exercise battle (BattleGateDuel.Exit): an attempt
// is spent; a win (score above C) adds to the season score
// (exerciseGain), a defeat nothing. The merit, battle_award of the rank held
// (half on a defeat) and the award_list of each rank newly reached this
// season, comes as drops. Ships get exerciseShipExp, no affinity nor morale
// change. Then SC_18005 gives the new score, standing and rivals (the result
// screen finds the fought rival among the previous ones).
func (game *Game) finishDuel(client *connection.Client, req *protobuf.CS_40003) error {
	ctx, now := context.Background(), time.Now()
	win := min(req.GetScore(), scoreS) > scoreC
	reply := &protobuf.SC_40004{Result: result(nil)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		queries := tx.Queries
		state, err := loadExercise(ctx, queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		rivalID := req.GetData()
		if state.FightCount == 0 || !slices.Contains(state.Rivals, rivalID) {
			return errRefused
		}
		standings, err := listExerciseStandings(ctx, queries, now)
		if err != nil {
			return err
		}
		rival, rivalLevels, ok, err := game.targetInfo(ctx, queries, standings, rivalID, now)
		if err != nil {
			return err
		}
		if !ok {
			return errRefused
		}
		state.FightCount--
		held := militaryRank(state.Score, standings.rank(client.CommanderID))
		merit := held.BattleAward
		if win {
			state.Score += exerciseGain(held.KValue, state.Score, rival.GetScore())
			tx.event(TaskEvent{SubType: TaskSubExercise, Target: exerciseTaskWin, Count: 1})
		} else {
			merit /= 2
			tx.event(TaskEvent{SubType: TaskSubExercise, Count: 1})
		}
		if err := saveExercise(ctx, queries, state); err != nil {
			return err
		}
		if standings, err = listExerciseStandings(ctx, queries, now); err != nil {
			return err
		}
		reply.DropInfo = append(reply.DropInfo, dropInfo(dropTypeResource, ResourceExploit, merit))
		reached := militaryRank(state.Score, standings.rank(client.CommanderID))
		for _, row := range arenaRanks {
			if row.ID > state.BestRank && row.ID <= reached.ID {
				for _, award := range row.AwardList {
					reply.DropInfo = append(reply.DropInfo, dropInfo(award[0], award[1], award[2]))
				}
			}
		}
		state.BestRank = max(state.BestRank, reached.ID)

		statShips := make([]uint32, len(req.GetStatistics()))
		for i, stat := range req.GetStatistics() {
			statShips[i] = stat.GetShipId()
		}
		byID, err := shipsByID(ctx, queries, client.CommanderID, statShips)
		if err != nil {
			return err
		}
		// the fleet's main ships come first (BayProxy:getSortShipsByFleet),
		// its flagship leading
		mvp, best := uint32(0), uint32(0)
		for _, stat := range req.GetStatistics() {
			if _, ok := byID[stat.GetShipId()]; ok && (mvp == 0 || stat.GetDamageCause() > best) {
				mvp, best = stat.GetShipId(), stat.GetDamageCause()
			}
		}
		reply.Mvp = proto.Uint32(mvp)
		for i, stat := range req.GetStatistics() {
			ship, ok := byID[stat.GetShipId()]
			if !ok {
				continue
			}
			gained := exerciseShipExp(rivalLevels, win, i == 0, ship.ID == mvp)
			reply.ShipExpList = append(reply.ShipExpList, &protobuf.SHIP_EXP{
				ShipId:   proto.Uint32(ship.ID),
				Exp:      proto.Uint32(gained),
				Intimacy: proto.Uint32(maxIntimacy),
			})
			addShipExp(&ship, gained)
			if err := saveShipBattleState(ctx, queries, ship); err != nil {
				return err
			}
		}
		reply.PlayerExp = proto.Uint32(0)
		if err := tx.grantDrops(reply.DropInfo); err != nil {
			return err
		}
		rivals, err := game.rivalList(ctx, queries, &state, standings, true, now)
		if err != nil {
			return err
		}
		tx.push(&protobuf.SC_18005{
			Score:      proto.Uint32(state.Score),
			Rank:       proto.Uint32(standings.rank(client.CommanderID)),
			TargetList: rivals,
		})
		return saveExercise(ctx, queries, state)
	})
	if refused(err) {
		return client.Send(&protobuf.SC_40004{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// arena_data_shop row: the merit shop's commodities (arena_data_shop ids and
// their weight), five picks from the common list, then one from each other
// list, in that order (as the captured shop is); refresh_price is the gems
// each manual refresh of the day costs, as many as it lists.
type arenaShopConfig struct {
	Common       [][2]uint32 `json:"commodity_list_common"`
	List1        [][2]uint32 `json:"commodity_list_1"`
	List2        [][2]uint32 `json:"commodity_list_2"`
	List3        [][2]uint32 `json:"commodity_list_3"`
	List4        [][2]uint32 `json:"commodity_list_4"`
	List5        [][2]uint32 `json:"commodity_list_5"`
	RefreshPrice []uint32    `json:"refresh_price"`
}

// arenaShopID is the arena_data_shop row the merit shop draws from, the one
// the client reads its refresh prices from (MilitaryShopPage,
// RefreshMilitaryShopCommand: pg.arena_data_shop[1]).
const arenaShopID = 1

var arenaShop arenaShopConfig

func loadArenaShop() error {
	shops, err := gamedata.Table[arenaShopConfig]("arena_data_shop")
	if err != nil {
		return err
	}
	shop, ok := shops[arenaShopID]
	if !ok {
		return gamedata.Missing("arena_data_shop", "shop %d", arenaShopID)
	}
	arenaShop = shop
	return nil
}

// arenaShopCommonPicks is how many goods come from commodity_list_common:
// five in the captured shop (ten goods), which no table or Lua says.
const arenaShopCommonPicks = 5

// arenaShopGoods draws the merit shop's goods (each pick by weight, the
// common ones without repeats) for a commander, day and the manual refreshes
// done that day. The draw is seeded by them, so the shop stays the same
// until it refreshes without being stored.
func arenaShopGoods(shop arenaShopConfig, commanderID uint32, day time.Time, refreshes uint32) []uint32 {
	random := rand.New(rand.NewPCG(uint64(commanderID), uint64(day.Unix())+uint64(refreshes)<<40))
	weight := func(entry [2]uint32) uint32 { return entry[1] }
	var goods []uint32
	pool := slices.Clone(shop.Common)
	for range arenaShopCommonPicks {
		i := weightedPick(random.Uint32N, pool, weight)
		if i < 0 {
			break
		}
		goods = append(goods, pool[i][0])
		pool = slices.Delete(pool, i, i+1)
	}
	for _, list := range [][][2]uint32{shop.List1, shop.List2, shop.List3, shop.List4, shop.List5} {
		if i := weightedPick(random.Uint32N, list, weight); i >= 0 {
			goods = append(goods, list[i][0])
		}
	}
	return goods
}

// merit shop goods today: the commander's stock after the day's refreshes
func meritShopGoods(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) ([]uint32, uint32, error) {
	state, err := loadExercise(ctx, queries, commanderID, now)
	if err != nil {
		return nil, 0, err
	}
	return arenaShopGoods(arenaShop, commanderID, nextDay(now), state.ShopRefreshes), state.ShopRefreshes, nil
}

// arenaShopList is the stock's ARENASHOP entries: count the times each was
// bought from it (once at most, Goods.TYPE_MILITARY).
func arenaShopList(ctx context.Context, queries *db.Queries, commanderID uint32, goods []uint32, now time.Time) ([]*protobuf.ARENASHOP, error) {
	purchases, err := shopPurchases(ctx, queries, commanderID, now)
	if err != nil {
		return nil, err
	}
	list := make([]*protobuf.ARENASHOP, len(goods))
	for i, id := range goods {
		list[i] = &protobuf.ARENASHOP{ShopId: proto.Uint32(id), Count: proto.Uint32(purchases[id])}
	}
	return list, nil
}

// handleMilitaryShop answers GetMilitaryShopCommand, the merit shop: today's
// goods, the manual refreshes done today; it refreshes at midnight.
func (game *Game) handleMilitaryShop(client *connection.Client, req *protobuf.CS_18100) error {
	ctx, now := context.Background(), time.Now()
	goods, refreshes, err := meritShopGoods(ctx, game.DB.Queries, client.CommanderID, now)
	if err != nil {
		return err
	}
	list, err := arenaShopList(ctx, game.DB.Queries, client.CommanderID, goods, now)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_18101{
		FlashCount:    proto.Uint32(refreshes),
		ArenaShopList: list,
		NextFlashTime: proto.Uint32(uint32(nextDay(now).Unix())),
	})
}

// handleRefreshMilitaryShop restocks the merit shop for gems
// (RefreshMilitaryShopCommand): the day's nth refresh costs refresh_price[n]
// (MeritorousShop.refreshCount is flash_count + 1), as many a day as it
// lists. The new stock can be bought from again.
func (game *Game) handleRefreshMilitaryShop(client *connection.Client, req *protobuf.CS_18102) error {
	ctx, now := context.Background(), time.Now()
	reply := &protobuf.SC_18103{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := loadExercise(ctx, tx.Queries, client.CommanderID, now)
		if err != nil {
			return err
		}
		if int(state.ShopRefreshes) >= len(arenaShop.RefreshPrice) {
			return errRefused
		}
		if err := tx.SpendShopResource(ResourceGem, arenaShop.RefreshPrice[state.ShopRefreshes]); err != nil {
			return err
		}
		old := arenaShopGoods(arenaShop, client.CommanderID, nextDay(now), state.ShopRefreshes)
		if err := tx.DeleteShopPurchases(ctx, db.DeleteShopPurchasesParams{CommanderID: client.CommanderID, ExpiresAt: nextDay(now).Unix(), ShopIds: old}); err != nil {
			return err
		}
		state.ShopRefreshes++
		if err := saveExercise(ctx, tx.Queries, state); err != nil {
			return err
		}
		goods := arenaShopGoods(arenaShop, client.CommanderID, nextDay(now), state.ShopRefreshes)
		reply.ArenaShopList, err = arenaShopList(ctx, tx.Queries, client.CommanderID, goods, now)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}
