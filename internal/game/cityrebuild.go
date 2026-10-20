package game

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
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

// The City Rebuild activities (ActivityConst.ACTIVITY_TYPE_CITY_REBUILD,
// CityRebuildProxy, CityRebuildData; "Turmoil in the Ninja Castle" on the
// wiki, activity 50116): an idle game. The commander repairs facilities and
// recruits troops (activity_ninja_building) with Repair Tools / Lord's Orders
// (virtual items kept in the activity's bag, ACTIVITYINFO
// date1_key_value_list[1]) and Ninja Koban (the city's pt), which raises the
// city level (activity_ninja_city: every unit of a level done) and the
// damage per second of the Castle Defense. Its waves (activity_ninja_enemy)
// are fought on their own while time passes, even offline (wiki): an enemy
// of the wave chosen dies every ceil(HP / DPS) seconds and drops koban,
// collected with CS_26068; defeating the highest wave lets the next one be
// chosen. Buffs (activity_ninja_buff) bought with koban raise the DPS, the
// koban per kill, lower the enemies' HP or the buffs' cost.
//
// The formulas (HP, DPS, koban per kill, buff costs) are the client's
// (CityRebuildGameLayer:SetSummaryPanelAndHp, CityRebuildBookLayer
// RefreshBuffPage, gameset ninja_Param*), which the server mirrors to run the
// fight: the NINJA_ADJUST it sends (when the wave fought started, its HP
// left, the highest wave defeated) is what the client's own timer starts
// from. No capture has any of it; the rules the Lua doesn't fix are guesses,
// named where they are made.

const activityTypeCityRebuild = 129

// activity_ninja_building.type
const (
	cityUnitFacility = 1 // repaired at once
	cityUnitTroop    = 2 // recruited over time seconds
)

// Task sub types of the Castle Defense tasks (task_data_template, activity
// 50134's and 50133's; the server counts them, their descs say what)
const (
	TaskSubCityDPS     = 422 // "Reach a DPS of N" (absolute)
	TaskSubCityPtSpent = 423 // "Spend a total of N Ninja Koban"
	TaskSubCityWave    = 424 // "Reach wave N" (absolute)
	TaskSubCityCollect = 425 // "Pick up rewards N times"
)

// cityBuffDiscountGroup is the buff group lowering the buffs' cost
// (ninja_buff_effect6, ninja_Param7 per level)
const cityBuffDiscountGroup = 6

// activity_ninja_city row: the units of a city level and the highest level
// of each buff group (by group - 1) it allows
type cityRebuildLevel struct {
	ID      uint32   `json:"id"`
	Level   uint32   `json:"level"`
	Buff    []uint32 `json:"buff"`
	Include []uint32 `json:"include"`
}

// activity_ninja_building row: [DROP_TYPE, id, count] costs
type cityRebuildUnit struct {
	Type   uint32   `json:"type"`
	Cost   []uint32 `json:"cost"`
	PtCost []uint32 `json:"pt_cost"`
	Time   int64    `json:"time"`
}

// activity_ninja_buff row; cost is a Lua string ("1.12")
type cityRebuildBuff struct {
	ID        uint32          `json:"id"`
	Group     uint32          `json:"group"`
	Level     uint32          `json:"level"`
	BasicCost float64         `json:"basic_cost"`
	Cost      json.RawMessage `json:"cost"`
	cost      float64
}

// activity_ninja_enemy row: a wave's base HP and its first defeat rewards
type cityRebuildEnemy struct {
	Basic         float64    `json:"basic"`
	FirstDropShow [][]uint32 `json:"first_drop_show"`
}

