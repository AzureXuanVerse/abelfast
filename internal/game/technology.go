package game

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Research Academy (view/technology/technologyscene.lua, TechnologyProxy):
// five technology_data_template projects are offered; one at a time is
// started (paying its consume up front), and runs its time. A project with
// a condition also needs its task (task_data_template, hidden) completed.
// A running project whose task is done can be moved to the research queue
// (up to TechnologyConst.QUEUE_TOTAL_COUNT), the next one started then only
// begins once the queue is through. A project's time is its finish time
// (Technology.time; waiting while time - its config time is still ahead).
//
// The official capture only has an empty SC_63000 (a level 3 commander);
// the packets are built from the Lua. The server-side rules come from the
// wiki's Research Academy page (offers, refreshes, focus, catch-up); the
// rewards' amounts are server-side and nowhere public, they are guesses
// (researchUnits).

// researchConfig is a technology_data_template row: its series
// (blueprint_version), time in seconds, the commander level it shows from,
// its condition task (0 none), its cost and what it gives (drop_client:
// items and the item_virtual_data_statistics placeholders of random
// rewards).
type researchConfig struct {
	Version    uint32     `json:"blueprint_version"`
	Time       uint32     `json:"time"`
	LvLimit    uint32     `json:"lv_limit"`
	Condition  uint32     `json:"condition"`
	Consume    [][]uint32 `json:"consume"`
	DropClient [][]uint32 `json:"drop_client"`
}

// researchCatchupConfig is a technology_catchup_template row: the ship
// groups one may pick (char_choice), the decisive ones (ur_char), and how
// many catch-up blueprints the priority ones share (obtain_max) and each
// decisive one gets (obtain_max_per_ur).
type researchCatchupConfig struct {
	CharChoice     []uint32 `json:"char_choice"`
	URChar         []uint32 `json:"ur_char"`
	ObtainMax      uint32   `json:"obtain_max"`
	ObtainMaxPerUR uint32   `json:"obtain_max_per_ur"`
}

// researchItemConfig is what researchUnits reads of item_data_statistics.
type researchItemConfig struct {
	Type   uint32 `json:"type"`
	Rarity uint32 `json:"rarity"`
}

const (
	// researchPool is the one refresh_list pool belfast offers: the one
	// whose target TechnologySettingsMediator sets (pool_id 2) and
	// TechnologyScene reads (getTendency(2)).
	researchPool = 2
	// researchOffers projects are offered (wiki: five).
	researchOffers = 5
	// researchFocusOffers of them at least belong to the focused series
	// (wiki: at least 3 out of 5).
	researchFocusOffers = 3
	// researchRetiredSeries: the series up to it are only offered when
	// focused (wiki: seasons 1-5 are retired).
	researchRetiredSeries = 5
	// researchQueueSize is TechnologyConst.QUEUE_TOTAL_COUNT.
	researchQueueSize = 5
	// researchRefreshType is the only type RefreshTechnologysCommand sends.
	researchRefreshType = 1
)

// item_data_statistics types of the rewards (researchUnits)
const (
	itemTypeMaterial  = 1  // materials, the Cognitive Chips among them
	itemTypeDesign    = 9  // gear designs
	itemTypeBlueprint = 12 // a ship's blueprints
	// the designs "Random Gear Design T2" (an item_virtual_data_statistics
	// entry with no display_icon) stands for: guessed to be the Elite
	// ones, as the "T3" one lists Super Rare ones
	researchT2DesignRarity = 3
)

// the drop_client placeholders without a display_icon: "Coins" and
// "Random Gear Design T2" (item_virtual_data_statistics type 98)
const (
	vitemResearchCoins    = 59001
	vitemResearchT2Design = 52004
)

var (
	researchProjects map[uint32]researchConfig
	researchIDs      []uint32 // researchProjects' ids, sorted
	researchCatchups map[uint32]researchCatchupConfig
	// researchBlueprintItems are ship_data_blueprint's strengthen_item by
	// ship group: the catch-up blueprints
	researchBlueprintItems map[uint32]uint32
	researchItems          map[uint32]researchItemConfig
	researchT2Designs      []uint32
	// researchMaxVersion is TechnologyProxy:getConfigMaxVersion, the
	// highest focus the settings offer
	researchMaxVersion uint32
	// researchOpenLevel is open_systems_limited's TechnologyMediator level
	researchOpenLevel uint32
)

