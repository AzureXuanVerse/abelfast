package game

import (
	"context"
	"database/sql"
	"errors"
	"maps"
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

// Challenge mode (view/challenge/, ChallengeProxy): an activity of type
// ActivityConst.ACTIVITY_TYPE_CHALLENGE runs weekly seasons of bosses fought
// one after the other with the fleets a run started with, their hp carried
// from battle to battle. No capture has it (the last one ended in 2022): the
// replies are built from the Lua alone, the score from the client tables.
const activityTypeChallenge = 37

// const.lua SYSTEM_CHALLENGE: a challenge mode battle (BattleGateChallenge)
const systemChallenge = 6

// ChallengeProxy.MODE_CASUAL and MODE_INFINITE
const (
	challengeCasual   = 0
	challengeInfinite = 1
)

// ChallengeConst.BOSS_NUM: the bosses of a round (a casual run is one round)
const challengeBossNum = 5

// Fleet.SUBMARINE_FLEET_ID: a mode's fleets are mode+1 and mode+11
// (Challenge2InitialRequestCommand)
const submarineFleetID = 11

// challenge_rate_template type 1: the share of a boss' points kept, by
// ships sunk in the battle
const challengeRateSunk = 1

// activity_event_challenge row (the activity's config_id): infinite_stage
// lists, per season, the rounds of expedition_challenge_template ids; buff
// is the season's buff_list.
type challengeConfig struct {
	InfiniteStage [][][]uint32 `json:"infinite_stage"`
	Buff          []uint32     `json:"buff"`
}

// expedition_challenge_template row: dungeon_id is the stage fought
// (UserChallengeInfo.getNextStageID, CS_40001 data), challenge the boss'
// points.
type challengeExpedition struct {
	DungeonID uint32 `json:"dungeon_id"`
	Challenge uint32 `json:"challenge"`
}

type challengeRate struct {
	Type    uint32  `json:"type"`
	Content uint32  `json:"content"`
	Rate    float64 `json:"rate"`
}

var (
	challengeConfigs     map[uint32]challengeConfig
	challengeExpeditions map[uint32]challengeExpedition
	challengeSunkRates   map[uint32]float64 // ships sunk → rate
)

func loadChallengeData() error {
	var rates map[uint32]challengeRate
	err := gamedata.LoadAll(
		gamedata.Load("activity_event_challenge", &challengeConfigs),
		gamedata.Load("expedition_challenge_template", &challengeExpeditions),
		gamedata.Load("challenge_rate_template", &rates),
	)
	if err != nil {
		return err
	}
	challengeSunkRates = map[uint32]float64{}
	for _, rate := range rates {
		if rate.Type == challengeRateSunk {
			challengeSunkRates[rate.Content] = rate.Rate
		}
	}
	return nil
}

// openChallenge is the challenge activity running at now with its config,
// the one ActivityProxy:getActivityByType(ACTIVITY_TYPE_CHALLENGE) finds.
func openChallenge(now time.Time) (activityConfig, challengeConfig, bool) {
	for _, activity := range openActivities(now) {
		if activity.Type != activityTypeChallenge {
			continue
		}
		config, ok := challengeConfigs[activity.ConfigID]
		if ok && len(config.InfiniteStage) > 0 {
			return activity, config, true
		}
	}
	return activityConfig{}, challengeConfig{}, false
}

// challengeSeason is the season (1-based infinite_stage index) running at
// now: a new one every Monday 00:00 (ChallengeMainScene.updateTimePanel
// counts down to TimeMgr:GetNextWeekTime(1)), the first from the activity's
// start; the last one lasts until the activity ends.
func challengeSeason(activity activityConfig, config challengeConfig, now time.Time) uint32 {
	season := uint32(1)
	if !activity.Time.Start.IsZero() {
		season += uint32(periodStart(now, week).Sub(periodStart(activity.Time.Start, week)) / week)
	}
	return min(season, uint32(len(config.InfiniteStage)))
}

// challengeDungeons are the expedition_challenge_template ids of the round
// a run of mode is at: a casual run fights its season's first round
// (ChallengeMainScene.updateData shows infinite_stage[season][1] before an
// infinite run starts, CHALLENGEINFO.dungeon_id_list before a casual one),
// an infinite one each round in turn (UserChallengeInfo.setInfiniteDungeonIDListByLevel).
func challengeDungeons(config challengeConfig, season, mode, level uint32) []uint32 {
	rounds := config.InfiniteStage[(max(season, 1)-1)%uint32(len(config.InfiniteStage))]
	if len(rounds) == 0 {
		return nil
	}
	if mode == challengeCasual {
		return rounds[0]
	}
	return rounds[(max(level, 1)-1)/challengeBossNum%uint32(len(rounds))]
}

// challengeBoss is the expedition_challenge_template id of the boss a run
// fights at level (UserChallengeInfo.getNextExpedition), ok false once a
// casual run cleared its round.
func challengeBoss(config challengeConfig, run db.ChallengeRun) (uint32, bool) {
	if run.Mode == challengeCasual && run.Level > challengeBossNum {
		return 0, false
	}
	dungeons := challengeDungeons(config, run.Season, run.Mode, run.Level)
	if len(dungeons) == 0 {
		return 0, false
	}
	return dungeons[(run.Level-1)%uint32(len(dungeons))], true
}

// challengeInfo is SC_24005: the records (CHALLENGEINFO, season 0 the
// activity's) and the runs (USERCHALLENGEINFO) with their fleets.
func (game *Game) challengeInfo(ctx context.Context, commanderID uint32, activity activityConfig, config challengeConfig, now time.Time) (*protobuf.SC_24005, error) {
	queries := game.DB.Queries
	records, err := queries.ListChallengeRecords(ctx, db.ListChallengeRecordsParams{CommanderID: commanderID, ActivityID: activity.ID})
	if err != nil {
		return nil, err
	}
	season := challengeSeason(activity, config, now)
	current := &protobuf.CHALLENGEINFO{
		SeasonId:      proto.Uint32(season),
		DungeonIdList: challengeDungeons(config, season, challengeCasual, 1),
		BuffList:      config.Buff,
	}
	for _, record := range records {
		switch record.Season {
		case 0:
			current.ActivityMaxScore, current.ActivityMaxLevel = proto.Uint32(record.MaxScore), proto.Uint32(record.MaxLevel)
		case season:
			current.SeasonMaxScore, current.SeasonMaxLevel = proto.Uint32(record.MaxScore), proto.Uint32(record.MaxLevel)
		}
	}
	reply := &protobuf.SC_24005{Result: result(nil), CurrentChallenge: current}

	runs, err := queries.ListChallengeRuns(ctx, db.ListChallengeRunsParams{CommanderID: commanderID, ActivityID: activity.ID})
	if err != nil || len(runs) == 0 {
		return reply, err
	}
	ships, err := queries.ListChallengeShips(ctx, db.ListChallengeShipsParams{CommanderID: commanderID, ActivityID: activity.ID})
	if err != nil {
		return nil, err
	}
	cats, err := queries.ListChallengeMeowfficers(ctx, db.ListChallengeMeowfficersParams{CommanderID: commanderID, ActivityID: activity.ID})
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, len(ships))
	for i, ship := range ships {
		ids[i] = ship.ShipID
	}
	infos, err := guildAssaultShipInfos(ctx, queries, commanderID, ids)
	if err != nil {
		return nil, err
	}
	infoByID := make(map[uint32]*protobuf.SHIPINFO, len(infos))
	for _, info := range infos {
		infoByID[info.GetId()] = info
	}
	catByID := map[uint32]*protobuf.COMMANDERINFO{}
	for _, cat := range cats {
		row, err := queries.GetMeowfficer(ctx, db.GetMeowfficerParams{CommanderID: commanderID, ID: cat.MeowfficerID})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		} else if err != nil {
			return nil, err
		}
		catByID[cat.MeowfficerID] = meowfficerInfo(row)
	}
	for _, run := range runs {
		info := &protobuf.USERCHALLENGEINFO{
			CurrentScore:  proto.Uint32(run.Score),
			Level:         proto.Uint32(run.Level),
			Mode:          proto.Uint32(run.Mode),
			Issl:          proto.Uint32(boolFlag(run.Battling)),
			SeasonId:      proto.Uint32(run.Season),
			DungeonIdList: challengeDungeons(config, run.Season, run.Mode, run.Level),
			BuffList:      config.Buff,
		}
		group := func(id uint32) *protobuf.GROUPINFOINCHALLENGE {
			for _, existing := range info.GroupincList {
				if existing.GetId() == id {
					return existing
				}
			}
			created := &protobuf.GROUPINFOINCHALLENGE{Id: proto.Uint32(id)}
			info.GroupincList = append(info.GroupincList, created)
			return created
		}
		// both fleets, even empty: UserChallengeInfo reads _fleet and
		// _submarineFleet unchecked
		group(run.Mode + 1)
		group(run.Mode + submarineFleetID)
		for _, ship := range ships {
			if ship.Mode != run.Mode || infoByID[ship.ShipID] == nil {
				continue
			}
			entry := group(ship.FleetID)
			entry.Ships = append(entry.Ships, &protobuf.SHIPINCHALLENGE{
				Id:       proto.Uint32(ship.ShipID),
				HpRant:   proto.Uint32(ship.HpRant),
				ShipInfo: infoByID[ship.ShipID],
			})
		}
		for _, cat := range cats {
			if cat.Mode != run.Mode || catByID[cat.MeowfficerID] == nil {
				continue
			}
			entry := group(cat.FleetID)
			entry.Commanders = append(entry.Commanders, &protobuf.COMMANDERINCHALLENGE{
				Pos:           proto.Uint32(cat.Pos),
				Commanderinfo: catByID[cat.MeowfficerID],
			})
		}
		reply.UserChallenge = append(reply.UserChallenge, info)
	}
	return reply, nil
}