var cityRebuild struct {
	levels    []cityRebuildLevel // by id, city level 1 first
	units     map[uint32]cityRebuildUnit
	buffs     map[uint32]map[uint32]cityRebuildBuff // by group, then level
	groups    []uint32                              // buff groups, ascending
	enemies   map[uint32]cityRebuildEnemy
	lastEnemy uint32
	param     map[uint32]float64 // gameset ninja_Param<n>

	// item_virtual_data_statistics: the koban (virtual_type 102, which the
	// client adds to its city, Drop's case 102) by item id → activity, and
	// the activities' bag items (virtual_type 0, linked to the activity)
	ptItems  map[uint32]uint32
	bagItems map[uint32][]uint32
}

// cityRebuildParams are the ninja_Param<n> the client's formulas read.
var cityRebuildParams = []uint32{1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 13, 14, 15, 16}

func loadCityRebuildData() error {
	var levels map[uint32]cityRebuildLevel
	var buffs map[uint32]cityRebuildBuff
	var vitems map[uint32]struct {
		VirtualType uint32 `json:"virtual_type"`
		LinkID      uint32 `json:"link_id"`
	}
	var gameset map[string]uint32
	err := gamedata.LoadAll(
		gamedata.Load("activity_ninja_city", &levels),
		gamedata.Load("activity_ninja_building", &cityRebuild.units),
		gamedata.Load("activity_ninja_buff", &buffs),
		gamedata.Load("activity_ninja_enemy", &cityRebuild.enemies),
		gamedata.Load("item_virtual_data_statistics", &vitems),
		func() (err error) { gameset, err = gamedata.Gameset(); return err },
	)
	if err != nil {
		return err
	}
	cityRebuild.levels = slices.SortedFunc(maps.Values(levels), func(a, b cityRebuildLevel) int { return cmp.Compare(a.ID, b.ID) })
	cityRebuild.buffs = map[uint32]map[uint32]cityRebuildBuff{}
	for _, buff := range buffs {
		text := strings.Trim(string(buff.Cost), `"`)
		if buff.cost, err = strconv.ParseFloat(text, 64); err != nil {
			return gamedata.Missing("activity_ninja_buff", "cost of %d", buff.ID)
		}
		if cityRebuild.buffs[buff.Group] == nil {
			cityRebuild.buffs[buff.Group] = map[uint32]cityRebuildBuff{}
		}
		cityRebuild.buffs[buff.Group][buff.Level] = buff
	}
	cityRebuild.groups = slices.Sorted(maps.Keys(cityRebuild.buffs))
	for _, group := range cityRebuild.groups {
		if _, ok := cityRebuild.buffs[group][1]; !ok {
			return gamedata.Missing("activity_ninja_buff", "level 1 of group %d", group)
		}
	}
	if len(cityRebuild.enemies) > 0 {
		cityRebuild.lastEnemy = slices.Max(slices.Collect(maps.Keys(cityRebuild.enemies)))
	}
	// CityRebuildGameLayer:GetParam: key_value, or the description's number
	cityRebuild.param = map[uint32]float64{}
	for _, n := range cityRebuildParams {
		key := fmt.Sprintf("ninja_Param%d", n)
		if value := gameset[key]; value != 0 {
			cityRebuild.param[n] = float64(value)
			continue
		}
		var text string
		if err := gamedata.GamesetDescription(key, &text); err != nil {
			return err
		}
		if cityRebuild.param[n], err = strconv.ParseFloat(text, 64); err != nil {
			return gamedata.Missing("gameset", "%s", key)
		}
	}
	cityRebuild.ptItems, cityRebuild.bagItems = map[uint32]uint32{}, map[uint32][]uint32{}
	for _, id := range slices.Sorted(maps.Keys(vitems)) {
		switch item := vitems[id]; {
		case item.VirtualType == 102 && item.LinkID != 0:
			cityRebuild.ptItems[id] = item.LinkID
		case item.VirtualType == 0 && item.LinkID != 0:
			cityRebuild.bagItems[item.LinkID] = append(cityRebuild.bagItems[item.LinkID], id)
		}
	}
	return nil
}

// cityParam is gameset ninja_Param<n>.
func cityParam(n uint32) float64 { return cityRebuild.param[n] }