func loadResearchData() error {
	var blueprints map[uint32]struct {
		StrengthenItem uint32 `json:"strengthen_item"`
		Version        uint32 `json:"blueprint_version"`
	}
	var systems map[uint32]struct {
		Mediator string `json:"mediator"`
		Level    uint32 `json:"level"`
	}
	err := gamedata.LoadAll(
		gamedata.Load("technology_data_template", &researchProjects),
		gamedata.Load("technology_catchup_template", &researchCatchups),
		gamedata.Load("ship_data_blueprint", &blueprints),
		gamedata.Load("item_data_statistics", &researchItems),
		gamedata.Load("open_systems_limited", &systems),
	)
	if err != nil {
		return err
	}
	researchOpenLevel = 0
	for _, row := range systems {
		if row.Mediator == "TechnologyMediator" {
			researchOpenLevel = row.Level
		}
	}
	if researchOpenLevel == 0 {
		return gamedata.Missing("open_systems_limited", "TechnologyMediator")
	}
	researchBlueprintItems = make(map[uint32]uint32, len(blueprints))
	researchMaxVersion = 0
	for group, row := range blueprints {
		researchBlueprintItems[group] = row.StrengthenItem
		researchMaxVersion = max(researchMaxVersion, row.Version)
	}
	researchIDs = researchIDs[:0]
	for id := range researchProjects {
		researchIDs = append(researchIDs, id)
	}
	slices.Sort(researchIDs)
	researchT2Designs = researchT2Designs[:0]
	for id, item := range researchItems {
		if item.Type == itemTypeDesign && item.Rarity == researchT2DesignRarity {
			researchT2Designs = append(researchT2Designs, id)
		}
	}
	slices.Sort(researchT2Designs)
	return nil
}

// researchState is the commander's technology_states row, its zero value
// before any.
func researchState(ctx context.Context, queries *db.Queries, commanderID uint32) (db.TechnologyState, error) {
	state, err := queries.GetTechnologyState(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.TechnologyState{CommanderID: commanderID}, nil
	}
	return state, err
}

func (tx *Tx) saveResearchState(state db.TechnologyState) error {
	return tx.SaveTechnologyState(tx.ctx, db.SaveTechnologyStateParams(state))
}

// researchRefreshList is refresh_list: the offers in their pool, with the
// pool's focus, none before any offer (the capture's commander below the
// open level).
func researchRefreshList(offers []db.TechnologyOffer, focus uint32) []*protobuf.TECHNOLOGYREFRESH {
	if len(offers) == 0 {
		return nil
	}
	pool := &protobuf.TECHNOLOGYREFRESH{Id: proto.Uint32(researchPool), Target: proto.Uint32(focus)}
	for _, offer := range offers {
		pool.Technologys = append(pool.Technologys, &protobuf.TECHNOLOGYINFO{
			Id:   proto.Uint32(offer.TechID),
			Time: proto.Uint32(uint32(offer.FinishAt)),
		})
	}
	return []*protobuf.TECHNOLOGYREFRESH{pool}
}