func boolFlag(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

// handleChallengeInfo answers Challenge2InfoRequestCommand (CS_24004): the
// season and records, and the runs in progress. Refused when the activity
// isn't the running challenge.
func (game *Game) handleChallengeInfo(client *connection.Client, req *protobuf.CS_24004) error {
	ctx, now := context.Background(), time.Now()
	activity, config, ok := openChallenge(now)
	if !ok || activity.ID != req.GetActivityId() {
		return client.Send(&protobuf.SC_24005{Result: result(errRefused), CurrentChallenge: &protobuf.CHALLENGEINFO{}})
	}
	reply, err := game.challengeInfo(ctx, client.CommanderID, activity, config, now)
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleChallengeStart answers Challenge2InitialRequestCommand (CS_24002):
// a run of mode starts at level 1 in the current season with the mode's
// fleets (mode+1, its submarines mode+11), every ship at full hp; the client
// then asks for CS_24004. Refused for a mode already running, a fleet of
// another mode, ships or meowfficers not the commander's, a ship twice, or
// no ship at all.
func (game *Game) handleChallengeStart(client *connection.Client, req *protobuf.CS_24002) error {
	ctx, now := context.Background(), time.Now()
	activity, config, ok := openChallenge(now)
	mode := req.GetMode()
	if !ok || activity.ID != req.GetActivityId() || (mode != challengeCasual && mode != challengeInfinite) {
		return client.Send(&protobuf.SC_24003{Result: result(errRefused)})
	}
	var ids []uint32
	for _, group := range req.GetGroupList() {
		if group.GetId() != mode+1 && group.GetId() != mode+submarineFleetID {
			return client.Send(&protobuf.SC_24003{Result: result(errRefused)})
		}
		ids = append(ids, group.GetShipList()...)
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, ids)
	if err != nil {
		return err
	}
	if len(ids) == 0 || len(byID) != len(ids) {
		return client.Send(&protobuf.SC_24003{Result: result(errRefused)})
	}
	err = game.tx(ctx, client, func(tx *Tx) error {
		_, err := tx.GetChallengeRun(ctx, db.GetChallengeRunParams{CommanderID: client.CommanderID, ActivityID: activity.ID, Mode: mode})
		if err == nil {
			return errRefused
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		err = tx.CreateChallengeRun(ctx, db.CreateChallengeRunParams{
			CommanderID: client.CommanderID,
			ActivityID:  activity.ID,
			Mode:        mode,
			Season:      challengeSeason(activity, config, now),
		})
		if err != nil {
			return err
		}
		for _, group := range req.GetGroupList() {
			for i, id := range group.GetShipList() {
				err := tx.AddChallengeShip(ctx, db.AddChallengeShipParams{
					CommanderID: client.CommanderID,
					ActivityID:  activity.ID,
					Mode:        mode,
					FleetID:     group.GetId(),
					Pos:         uint32(i),
					ShipID:      id,
					HpRant:      fullHP,
				})
				if err != nil {
					return err
				}
			}
			for _, cat := range group.GetCommanders() {
				_, err := tx.GetMeowfficer(ctx, db.GetMeowfficerParams{CommanderID: client.CommanderID, ID: cat.GetId()})
				if errors.Is(err, sql.ErrNoRows) {
					return errRefused
				} else if err != nil {
					return err
				}
				err = tx.AddChallengeMeowfficer(ctx, db.AddChallengeMeowfficerParams{
					CommanderID:  client.CommanderID,
					ActivityID:   activity.ID,
					Mode:         mode,
					FleetID:      group.GetId(),
					Pos:          cat.GetPos(),
					MeowfficerID: cat.GetId(),
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if refused(err) {
		return client.Send(&protobuf.SC_24003{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_24003{Result: result(nil)})
}

// handleChallengeReset answers Challenge2ResetRequestCommand (CS_24011): the
// run of mode ends (a battle lost below S, a run quit, a season over); its
// records were kept as it went. Nothing to end is not an error, the client
// only drops the run.
func (game *Game) handleChallengeReset(client *connection.Client, req *protobuf.CS_24011) error {
	ctx := context.Background()
	activity, _, ok := openChallenge(time.Now())
	if !ok || activity.ID != req.GetActivityId() {
		return client.Send(&protobuf.SC_24012{Result: result(errRefused)})
	}
	_, err := game.DB.DeleteChallengeRun(ctx, db.DeleteChallengeRunParams{CommanderID: client.CommanderID, ActivityID: activity.ID, Mode: req.GetMode()})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_24012{Result: result(nil)})
}

// challengeBattleRun is the run a CS_40001 / CS_40003 of SYSTEM_CHALLENGE
// is about (data2 is {level, mode}, data the stage), ok false when it isn't
// the boss the run is at.
func challengeBattleRun(ctx context.Context, queries *db.Queries, commanderID uint32, data uint32, data2 []uint32) (db.ChallengeRun, challengeExpedition, bool, error) {
	activity, config, ok := openChallenge(time.Now())
	if !ok || len(data2) < 2 {
		return db.ChallengeRun{}, challengeExpedition{}, false, nil
	}
	run, err := queries.GetChallengeRun(ctx, db.GetChallengeRunParams{CommanderID: commanderID, ActivityID: activity.ID, Mode: data2[1]})
	if errors.Is(err, sql.ErrNoRows) {
		return run, challengeExpedition{}, false, nil
	} else if err != nil {
		return run, challengeExpedition{}, false, err
	}
	boss, ok := challengeBoss(config, run)
	expedition, known := challengeExpeditions[boss]
	if !ok || !known || run.Level != data2[0] || expedition.DungeonID != data {
		return run, expedition, false, nil
	}
	return run, expedition, true, nil
}

// beginChallenge starts a challenge battle (BattleGateChallenge.Entrance):
// battle_cost_template[SYSTEM_CHALLENGE] costs no oil nor morale. The run
// counts as battling until the result (issl, the client offers a reset
// after a battle left without one).
func (game *Game) beginChallenge(client *connection.Client, req *protobuf.CS_40001) error {
	ctx := context.Background()
	run, _, ok, err := challengeBattleRun(ctx, game.DB.Queries, client.CommanderID, req.GetData(), req.GetData2())
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_40002{Result: result(errRefused)})
	}
	err = game.DB.SaveChallengeRun(ctx, db.SaveChallengeRunParams{
		Level:       run.Level,
		Score:       run.Score,
		Battling:    true,
		CommanderID: run.CommanderID,
		ActivityID:  run.ActivityID,
		Mode:        run.Mode,
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_40002{Result: result(nil), Key: proto.Uint32(rand.Uint32N(10000))})
}

// finishChallenge settles a challenge battle (BattleGateChallenge.Exit):
// the fleet keeps the hp it ended with (hp_rest, as the client writes it
// back). At S the run goes to the next level and gains the boss' points
// (its challenge, times challenge_rate_template's rate for the ships sunk:
// belfast's reading of the tables, the official formula is server-side),
// pushed in SC_24010 before the result (the result screen shows
// getLastScore); the records follow (ChallengeInfo.checkRecord: the casual
// score, the levels cleared). Below S the client resets the run itself
// (ChallengeProxy.WriteBackOnExitBattleResult). No exp nor drops
// (battle_cost_template[SYSTEM_CHALLENGE] awards none).
func (game *Game) finishChallenge(client *connection.Client, req *protobuf.CS_40003) error {
	ctx, now := context.Background(), time.Now()
	run, expedition, ok, err := challengeBattleRun(ctx, game.DB.Queries, client.CommanderID, req.GetData(), req.GetData2())
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_40004{Result: result(errRefused)})
	}
	activity, config, _ := openChallenge(now)
	won := min(req.GetScore(), scoreS) >= scoreS
	points := uint32(0)
	statShips := make([]uint32, len(req.GetStatistics()))
	for i, stat := range req.GetStatistics() {
		statShips[i] = stat.GetShipId()
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, statShips)
	if err != nil {
		return err
	}
	if won {
		sunk := uint32(0)
		for _, stat := range req.GetStatistics() {
			if _, ok := byID[stat.GetShipId()]; ok && stat.GetHpRest() == 0 {
				sunk++
			}
		}
		// more sunk than the table has: its lowest rate
		rate, known := challengeSunkRates[sunk]
		if !known && len(challengeSunkRates) > 0 {
			rate = slices.Min(slices.Collect(maps.Values(challengeSunkRates)))
		}
		points = uint32(math.Round(float64(expedition.Challenge) * rate))
		run.Level++
		run.Score += points
	}
	run.Battling = false
	err = game.tx(ctx, client, func(tx *Tx) error {
		for _, stat := range req.GetStatistics() {
			if _, ok := byID[stat.GetShipId()]; !ok {
				continue
			}
			err := tx.SetChallengeShipHP(ctx, db.SetChallengeShipHPParams{
				HpRant:      min(stat.GetHpRest(), fullHP),
				CommanderID: client.CommanderID,
				ActivityID:  run.ActivityID,
				Mode:        run.Mode,
				ShipID:      stat.GetShipId(),
			})
			if err != nil {
				return err
			}
		}
		err := tx.SaveChallengeRun(ctx, db.SaveChallengeRunParams{
			Level:       run.Level,
			Score:       run.Score,
			Battling:    run.Battling,
			CommanderID: run.CommanderID,
			ActivityID:  run.ActivityID,
			Mode:        run.Mode,
		})
		if err != nil || !won {
			return err
		}
		score := uint32(0)
		if run.Mode == challengeCasual {
			score = run.Score
		}
		seasons := []uint32{0}
		if run.Season == challengeSeason(activity, config, now) {
			seasons = append(seasons, run.Season)
		}
		for _, season := range seasons {
			err := tx.RecordChallenge(ctx, db.RecordChallengeParams{
				CommanderID: client.CommanderID,
				ActivityID:  run.ActivityID,
				Season:      season,
				MaxScore:    score,
				MaxLevel:    run.Level - 1,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := client.Push(&protobuf.SC_24010{Score: proto.Uint32(points)}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_40004{Result: result(nil), Mvp: proto.Uint32(battleMVP(req.GetStatistics(), byID))})
}

// activityFleets are the commander's stored activity fleets
// (ACTIVITYINFO.group_list), by activity.
func activityFleets(ctx context.Context, queries *db.Queries, commanderID uint32) (map[uint32][]*protobuf.GROUPINFO_P11, error) {
	ships, err := queries.ListActivityFleetShips(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	cats, err := queries.ListActivityFleetMeowfficers(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	fleets := map[uint32][]*protobuf.GROUPINFO_P11{}
	fleet := func(activityID, id uint32) *protobuf.GROUPINFO_P11 {
		for _, existing := range fleets[activityID] {
			if existing.GetId() == id {
				return existing
			}
		}
		created := &protobuf.GROUPINFO_P11{Id: proto.Uint32(id)}
		fleets[activityID] = append(fleets[activityID], created)
		return created
	}
	for _, ship := range ships {
		entry := fleet(ship.ActivityID, ship.FleetID)
		entry.ShipList = append(entry.ShipList, ship.ShipID)
	}
	for _, cat := range cats {
		entry := fleet(cat.ActivityID, cat.FleetID)
		entry.Commanders = append(entry.Commanders, &protobuf.COMMANDERSINFO{Pos: proto.Uint32(cat.Pos), Id: proto.Uint32(cat.MeowfficerID)})
	}
	return fleets, nil
}

// handleEditActivityFleet answers EditActivityFleetCommand (CS_11204): the
// activity's fleets are replaced by the ones sent, as the client keeps them
// (FleetProxy.activityFleetData, sent back in ACTIVITYINFO.group_list).
// Refused for an activity not running, or ships / meowfficers not the
// commander's.
func (game *Game) handleEditActivityFleet(client *connection.Client, req *protobuf.CS_11204) error {
	ctx := context.Background()
	reply := func(err error) error {
		return client.Send(&protobuf.SC_11205{ActivityId: proto.Uint32(req.GetActivityId()), Result: result(err)})
	}
	if _, ok := openActivity(req.GetActivityId(), time.Now()); !ok {
		return reply(errRefused)
	}
	var ids []uint32
	for _, group := range req.GetGroupList() {
		ids = append(ids, group.GetShipList()...)
	}
	byID, err := shipsByID(ctx, game.DB.Queries, client.CommanderID, ids)
	if err != nil {
		return err
	}
	if len(byID) != len(slices.Compact(slices.Sorted(slices.Values(ids)))) {
		return reply(errRefused)
	}
	err = game.tx(ctx, client, func(tx *Tx) error {
		key := db.ClearActivityFleetShipsParams{CommanderID: client.CommanderID, ActivityID: req.GetActivityId()}
		if err := tx.ClearActivityFleetShips(ctx, key); err != nil {
			return err
		}
		if err := tx.ClearActivityFleetMeowfficers(ctx, db.ClearActivityFleetMeowfficersParams(key)); err != nil {
			return err
		}
		for _, group := range req.GetGroupList() {
			for i, id := range group.GetShipList() {
				err := tx.AddActivityFleetShip(ctx, db.AddActivityFleetShipParams{
					CommanderID: client.CommanderID,
					ActivityID:  req.GetActivityId(),
					FleetID:     group.GetId(),
					Pos:         uint32(i),
					ShipID:      id,
				})
				if err != nil {
					return err
				}
			}
			for _, cat := range group.GetCommanders() {
				_, err := tx.GetMeowfficer(ctx, db.GetMeowfficerParams{CommanderID: client.CommanderID, ID: cat.GetId()})
				if errors.Is(err, sql.ErrNoRows) {
					return errRefused
				} else if err != nil {
					return err
				}
				err = tx.AddActivityFleetMeowfficer(ctx, db.AddActivityFleetMeowfficerParams{
					CommanderID:  client.CommanderID,
					ActivityID:   req.GetActivityId(),
					FleetID:      group.GetId(),
					Pos:          cat.GetPos(),
					MeowfficerID: cat.GetId(),
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if refused(err) {
		return reply(err)
	}
	if err != nil {
		return err
	}
	return reply(nil)
}