// cityCeil is the client's math.ceil of a formula.
func cityCeil(value float64) int64 { return int64(math.Ceil(value)) }

// cityState is a commander's city as stored: the state row, the units by id
// and the buff levels by group (1 when not stored).
type cityState struct {
	row    db.CommanderCityRebuild
	units  map[uint32]db.ListCityRebuildUnitsRow
	levels map[uint32]uint32
}

// loadCity reads the commander's city of activity, created when new.
func loadCity(ctx context.Context, queries *db.Queries, commanderID, activityID uint32) (*cityState, error) {
	if err := queries.CreateCityRebuild(ctx, db.CreateCityRebuildParams{CommanderID: commanderID, ActivityID: activityID}); err != nil {
		return nil, err
	}
	row, err := queries.GetCityRebuild(ctx, db.GetCityRebuildParams{CommanderID: commanderID, ActivityID: activityID})
	if err != nil {
		return nil, err
	}
	units, err := queries.ListCityRebuildUnits(ctx, db.ListCityRebuildUnitsParams{CommanderID: commanderID, ActivityID: activityID})
	if err != nil {
		return nil, err
	}
	buffs, err := queries.ListCityRebuildBuffs(ctx, db.ListCityRebuildBuffsParams{CommanderID: commanderID, ActivityID: activityID})
	if err != nil {
		return nil, err
	}
	city := &cityState{row: row, units: map[uint32]db.ListCityRebuildUnitsRow{}, levels: map[uint32]uint32{}}
	for _, unit := range units {
		city.units[unit.UnitID] = unit
	}
	for _, buff := range buffs {
		city.levels[buff.BuffGroup] = buff.Level
	}
	return city, nil
}

// save stores the city's state row.
func (city *cityState) save(ctx context.Context, queries *db.Queries) error {
	row := city.row
	return queries.SetCityRebuild(ctx, db.SetCityRebuildParams{
		Pt: row.Pt, SummaryPt: row.SummaryPt, MaxLevel: row.MaxLevel, CurLevel: row.CurLevel, MaxDisplay: row.MaxDisplay,
		FightTime: row.FightTime, LeftHp: row.LeftHp, AwardedLevel: row.AwardedLevel,
		CommanderID: row.CommanderID, ActivityID: row.ActivityID,
	})
}

// buff is the level of a buff group.
func (city *cityState) buff(group uint32) uint32 {
	if level, ok := city.levels[group]; ok {
		return level
	}
	return 1
}

// done tells whether unit was repaired or recruited
// (CityRebuildData:IsRepairedOrRecruited).
func (city *cityState) done(unit uint32) bool {
	return city.units[unit].Done
}

// count is how many units of a type are done.
func (city *cityState) count(kind uint32) int {
	n := 0
	for id, unit := range city.units {
		if unit.Done && cityRebuild.units[id].Type == kind {
			n++
		}
	}
	return n
}

// cityLevel is CityRebuildData:TryUpgradeCityLevel: the next level once
// every unit of the current one is done.
func (city *cityState) cityLevel() uint32 {
	level := uint32(1)
	for {
		i := slices.IndexFunc(cityRebuild.levels, func(row cityRebuildLevel) bool { return row.Level == level })
		if i < 0 || i+1 >= len(cityRebuild.levels) {
			return level
		}
		for _, unit := range cityRebuild.levels[i].Include {
			if !city.done(unit) {
				return level
			}
		}
		level++
	}
}

// unlocked tells whether unit belongs to a city level reached
// (CityRebuildData:IsUnlock).
func (city *cityState) unlocked(unit uint32) bool {
	level := city.cityLevel()
	for _, row := range cityRebuild.levels {
		if row.Level <= level && slices.Contains(row.Include, unit) {
			return true
		}
	}
	return false
}