// researchInfo is SC_63000, TechnologyProxy's state: the offers, whether
// they were refreshed today (the client clears refresh_flag itself at
// midnight, its DayCall), the catch-up research and what it gave so far
// (the selected version's always, TechnologyProxy:getCurCatchNum reads
// it), the queue.
func researchInfo(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.SC_63000, error) {
	state, err := researchState(ctx, queries, commanderID)
	if err != nil {
		return nil, err
	}
	offers, err := queries.ListTechnologyOffers(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	queue, err := queries.ListTechnologyQueue(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	catchups, err := queries.ListTechnologyCatchups(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_63000{
		RefreshList: researchRefreshList(offers, state.Focus),
		RefreshFlag: proto.Uint32(0),
		Catchup: &protobuf.TECHNOLOGYCATCHUP{
			Version: proto.Uint32(state.CatchupVersion),
			Target:  proto.Uint32(state.CatchupTarget),
		},
	}
	if state.RefreshedAt != 0 && sameServerDay(time.Unix(state.RefreshedAt, 0), now) {
		msg.RefreshFlag = proto.Uint32(1)
	}
	pursuings := map[uint32]*protobuf.TECHPURSUING{}
	pursuing := func(version uint32) *protobuf.TECHPURSUING {
		if pursuings[version] == nil {
			pursuings[version] = &protobuf.TECHPURSUING{Version: proto.Uint32(version), Number: proto.Uint32(0)}
			msg.Catchup.Pursuings = append(msg.Catchup.Pursuings, pursuings[version])
		}
		return pursuings[version]
	}
	if state.CatchupVersion != 0 {
		pursuing(state.CatchupVersion)
	}
	for _, row := range catchups {
		entry := pursuing(row.Version)
		if row.ShipGroup == 0 {
			entry.Number = proto.Uint32(row.Number)
			continue
		}
		entry.DrNumbers = append(entry.DrNumbers, &protobuf.DR_NUMBER{Id: proto.Uint32(row.ShipGroup), Number: proto.Uint32(row.Number)})
	}
	for _, entry := range queue {
		msg.Queue = append(msg.Queue, &protobuf.TECHNOLOGYINFO{Id: proto.Uint32(entry.TechID), Time: proto.Uint32(uint32(entry.FinishAt))})
	}
	return msg, nil
}

// researchLoginPush is SC_63000 at login; a commander at the open level
// who has no offers yet gets them first.
func (game *Game) researchLoginPush(l *login) (*protobuf.SC_63000, error) {
	if l.commander.Level >= researchOpenLevel {
		offers, err := l.queries.ListTechnologyOffers(l.ctx, l.commander.ID)
		if err != nil {
			return nil, err
		}
		if len(offers) == 0 {
			err := game.DB.InTx(l.ctx, func(queries *db.Queries) error {
				return newTx(l.ctx, queries, l.commander.ID).rollResearch(l.commander.Level)
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return researchInfo(l.ctx, l.queries, l.commander.ID, l.now)
}

// researchUnlocked pushes the research state when a level up reaches the
// open level (addCommanderExp): TechnologyScene needs the offers and pool
// 2's focus as soon as the lab opens.
func (tx *Tx) researchUnlocked(from, to uint32) error {
	if from >= researchOpenLevel || to < researchOpenLevel {
		return nil
	}
	if err := tx.rollResearch(to); err != nil {
		return err
	}
	msg, err := researchInfo(tx.ctx, tx.Queries, tx.commanderID, time.Now())
	if err != nil {
		return err
	}
	tx.push(msg)
	return nil
}

// rollResearch draws new offers (wiki, Research Academy): researchOffers
// projects the commander's level shows, from the series not retired or
// the focused one, at least researchFocusOffers of them of the focused
// series when there is one. A project whose condition task a queued
// project still holds is left out: the task is the queued project's until
// it is claimed.
func (tx *Tx) rollResearch(level uint32) error {
	state, err := researchState(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return err
	}
	queue, err := tx.ListTechnologyQueue(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	held := map[uint32]bool{}
	for _, entry := range queue {
		held[researchProjects[entry.TechID].Condition] = true
	}
	var all, focused []uint32
	for _, id := range researchIDs {
		config := researchProjects[id]
		if config.LvLimit > level || (config.Condition != 0 && held[config.Condition]) {
			continue
		}
		if config.Version == state.Focus {
			focused = append(focused, id)
		}
		if config.Version > researchRetiredSeries || config.Version == state.Focus {
			all = append(all, id)
		}
	}
	shuffle := func(ids []uint32) {
		rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	}
	var picks []uint32
	if state.Focus != 0 {
		shuffle(focused)
		picks = append(picks, focused[:min(researchFocusOffers, len(focused))]...)
	}
	shuffle(all)
	for _, id := range all {
		if len(picks) >= researchOffers {
			break
		}
		if !slices.Contains(picks, id) {
			picks = append(picks, id)
		}
	}
	shuffle(picks)
	if err := tx.DeleteTechnologyOffers(tx.ctx, tx.commanderID); err != nil {
		return err
	}
	for i, id := range picks {
		err := tx.AddTechnologyOffer(tx.ctx, db.AddTechnologyOfferParams{CommanderID: tx.commanderID, Pos: uint32(i), PoolID: researchPool, TechID: id})
		if err != nil {
			return err
		}
	}
	return nil
}

// researchOffer finds the offer tech_id of pool refresh_id.
func (tx *Tx) researchOffer(techID, poolID uint32) (db.TechnologyOffer, []db.TechnologyOffer, error) {
	offers, err := tx.ListTechnologyOffers(tx.ctx, tx.commanderID)
	if err != nil {
		return db.TechnologyOffer{}, nil, err
	}
	for _, offer := range offers {
		if offer.TechID == techID && offer.PoolID == poolID {
			if _, ok := researchProjects[techID]; ok {
				return offer, offers, nil
			}
		}
	}
	return db.TechnologyOffer{}, nil, errRefused
}

// researchTaskDone is Technology:finishCondition outside of the queue: no
// condition, or its task completed.
func (tx *Tx) researchTaskDone(config researchConfig) (bool, error) {
	if config.Condition == 0 {
		return true, nil
	}
	task, err := tx.GetTask(tx.ctx, db.GetTaskParams{CommanderID: tx.commanderID, TaskID: config.Condition})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return task.Progress >= taskConfigs[config.Condition].TargetNum, nil
}

// dropResearchTask removes a project's condition task (SC_20004) once the
// project is stopped or claimed.
func (tx *Tx) dropResearchTask(config researchConfig) error {
	if config.Condition == 0 {
		return nil
	}
	_, err := tx.GetTask(tx.ctx, db.GetTaskParams{CommanderID: tx.commanderID, TaskID: config.Condition})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := tx.DeleteTasks(tx.ctx, db.DeleteTasksParams{CommanderID: tx.commanderID, TaskIds: []uint32{config.Condition}}); err != nil {
		return err
	}
	tx.push(&protobuf.SC_20004{IdList: []uint32{config.Condition}})
	return nil
}

// startResearchTask gives a started project its condition task from
// zero (SC_20003), the one a previous run left dropped first: TechnologyScene
// and the commission panel read it (TaskProxy:getTaskVO) while it runs.
func (tx *Tx) startResearchTask(config researchConfig, now time.Time) error {
	if config.Condition == 0 {
		return nil
	}
	if err := tx.dropResearchTask(config); err != nil {
		return err
	}
	added, err := acceptTasks(tx.ctx, tx.Queries, tx.commanderID, []uint32{config.Condition}, uint32(now.Unix()))
	if err != nil {
		return err
	}
	if added != nil {
		tx.push(added)
	}
	return nil
}

// researchUnits is how many of a drop_client entry a project gives, by
// what it is: these amounts are server-side and guessed from the project's
// time (one blueprint per two hours, one design per four hours, ten
// Cognitive Chips per hour, at least one), the rest one each.
func researchUnits(config researchConfig, kind, id uint32) uint32 {
	hours := config.Time / 3600
	itemType := uint32(0)
	if item, ok := researchItems[id]; ok && kind == dropTypeItem {
		itemType = item.Type
	} else if pool := dropPools[id]; len(pool) > 0 {
		itemType = researchItems[pool[0].id].Type
	}
	switch {
	case itemType == itemTypeBlueprint:
		return max(1, config.Time/7200)
	case itemType == itemTypeDesign || id == vitemResearchT2Design:
		return max(1, config.Time/14400)
	case itemType == itemTypeMaterial:
		return max(10, 10*hours)
	}
	return 1
}

// researchDrops is what a claimed project gives (common_list): its
// drop_client, each random placeholder drawn (resolveContent) per unit,
// the "Coins" placeholder as coins (guessed 100 per hour of the project)
// and the "Random Gear Design T2" one among the Elite designs, plus
// Specialized Cores (wiki: by the hours the project takes; guessed one per
// hour).
func researchDrops(config researchConfig) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for _, entry := range config.DropClient {
		if len(entry) < 2 {
			continue
		}
		kind, id := entry[0], entry[1]
		units := researchUnits(config, kind, id)
		switch {
		case id == vitemResearchCoins:
			drops = mergeDrop(drops, dropTypeResource, ResourceGold, max(100, 100*(config.Time/3600)))
		case id == vitemResearchT2Design:
			for range units {
				if len(researchT2Designs) > 0 {
					drops = mergeDrop(drops, dropTypeItem, researchT2Designs[rand.IntN(len(researchT2Designs))], 1)
				}
			}
		default:
			for _, drop := range resolveContent(nil, kind, id, units) {
				if storableDrop(drop.GetType(), drop.GetId()) {
					drops = mergeDrop(drops, drop.GetType(), drop.GetId(), drop.GetNumber())
				}
			}
		}
	}
	return mergeDrop(drops, dropTypeVItem, vitemSpecializedCore, max(1, config.Time/3600))
}

// researchCatchupDrop is the catch-up blueprints a claimed project adds
// (catchup_list) for the ship the commander picked (wiki: extra blueprints
// with every research done, 300 priority ones per series shared by its
// priority ships, 150 per decisive ship: obtain_max / obtain_max_per_ur),
// guessed as many as the project's own blueprints (one per two hours).
// TechnologyProxy:addCatupPrintsNum counts them on the client.
func (tx *Tx) researchCatchupDrop(config researchConfig) ([]*protobuf.DROPINFO, error) {
	state, err := researchState(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return nil, err
	}
	catchup, ok := researchCatchups[state.CatchupVersion]
	item := researchBlueprintItems[state.CatchupTarget]
	if !ok || item == 0 {
		return nil, nil
	}
	group, top := uint32(0), catchup.ObtainMax
	if slices.Contains(catchup.URChar, state.CatchupTarget) {
		group, top = state.CatchupTarget, catchup.ObtainMaxPerUR
	}
	rows, err := tx.ListTechnologyCatchups(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	got := uint32(0)
	for _, row := range rows {
		if row.Version == state.CatchupVersion && row.ShipGroup == group {
			got = row.Number
		}
	}
	if got >= top {
		return nil, nil
	}
	count := min(max(1, config.Time/7200), top-got)
	err = tx.AddTechnologyCatchup(tx.ctx, db.AddTechnologyCatchupParams{CommanderID: tx.commanderID, Version: state.CatchupVersion, ShipGroup: group, Number: count})
	if err != nil {
		return nil, err
	}
	drops := []*protobuf.DROPINFO{dropInfo(dropTypeItem, item, count)}
	return drops, tx.grantDrops(drops)
}

// claimResearch gives a project's rewards and drops its condition task.
func (tx *Tx) claimResearch(config researchConfig) (*protobuf.TECHNOLOGYDROP, error) {
	drops := researchDrops(config)
	if err := tx.grantDrops(drops); err != nil {
		return nil, err
	}
	catchup, err := tx.researchCatchupDrop(config)
	if err != nil {
		return nil, err
	}
	return &protobuf.TECHNOLOGYDROP{CommonList: drops, CatchupList: catchup}, tx.dropResearchTask(config)
}

// rerollResearch draws new offers after a project left them (claimed or
// queued, wiki) and returns them as refresh_list.
func (tx *Tx) rerollResearch() ([]*protobuf.TECHNOLOGYREFRESH, error) {
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	if err := tx.rollResearch(commander.Level); err != nil {
		return nil, err
	}
	state, err := researchState(tx.ctx, tx.Queries, tx.commanderID)
	if err != nil {
		return nil, err
	}
	offers, err := tx.ListTechnologyOffers(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	return researchRefreshList(offers, state.Focus), nil
}

// handleStartResearch starts the offer tech_id of pool refresh_id
// (StartTechnologyCommand, CS_63001) while no other offer runs, for its
// consume. It runs once the queue is through (the last queued project's
// finish time, or now), and gets its condition task. On result 0 the client
// takes the consume itself and sets the project's finish time to time; it
// tells technology_start_erro otherwise.
func (game *Game) handleStartResearch(client *connection.Client, req *protobuf.CS_63001) error {
	ctx := context.Background()
	var finish int64
	err := game.tx(ctx, client, func(tx *Tx) error {
		offer, offers, err := tx.researchOffer(req.GetTechId(), req.GetRefreshId())
		if err != nil {
			return err
		}
		for _, other := range offers {
			if other.FinishAt != 0 {
				return errRefused
			}
		}
		config := researchProjects[offer.TechID]
		if err := tx.payDrops(config.Consume); err != nil {
			return err
		}
		queue, err := tx.ListTechnologyQueue(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		now := time.Now()
		begin := now.Unix()
		for _, entry := range queue {
			begin = max(begin, entry.FinishAt)
		}
		finish = begin + int64(config.Time)
		err = tx.SetTechnologyOfferFinish(ctx, db.SetTechnologyOfferFinishParams{FinishAt: finish, CommanderID: tx.commanderID, Pos: offer.Pos})
		if err != nil {
			return err
		}
		return tx.startResearchTask(config, now)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63002{Result: result(err), Time: proto.Uint32(uint32(finish))})
}

// handleFinishResearch claims the offer tech_id of pool refresh_id once its
// time is up and its condition task done (FinishTechnologyCommand,
// CS_63003, from TechnologyScene or the commission panel), then draws new
// offers. On result 0 the client adds common_list, drop_list, catchup_list
// (counted as catch-up blueprints) and catchupact_list, and takes
// refresh_list as the offers; it tells technology_finish_erro otherwise.
func (game *Game) handleFinishResearch(client *connection.Client, req *protobuf.CS_63003) error {
	ctx := context.Background()
	reply := &protobuf.SC_63004{}
	err := game.tx(ctx, client, func(tx *Tx) error {
		offer, _, err := tx.researchOffer(req.GetTechId(), req.GetRefreshId())
		if err != nil {
			return err
		}
		config := researchProjects[offer.TechID]
		if offer.FinishAt == 0 || offer.FinishAt > time.Now().Unix() {
			return errRefused
		}
		done, err := tx.researchTaskDone(config)
		if err != nil {
			return err
		}
		if !done {
			return errRefused
		}
		drop, err := tx.claimResearch(config)
		if err != nil {
			return err
		}
		reply.CommonList, reply.CatchupList = drop.CommonList, drop.CatchupList
		reply.RefreshList, err = tx.rerollResearch()
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		reply = &protobuf.SC_63004{}
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleStopResearch stops the running offer tech_id of pool refresh_id
// before it is complete (StopTechnologyCommand, CS_63005): its consume is
// not given back and the offers stay (wiki), its condition task goes. On
// result 0 the client resets the project; it tells blueprint_stop_erro
// otherwise.
func (game *Game) handleStopResearch(client *connection.Client, req *protobuf.CS_63005) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		offer, _, err := tx.researchOffer(req.GetTechId(), req.GetRefreshId())
		if err != nil {
			return err
		}
		if offer.FinishAt == 0 {
			return errRefused
		}
		config := researchProjects[offer.TechID]
		done, err := tx.researchTaskDone(config)
		if err != nil {
			return err
		}
		if done && offer.FinishAt <= time.Now().Unix() {
			return errRefused // complete: claimed, not stopped
		}
		err = tx.SetTechnologyOfferFinish(ctx, db.SetTechnologyOfferFinishParams{FinishAt: 0, CommanderID: tx.commanderID, Pos: offer.Pos})
		if err != nil {
			return err
		}
		return tx.dropResearchTask(config)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63006{Result: result(err)})
}

// handleRefreshResearch draws new offers by hand (RefreshTechnologysCommand,
// CS_63007, type 1): once per server day (wiki), while none runs. On result
// 0 the client takes refresh_list and sets refresh_flag; it tells
// technology_refresh_erro otherwise.
func (game *Game) handleRefreshResearch(client *connection.Client, req *protobuf.CS_63007) error {
	ctx := context.Background()
	var list []*protobuf.TECHNOLOGYREFRESH
	err := game.tx(ctx, client, func(tx *Tx) error {
		if req.GetType() != researchRefreshType {
			return errRefused
		}
		state, err := researchState(ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		now := time.Now()
		if state.RefreshedAt != 0 && sameServerDay(time.Unix(state.RefreshedAt, 0), now) {
			return errRefused
		}
		offers, err := tx.ListTechnologyOffers(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if len(offers) == 0 {
			return errRefused
		}
		for _, offer := range offers {
			if offer.FinishAt != 0 {
				return errRefused
			}
		}
		state.RefreshedAt = now.Unix()
		if err := tx.saveResearchState(state); err != nil {
			return err
		}
		list, err = tx.rerollResearch()
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63008{Result: result(err), RefreshList: list})
}

// handleResearchFocus sets the series pool id favours, target 0 for none
// (ChangeRefreshTechnologysTendencyCommand, CS_63009, from
// TechnologySettingsLayer, always pool 2): the next draws take it into
// account, the offers stay. On result 0 the client keeps target as the
// pool's; it tells change_technology_refresh_erro otherwise.
func (game *Game) handleResearchFocus(client *connection.Client, req *protobuf.CS_63009) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		if req.GetId() != researchPool || req.GetTarget() > researchMaxVersion {
			return errRefused
		}
		state, err := researchState(ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		state.Focus = req.GetTarget()
		return tx.saveResearchState(state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63010{Result: result(err)})
}

// handleResearchCatchup picks the catch-up research: the ship group target
// of technology_catchup_template version (SelectTecTargetCatchupCommand,
// CS_63011, from the settings' catch-up pages), one whose catch-up isn't
// complete; it can change at any time (wiki). Target 0 drops the catch-up.
// On result 0 the client keeps them (TechnologyProxy:setCurCatchupTecInfo);
// it tells "Error Code" otherwise.
func (game *Game) handleResearchCatchup(client *connection.Client, req *protobuf.CS_63011) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := researchState(ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		if req.GetTarget() == 0 {
			state.CatchupVersion, state.CatchupTarget = 0, 0
			return tx.saveResearchState(state)
		}
		catchup, ok := researchCatchups[req.GetVersion()]
		if !ok || !slices.Contains(catchup.CharChoice, req.GetTarget()) || researchBlueprintItems[req.GetTarget()] == 0 {
			return errRefused
		}
		group, top := uint32(0), catchup.ObtainMax
		if slices.Contains(catchup.URChar, req.GetTarget()) {
			group, top = req.GetTarget(), catchup.ObtainMaxPerUR
		}
		rows, err := tx.ListTechnologyCatchups(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Version == req.GetVersion() && row.ShipGroup == group && row.Number >= top {
				return errRefused
			}
		}
		state.CatchupVersion, state.CatchupTarget = req.GetVersion(), req.GetTarget()
		return tx.saveResearchState(state)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63012{Result: result(err)})
}

// handleJoinResearchQueue moves the running offer tech_id of pool
// refresh_id to the research queue (JoinQueueTechnologyCommand, CS_63013):
// its condition task done, its time not up yet, the queue not full; new
// offers are drawn (wiki). ReSelectTecTargetCatchupCommand also names
// 63013 but nothing sends it. On result 0 the client moves the project
// (TechnologyProxy:moveTechnologyToQueue) and takes refresh_list; it tells
// blueprint_stop_erro otherwise.
func (game *Game) handleJoinResearchQueue(client *connection.Client, req *protobuf.CS_63013) error {
	ctx := context.Background()
	var list []*protobuf.TECHNOLOGYREFRESH
	err := game.tx(ctx, client, func(tx *Tx) error {
		offer, _, err := tx.researchOffer(req.GetTechId(), req.GetRefreshId())
		if err != nil {
			return err
		}
		if offer.FinishAt <= time.Now().Unix() {
			return errRefused
		}
		done, err := tx.researchTaskDone(researchProjects[offer.TechID])
		if err != nil {
			return err
		}
		if !done {
			return errRefused
		}
		queue, err := tx.ListTechnologyQueue(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if len(queue) >= researchQueueSize {
			return errRefused
		}
		err = tx.AddTechnologyQueue(ctx, db.AddTechnologyQueueParams{CommanderID: tx.commanderID, TechID: offer.TechID, FinishAt: offer.FinishAt})
		if err != nil {
			return err
		}
		list, err = tx.rerollResearch()
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_63014{Result: result(err), RefreshList: list})
}

// handleFinishResearchQueue claims the queued projects whose time is up,
// in order (FinishQueueTechnologyCommand, CS_63015, id always 0). On result
// 0 the client removes the queue's first project per entry of drops and
// adds its rewards; it tells blueprint_stop_erro otherwise.
func (game *Game) handleFinishResearchQueue(client *connection.Client, req *protobuf.CS_63015) error {
	ctx := context.Background()
	var drops []*protobuf.TECHNOLOGYDROP
	err := game.tx(ctx, client, func(tx *Tx) error {
		queue, err := tx.ListTechnologyQueue(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		for _, entry := range queue {
			if entry.FinishAt > now {
				break
			}
			drop, err := tx.claimResearch(researchProjects[entry.TechID])
			if err != nil {
				return err
			}
			drops = append(drops, drop)
			if err := tx.DeleteTechnologyQueue(ctx, db.DeleteTechnologyQueueParams{CommanderID: tx.commanderID, ID: entry.ID}); err != nil {
				return err
			}
		}
		if len(drops) == 0 {
			return errRefused
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_63016{Result: result(err), Drops: drops})
}