// buffCap is the highest level of a buff group the city level allows.
func (city *cityState) buffCap(group uint32) uint32 {
	level := city.cityLevel()
	for _, row := range cityRebuild.levels {
		if row.Level == level && group >= 1 && int(group) <= len(row.Buff) {
			return row.Buff[group-1]
		}
	}
	return 0
}

// dps is the facilities' plus the troops' damage per second
// (SetSummaryPanelAndHp).
func (city *cityState) dps() int64 {
	b := city.buff
	facilities := float64(city.count(cityUnitFacility)) + float64(b(10))
	facilityDPS := cityCeil((facilities+facilities*math.Pow(cityParam(1), float64(b(8))-1)/cityParam(2))*math.Pow(cityParam(3), float64(b(9))-1) + cityParam(4)*float64(b(7)))
	troopDPS := cityCeil((float64(city.count(cityUnitTroop))+float64(b(5)))*math.Pow(cityParam(5), float64(b(1))-1) + cityParam(6)*float64(b(2)))
	return facilityDPS + troopDPS
}

// hp is the HP of an enemy of a wave (SetSummaryPanelAndHp). Every 5th wave
// is a boss with ninja_Param10 times the HP. The decompiled Lua reads
// `(level >= 51 or A) and B`, taken as "A below wave 51, B from it": B for
// every wave would give wave 1 ~18,000 HP against 43 DPS (a GUESS on a
// decompiler artefact).
func (city *cityState) hp(level uint32) int64 {
	enemy, ok := cityRebuild.enemies[level]
	if !ok {
		return 0
	}
	boss := 1.0
	if level%5 == 0 {
		boss = cityParam(10)
	}
	weaken := math.Pow(cityParam(11), float64(city.buff(3))-1)
	if level < 51 {
		return cityCeil(enemy.Basic * (float64(level) - 1 + math.Pow(cityParam(9), float64(level)-1)) * boss * weaken)
	}
	p16 := cityParam(16)
	return cityCeil(enemy.Basic * (p16 + math.Pow(cityParam(9), p16)*math.Pow(cityParam(13), float64(level)-(p16+1))) * boss * weaken)
}

// ptPerKill is the koban an enemy of the wave drops (ptAwardCount).
func (city *cityState) ptPerKill(level uint32) int64 {
	return cityCeil(math.Pow(cityParam(14), float64(level)) * math.Pow(cityParam(15), float64(city.buff(4))))
}

// settle runs the fight up to now as the client's timer shows it
// (SetSummaryPanelAndHp): the first enemy has left_hp (0: full) and dies
// after ceil(left_hp / DPS) seconds, the next ones every ceil(HP / DPS)
// seconds; each kill adds the wave's koban to the summary, and a kill
// defeats the wave (max_level). The fight then restarts from now with the HP
// left, which is what NINJA_ADJUST tells the client. Nothing runs before
// CS_26072 started the fight.
func (city *cityState) settle(now int64) {
	row := &city.row
	if row.FightTime == 0 || now <= row.FightTime {
		return
	}
	dps, full := city.dps(), city.hp(row.CurLevel)
	if dps <= 0 || full <= 0 {
		return
	}
	first := row.LeftHp
	if first == 0 {
		first = full
	}
	toFirst, each := (first+dps-1)/dps, (full+dps-1)/dps
	elapsed := now - row.FightTime
	if elapsed < toFirst {
		row.LeftHp = first - elapsed*dps
	} else {
		kills := 1 + (elapsed-toFirst)/each
		row.LeftHp = full - (elapsed-toFirst)%each*dps
		row.SummaryPt += kills * city.ptPerKill(row.CurLevel)
		row.MaxLevel = max(row.MaxLevel, row.CurLevel)
	}
	row.FightTime = now
}

// ninjaPt is a big number as NINJA_PT: k + m * 10^6 + b * 10^9
// (CityRebuildData.Ctor).
func ninjaPt(value int64) *protobuf.NINJA_PT {
	value = max(value, 0)
	return &protobuf.NINJA_PT{
		B: proto.Uint32(uint32(value / 1e9)),
		M: proto.Uint32(uint32(value % 1e9 / 1e6)),
		K: proto.Uint32(uint32(value % 1e6)),
	}
}

// adjust is the fight's NINJA_ADJUST (CityRebuildData:Adjust).
func (city *cityState) adjust() *protobuf.NINJA_ADJUST {
	if city == nil {
		return &protobuf.NINJA_ADJUST{Time: proto.Uint32(0), LeftHp: ninjaPt(0), MaxLevel: proto.Uint32(0)}
	}
	return &protobuf.NINJA_ADJUST{
		Time:     proto.Uint32(uint32(city.row.FightTime)),
		LeftHp:   ninjaPt(city.row.LeftHp),
		MaxLevel: proto.Uint32(city.row.MaxLevel),
	}
}

// info is the city as SC_26061 gives it (CityRebuildData.Ctor). buffs holds
// one activity_ninja_buff id per group: the client indexes its buff page and
// its formulas by group, level 1 to begin with (a GUESS: a new city with no
// buff would break both).
func (city *cityState) info() *protobuf.NINJA_INFO {
	info := &protobuf.NINJA_INFO{
		Pt:         ninjaPt(city.row.Pt),
		MaxLevel:   proto.Uint32(city.row.MaxLevel),
		CurLevel:   proto.Uint32(city.row.CurLevel),
		MaxDisplay: proto.Uint32(city.row.MaxDisplay),
		Adjust:     city.adjust(),
		SummaryPt:  ninjaPt(city.row.SummaryPt),
	}
	for _, id := range slices.Sorted(maps.Keys(city.units)) {
		unit := city.units[id]
		switch {
		case !unit.Done:
			info.Recruits = append(info.Recruits, &protobuf.NINJA_ROLE_RECRUIT{Id: proto.Uint32(id), StartTime: proto.Uint32(uint32(unit.StartTime))})
		case cityRebuild.units[id].Type == cityUnitFacility:
			info.Builds = append(info.Builds, id)
		default:
			info.Roles = append(info.Roles, id)
		}
	}
	for _, group := range cityRebuild.groups {
		info.Buffs = append(info.Buffs, cityRebuild.buffs[group][city.buff(group)].ID)
	}
	return info
}

// pay takes one [DROP_TYPE, id, count] cost: the activity's koban from the
// city, a bag item from the virtual items.
func (city *cityState) pay(tx *Tx, cost []uint32) error {
	if len(cost) < 3 || cost[2] == 0 {
		return nil
	}
	if cost[0] != dropTypeVItem {
		return errRefused
	}
	if cityRebuild.ptItems[cost[1]] == city.row.ActivityID {
		if city.row.Pt < int64(cost[2]) {
			return ErrNotEnough
		}
		city.row.Pt -= int64(cost[2])
		tx.event(TaskEvent{SubType: TaskSubCityPtSpent, Count: cost[2]})
		return nil
	}
	n, err := tx.SpendCityRebuildItem(tx.ctx, db.SpendCityRebuildItemParams{Amount: cost[2], CommanderID: tx.commanderID, ItemID: cost[1]})
	if err == nil && n == 0 {
		err = ErrNotEnough
	}
	return err
}

// buffCost is the koban count levels of a buff group cost, false past the
// level the city allows (CityRebuildBookLayer upgradeBtn / upgradeTenBtn):
// level n costs ceil(basic_cost * cost^(n-1) * (1 - ninja_Param7 * d)), d
// the discount group's level (its level before the step for itself).
func (city *cityState) buffCost(group, count uint32) (int64, bool) {
	current := city.buff(group)
	if count == 0 || current+count > city.buffCap(group) {
		return 0, false
	}
	total := int64(0)
	for level := current + 1; level <= current+count; level++ {
		row, ok := cityRebuild.buffs[group][level]
		if !ok {
			return 0, false
		}
		discount := city.buff(cityBuffDiscountGroup)
		if group == cityBuffDiscountGroup {
			discount = level - 1
		}
		total += cityCeil(row.BasicCost * math.Pow(row.cost, float64(row.Level)-1) * (1 - cityParam(7)*float64(discount)))
	}
	return total, true
}

// cityEvents records the absolute Castle Defense counters: the DPS and the
// wave reached (max_display, shown as the wave, ninja_game_wave).
func (city *cityState) cityEvents(tx *Tx) {
	tx.event(TaskEvent{SubType: TaskSubCityDPS, Count: uint32(min(city.dps(), math.MaxUint32))},
		TaskEvent{SubType: TaskSubCityWave, Count: city.row.MaxDisplay})
}

// cityTx runs fn on the commander's city of a running City Rebuild activity,
// its fight settled at now, and stores the city when fn succeeds. The city
// is nil when the activity isn't one or doesn't run.
func (game *Game) cityTx(client *connection.Client, activityID uint32, fn func(tx *Tx, city *cityState, now int64) error) (*cityState, error) {
	ctx := context.Background()
	now := time.Now()
	config, open, err := game.commanderActivity(ctx, client.CommanderID, activityID, now)
	if err != nil {
		return nil, err
	}
	if !open || config.Type != activityTypeCityRebuild {
		return nil, errRefused
	}
	var city *cityState
	err = game.tx(ctx, client, func(tx *Tx) error {
		loaded, err := loadCity(tx.ctx, tx.Queries, tx.commanderID, activityID)
		if err != nil {
			return err
		}
		loaded.settle(now.Unix())
		if err := fn(tx, loaded, now.Unix()); err != nil {
			return err
		}
		city = loaded
		return city.save(tx.ctx, tx.Queries)
	})
	if err != nil {
		return nil, err
	}
	return city, nil
}

// cityReplyErr is what a city handler's reply is built from: a refusal
// becomes the reply's result, anything else ends the connection.
func cityReplyErr(err error) error {
	if err != nil && !refused(err) {
		return err
	}
	return nil
}

// handleCityRebuildData answers CityRebuildCommand GET_DATA (CS_26060,
// controller/command/activity/cityrebuildcommand.lua), sent when the city
// map opens: the city with the fight settled, its koban to collect in
// summary_pt.
func (game *Game) handleCityRebuildData(client *connection.Client, req *protobuf.CS_26060) error {
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		city.cityEvents(tx)
		return nil
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	reply := &protobuf.SC_26061{Result: result(err)}
	if city != nil {
		reply.Info = city.info()
	}
	return client.Send(reply)
}

// handleCityRebuildBuild answers CityRebuildCommand REBUILD_OR_START_RECRUIT
// (CS_26064, the book's repair / recruit buttons): a facility is repaired at
// once, a troop starts recruiting now; the client then takes the unit's
// pt_cost from its koban and its cost from the activity's bag (ComsumePt,
// subVitemNumber), as the server does. The unit must belong to a city level
// reached and not be repaired, recruited or recruiting.
func (game *Game) handleCityRebuildBuild(client *connection.Client, req *protobuf.CS_26064) error {
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		id := req.GetBuildingId()
		unit, ok := cityRebuild.units[id]
		if _, started := city.units[id]; !ok || started || !city.unlocked(id) {
			return errRefused
		}
		if err := city.pay(tx, unit.Cost); err != nil {
			return err
		}
		if err := city.pay(tx, unit.PtCost); err != nil {
			return err
		}
		row := db.ListCityRebuildUnitsRow{UnitID: id, Done: unit.Type == cityUnitFacility, StartTime: now}
		city.units[id] = row
		city.cityEvents(tx)
		return tx.SetCityRebuildUnit(tx.ctx, db.SetCityRebuildUnitParams{CommanderID: tx.commanderID, ActivityID: req.GetActId(), UnitID: id, Done: row.Done, StartTime: now})
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_26065{Result: result(err), Adjust: city.adjust()})
}

// handleCityRebuildRecruited answers CityRebuildCommand END_RECRUIT
// (CS_26062, the book's "recruit done" button): troops recruiting for their
// activity_ninja_building time join the city.
func (game *Game) handleCityRebuildRecruited(client *connection.Client, req *protobuf.CS_26062) error {
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		if len(req.GetRoles()) == 0 {
			return errRefused
		}
		for _, id := range req.GetRoles() {
			unit, ok := city.units[id]
			if !ok || unit.Done || now-unit.StartTime < cityRebuild.units[id].Time {
				return errRefused
			}
			unit.Done = true
			city.units[id] = unit
			if err := tx.SetCityRebuildUnit(tx.ctx, db.SetCityRebuildUnitParams{CommanderID: tx.commanderID, ActivityID: req.GetActId(), UnitID: id, Done: true, StartTime: unit.StartTime}); err != nil {
				return err
			}
		}
		city.cityEvents(tx)
		return nil
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_26063{Result: result(err), Adjust: city.adjust()})
}

// handleCityRebuildBuff answers CityRebuildCommand UPGRADE_BUFF (CS_26066,
// the buff page's +1 / +10 buttons): count levels of a buff group, up to
// what the city level allows, for the koban the client computed (ptCost,
// which ComsumePt takes off) and the server computes again.
func (game *Game) handleCityRebuildBuff(client *connection.Client, req *protobuf.CS_26066) error {
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		group := req.GetGroup()
		cost, ok := city.buffCost(group, req.GetCount())
		if !ok {
			return errRefused
		}
		if city.row.Pt < cost {
			return ErrNotEnough
		}
		city.row.Pt -= cost
		tx.event(TaskEvent{SubType: TaskSubCityPtSpent, Count: uint32(min(cost, math.MaxUint32))})
		level := city.buff(group) + req.GetCount()
		city.levels[group] = level
		// the wave fought restarts from the HP it has left, now hit
		// harder (or weakened)
		city.cityEvents(tx)
		return tx.SetCityRebuildBuff(tx.ctx, db.SetCityRebuildBuffParams{CommanderID: tx.commanderID, ActivityID: req.GetActId(), BuffGroup: group, Level: level})
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_26067{Result: result(err), Adjust: city.adjust()})
}

// handleCityRebuildResult answers CityRebuildCommand RESULT (CS_26068, the
// defense's collect button): the koban the fight earned move to the city
// (summary_pt, which CityRebuildData:Result adds), along with the first
// defeat rewards (activity_ninja_enemy first_drop_show) of the waves
// defeated since the last collect (award_list, which the client adds as
// drops: the koban in it through Drop's virtual_type 102). Giving those at
// collect time rather than at the kill is a GUESS (the summary is the only
// reply carrying drops); so is counting a collect that got nothing as no
// pick up for the "Pick up rewards" tasks (the client shows
// ninja_game_cant_pickup instead of a reward window). The enemies' own drop
// (activity_ninja_enemy.drop, a server-side drop table) is not known: kills
// only give their koban.
func (game *Game) handleCityRebuildResult(client *connection.Client, req *protobuf.CS_26068) error {
	var collected int64
	var awards []*protobuf.DROPINFO
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		row := &city.row
		collected = row.SummaryPt
		row.Pt += collected
		row.SummaryPt = 0
		for level := row.AwardedLevel + 1; level <= row.MaxLevel; level++ {
			awards = append(awards, configDrops(cityRebuild.enemies[level].FirstDropShow)...)
		}
		row.AwardedLevel = max(row.AwardedLevel, row.MaxLevel)
		if collected > 0 || len(awards) > 0 {
			tx.event(TaskEvent{SubType: TaskSubCityCollect, Count: 1})
		}
		// stored first: the koban among the awards are added to it
		if err := city.save(tx.ctx, tx.Queries); err != nil {
			return err
		}
		if err := tx.grantDrops(awards); err != nil {
			return err
		}
		reloaded, err := tx.GetCityRebuild(tx.ctx, db.GetCityRebuildParams{CommanderID: tx.commanderID, ActivityID: req.GetActId()})
		city.row = reloaded
		return err
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	if err != nil {
		collected, awards = 0, nil
	}
	return client.Send(&protobuf.SC_26069{Result: result(err), Summary: &protobuf.NINJA_SUMMARY{
		SummaryPt: ninjaPt(collected),
		AwardList: awards,
		Adjust:    city.adjust(),
	}})
}

// handleCityRebuildLevel answers CityRebuildCommand CHOOSE_LEVEL (CS_26070,
// the defense's previous / next wave buttons): any wave up to the one after
// the highest defeated (canChangeNextLevel), fought from full HP now.
func (game *Game) handleCityRebuildLevel(client *connection.Client, req *protobuf.CS_26070) error {
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		level := req.GetLevel()
		row := &city.row
		if _, ok := cityRebuild.enemies[level]; !ok || level > row.MaxLevel+1 || level > cityRebuild.lastEnemy {
			return errRefused
		}
		row.CurLevel, row.MaxDisplay, row.LeftHp = level, max(row.MaxDisplay, level), 0
		if row.FightTime != 0 {
			row.FightTime = now
		}
		city.cityEvents(tx)
		return nil
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_26071{Result: result(err), Adjust: city.adjust()})
}

// handleCityRebuildStart answers CityRebuildCommand INIT_TIME (CS_26072),
// sent when the defense opens while NINJA_ADJUST.time is 0: the fight
// starts now, the wave at full HP. A fight already started is left as is.
func (game *Game) handleCityRebuildStart(client *connection.Client, req *protobuf.CS_26072) error {
	city, err := game.cityTx(client, req.GetActId(), func(tx *Tx, city *cityState, now int64) error {
		if city.row.FightTime == 0 {
			city.row.FightTime, city.row.LeftHp = now, 0
		}
		return nil
	})
	if err := cityReplyErr(err); err != nil {
		return err
	}
	reply := &protobuf.SC_26073{Result: result(err)}
	if city != nil {
		reply.Adjust = city.adjust()
	}
	return client.Send(reply)
}

// addCityRebuildPt adds a City Rebuild activity's koban (Drop's
// virtual_type 102, CityRebuildProxy:AddPt) to the commander's city.
func (tx *Tx) addCityRebuildPt(activityID, count uint32) error {
	if err := tx.CreateCityRebuild(tx.ctx, db.CreateCityRebuildParams{CommanderID: tx.commanderID, ActivityID: activityID}); err != nil {
		return err
	}
	return tx.AddCityRebuildPt(tx.ctx, db.AddCityRebuildPtParams{Pt: int64(count), CommanderID: tx.commanderID, ActivityID: activityID})
}

// cityRebuildBag is a City Rebuild activity's bag (VirtualBagActivity,
// ACTIVITYINFO date1_key_value_list[1]): the count of each virtual item
// linked to it, the Repair Tools and Lord's Orders the facilities and troops
// cost.
func cityRebuildBag(ctx context.Context, queries *db.Queries, commanderID, activityID uint32) ([]*protobuf.KEYVALUELIST_P11, error) {
	bag := &protobuf.KEYVALUELIST_P11{Key: proto.Uint32(1)}
	for _, id := range cityRebuild.bagItems[activityID] {
		amount, err := queries.GetVirtualItem(ctx, db.GetVirtualItemParams{CommanderID: commanderID, ItemID: id})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		bag.ValueList = append(bag.ValueList, &protobuf.KEYVALUE_P11{Key: proto.Uint32(id), Value: proto.Uint32(amount)})
	}
	return []*protobuf.KEYVALUELIST_P11{bag}, nil
}
