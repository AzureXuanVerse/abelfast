package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Operation Siren's saved state. Nothing of it was captured (the official
// account never reached the mode): its shape is what the client's models
// read (model/proxy/worldproxy.lua NetFullUpdate / NetUpdateWorld,
// mod/experiment/world/model/*.lua), every field of SC_33001 / WORLDINFO /
// MAPINFO with a home here so the later handlers (map operations CS_33103,
// ports, items, tasks, achievements) only add logic.
//
// A world is a worldState (world_states, one per commander) plus a
// worldMap per map it has been on (world_maps), both stored as JSON.

// worldState is a commander's world.
type worldState struct {
	// ActivateTime is WORLDINFO.time (World.activateTime), when the world
	// was last activated; 0 for a world never activated.
	ActivateTime uint32 `json:"activate_time,omitempty"`
	// ExpiredTime is last_change_group_timestamp (World.expiredTime):
	// when the monthly reset is due (World:CheckReset).
	ExpiredTime uint32 `json:"expired_time,omitempty"`
	// Round is WORLDINFO.round (World.roundIndex), the turn of the active
	// map: even the player's, odd the enemies' (World:GetRound).
	Round uint32 `json:"round,omitempty"`
	// Camp is the realm (SC_33001.camp, CS_33101.camp; World:GetRealm is 1).
	Camp uint32 `json:"camp,omitempty"`
	// MapID is the active map's random id (WORLDINFO.map_id), 0 while the
	// world isn't activated; it is always in Maps.
	MapID uint32 `json:"map_id,omitempty"`
	// EnterMapID is the active entrance (WORLDINFO.enter_map_id,
	// world_chapter_colormask).
	EnterMapID uint32 `json:"enter_map_id,omitempty"`
	// Submarine is submarine_state: the submarine fleet supports the
	// active map (World.submarineSupport).
	Submarine bool `json:"submarine,omitempty"`

	Stamina worldStamina `json:"stamina"`

	// Fleets are the fleets deployed (WORLDINFO.group_list), by id.
	Fleets []*worldFleet `json:"fleets,omitempty"`
	// DefaultFleets are the formation the fleet selection starts from
	// (SC_33001.fleet_list, World.defaultFleets): the last one deployed,
	// kept through resets.
	DefaultFleets []worldBaseFleet `json:"default_fleets,omitempty"`

	// Maps are the atlas' maps in play (WORLDINFO.chapter_list, the
	// WorldAtlas cost maps): each random id with the template drawn for it.
	Maps []worldMapID `json:"maps,omitempty"`
	// SairenChapters are the entrances turned into Siren zones
	// (sairen_chapter, WorldAtlas:SetSairenEntranceList).
	SairenChapters []uint32 `json:"sairen_chapters,omitempty"`
	// CleanChapters are the maps secured (SC_33001.clean_chapter,
	// WorldAtlas:SetPressingMarkList).
	CleanChapters []uint32 `json:"clean_chapters,omitempty"`
	// ChapterAwards are the zone control rewards (chapter_award,
	// World.pressingAwardDic, by entrance): flag 1 while not taken.
	ChapterAwards []worldChapterAward `json:"chapter_awards,omitempty"`
	// AutoMaps are random_map_id_list_by_auto, the maps secured by
	// auto-search (WorldAtlas:SetDelegatedMarkList).
	AutoMaps []uint32 `json:"auto_maps,omitempty"`

	// Items are the world inventory (WORLDINFO.item_list,
	// WorldInventoryProxy, world_item_data_template).
	Items []worldCount `json:"items,omitempty"`
	// Tasks are the world tasks (task_list, WorldTaskProxy,
	// world_task_data); TaskFinishCount counts the ones submitted.
	Tasks           []worldTask `json:"tasks,omitempty"`
	TaskFinishCount uint32      `json:"task_finish_count,omitempty"`
	// Daily is the day's daily task offer (worldtask.go).
	Daily worldDaily `json:"daily"`

	// Goods are WORLDINFO.goods_list, the new port shop's purchases
	// (world_newshop_data, WorldAtlas:InitWorldNShopGoods).
	Goods []worldCount `json:"goods,omitempty"`
	// OutShopBuys are SC_33001.out_shop_buy_list, the purchases in the
	// world shops outside the ports (shop_template, World:UpdateWorldShopGoods).
	OutShopBuys []worldCount `json:"out_shop_buys,omitempty"`
	// PortMarks / NewPortMarks are port_list / new_flag_port_list (the ports
	// with goods / new goods, WorldAtlas:SetPortMarkList).
	PortMarks    []uint32 `json:"port_marks,omitempty"`
	NewPortMarks []uint32 `json:"new_port_marks,omitempty"`
	// Ports are each port's offer (SC_33402 PORT_INFO).
	Ports []*worldPortState `json:"ports,omitempty"`

	// Targets are the achievements' progress (target_list, WorldAchievement,
	// world_target_data); TargetFetches the stars claimed per entrance
	// (target_fetch_list).
	Targets       []worldTarget      `json:"targets,omitempty"`
	TargetFetches []worldTargetFetch `json:"target_fetches,omitempty"`

	Count worldCountInfo `json:"count"`

	// CDs are the orders' cooldowns (cd_list: WorldConst.OpReq* id, when it
	// was last used).
	CDs []worldCount `json:"cds,omitempty"`
	// Buffs are the global buffs (WORLDINFO.buff_list, World.globalBuffDic).
	Buffs []worldBuff `json:"buffs,omitempty"`
	// GlobalFlags are SC_33001.global_flag_list (World.gobalFlag by index,
	// 1 treasure_flag).
	GlobalFlags []uint32 `json:"global_flags,omitempty"`
	// MonthBoss are month_boss, the lowest hp each periodic boss was left
	// at (World.lowestHP, key enemy id).
	MonthBoss []worldCount `json:"month_boss,omitempty"`
	// Arbiter is the Arbiters' months (worldarbiter.go).
	Arbiter worldArbiter `json:"arbiter"`
	// Strongholds is the Siren Strongholds' spawning (worldstronghold.go).
	Strongholds worldStrongholds `json:"strongholds"`
}

// worldStamina is the world's action points (WorldStaminaManager).
type worldStamina struct {
	Power       uint32 `json:"power"`        // action_power, recovers to the max
	Extra       uint32 `json:"extra"`        // action_power_extra, above the max
	LastRecover uint32 `json:"last_recover"` // last_recover_timestamp
	// Exchanges are the oil exchanges of the week starting at
	// ExchangeWeek (staminaExchangeTimes, reset every Monday).
	Exchanges    uint32 `json:"exchanges"`
	ExchangeWeek int64  `json:"exchange_week"`
}

// worldFleet is a deployed fleet (GROUPINCHAPTER_P33, WorldMapFleet).
type worldFleet struct {
	ID         uint32      `json:"id"`
	Ships      []worldShip `json:"ships"`
	Submarine  bool        `json:"submarine,omitempty"` // its ships are submarines (FleetType.Submarine)
	Row        uint32      `json:"row"`
	Column     uint32      `json:"column"`
	StartRow   uint32      `json:"start_row"`
	StartCol   uint32      `json:"start_column"`
	LossFlag   uint32      `json:"loss_flag,omitempty"`
	Bullet     uint32      `json:"bullet,omitempty"`
	BulletMax  uint32      `json:"bullet_max,omitempty"`
	Damage     uint32      `json:"damage_level,omitempty"` // morale damage, WorldConst.DamageBuffList index
	KillCount  uint32      `json:"kill_count,omitempty"`
	Carries    []uint32    `json:"carries,omitempty"` // attach_list, world_carry_item ids
	Buffs      []worldBuff `json:"buffs,omitempty"`
	Commanders []worldPos  `json:"commanders,omitempty"` // meowfficers {pos, id}

	BoxStrategies  []worldCount `json:"box_strategies,omitempty"`
	ShipStrategies []worldCount `json:"ship_strategies,omitempty"`
	StrategyIDs    []uint32     `json:"strategy_ids,omitempty"`

	// Salvage is cmd_collection, the meowfficer salvage under way.
	Salvage worldSalvage `json:"salvage"`
}

// worldShip is a ship of a deployed fleet (SHIPINCHAPTER_P33).
type worldShip struct {
	ID     uint32      `json:"id"`
	HPRant uint32      `json:"hp_rant"` // hp in 1/10000
	Buffs  []worldBuff `json:"buffs,omitempty"`
}

// worldBaseFleet is a fleet of the default formation (FLEETINFO).
type worldBaseFleet struct {
	ID         uint32     `json:"id"`
	Ships      []uint32   `json:"ships"`
	Commanders []worldPos `json:"commanders,omitempty"`
}

// worldPos is a {pos, id} pair (COMMANDERSINFO).
type worldPos struct {
	Pos uint32 `json:"pos"`
	ID  uint32 `json:"id"`
}

// worldCount is an {id, count} pair: an item, goods bought, a cooldown
// (id, time), a strategy, a KVDATA.
type worldCount struct {
	ID    uint32 `json:"id"`
	Count uint32 `json:"count"`
}

// worldBuff is a BUFF_INFO (world_slgbuff_data).
type worldBuff struct {
	ID        uint32 `json:"id"`
	Stack     uint32 `json:"stack"`
	Timestamp uint32 `json:"timestamp,omitempty"`
	Round     uint32 `json:"round,omitempty"`
	Step      uint32 `json:"step,omitempty"`
}

// worldSalvage is a CMDCOLLECTION.
type worldSalvage struct {
	Progress uint32   `json:"progress,omitempty"`
	List     []uint32 `json:"list,omitempty"`
	RandomID uint32   `json:"random_id,omitempty"`
}

// worldMapID is a WORLDMAPID.
type worldMapID struct {
	RandomID   uint32 `json:"random_id"`
	TemplateID uint32 `json:"template_id"`
}

// worldChapterAward is a CHAPTERAWARDINFO.
type worldChapterAward struct {
	ID    uint32 `json:"id"`
	Award uint32 `json:"award"`
	Flag  uint32 `json:"flag"`
}

// worldTask is a TASK_INFO.
type worldTask struct {
	ID         uint32 `json:"id"`
	Progress   uint32 `json:"progress"`
	AcceptTime uint32 `json:"accept_time"`
	SubmitTime uint32 `json:"submit_time"`
	EventMapID uint32 `json:"event_map_id"`
}

// worldPortState is a port's offer (PORT_INFO): its tasks and goods
// (world_goods_data) until NextRefresh.
type worldPortState struct {
	PortID      uint32       `json:"port_id"`
	Tasks       []uint32     `json:"tasks,omitempty"`
	Goods       []worldCount `json:"goods,omitempty"`
	NextRefresh uint32       `json:"next_refresh"`
	// Bought are the goods of the port's pool bought since the monthly
	// reset (worldPortPools), never offered again until it
	Bought []uint32 `json:"bought,omitempty"`
	// MapID is the map a temporary port's offer was drawn on
	MapID uint32 `json:"map_id,omitempty"`
}

// worldTarget is a WORLDTARGET: an achievement's trigger counts.
type worldTarget struct {
	ID      uint32       `json:"id"`
	Process []worldCount `json:"process"` // {trigger_id, count}
}

// worldTargetFetch is a WORLDTARGET_FETCH.
type worldTargetFetch struct {
	ID    uint32   `json:"id"`
	Stars []uint32 `json:"stars"`
}

// worldCountInfo is COUNTINFO.
type worldCountInfo struct {
	Step     uint32 `json:"step,omitempty"`     // stepCount, cells moved
	Treasure uint32 `json:"treasure,omitempty"` // treasureCount
	// Progress is task_progress, the story progress (World.progress,
	// world_stage_template.stage_key, the maps' open_stage)
	Progress    uint32   `json:"progress,omitempty"`
	Activations uint32   `json:"activations,omitempty"` // activate_count
	Collections []uint32 `json:"collections,omitempty"` // collection_list
}

// activated tells whether the world has an active map (World:IsActivate).
func (state *worldState) activated() bool {
	return state.MapID != 0
}

// mapTemplate is the template the world's map randomID is played on, 0
// when it isn't one of its maps.
func (state *worldState) mapTemplate(randomID uint32) uint32 {
	for _, m := range state.Maps {
		if m.RandomID == randomID {
			return m.TemplateID
		}
	}
	return 0
}

// fleet is the deployed fleet id, nil for none.
func (state *worldState) fleet(id uint32) *worldFleet {
	for _, fleet := range state.Fleets {
		if fleet.ID == id {
			return fleet
		}
	}
	return nil
}

// shipIDs are the ships of the deployed fleets (SC_33114.ship_id_list).
func (state *worldState) shipIDs() []uint32 {
	var ids []uint32
	for _, fleet := range state.Fleets {
		for _, ship := range fleet.Ships {
			ids = append(ids, ship.ID)
		}
	}
	return ids
}

// commanderIDs are the meowfficers of the deployed fleets
// (SC_33114.cmd_id_list).
func (state *worldState) commanderIDs() []uint32 {
	var ids []uint32
	for _, fleet := range state.Fleets {
		for _, commander := range fleet.Commanders {
			ids = append(ids, commander.ID)
		}
	}
	return ids
}

// dropTypeWorldItem is DROP_TYPE_WORLD_ITEM: a world_item_data_template
// item, kept in the world inventory (Items), not the bag.
const dropTypeWorldItem = 12

// item is the count of a world item held.
func (state *worldState) item(id uint32) uint32 {
	for _, item := range state.Items {
		if item.ID == id {
			return item.Count
		}
	}
	return 0
}

// addItem changes a world item's count by delta (never below 0), dropping
// it at 0.
func (state *worldState) addItem(id uint32, delta int64) {
	for i, item := range state.Items {
		if item.ID == id {
			count := max(int64(item.Count)+delta, 0)
			if count == 0 {
				state.Items = slices.Delete(state.Items, i, i+1)
			} else {
				state.Items[i].Count = uint32(count)
			}
			return
		}
	}
	if delta > 0 {
		state.Items = append(state.Items, worldCount{ID: id, Count: uint32(delta)})
	}
}

// recover is WorldStaminaManager:UpdateStamina at now: a point
// every world_movepower_recovery_interval since LastRecover, up to the max;
// LastRecover moves on by the intervals elapsed even at the max, as the
// client's does. The weekly oil exchanges start again every Monday
// (WorldProxy's DayCall on server week day 1).
func (stamina *worldStamina) recover(now time.Time) {
	interval := int64(worldSettings.staminaInterval)
	if elapsed := now.Unix() - int64(stamina.LastRecover); stamina.LastRecover != 0 && elapsed >= interval {
		points := elapsed / interval
		stamina.LastRecover += uint32(points * interval)
		if stamina.Power < worldSettings.maxStamina {
			stamina.Power = uint32(min(int64(stamina.Power)+points, int64(worldSettings.maxStamina)))
		}
	}
	if start := periodStart(now, week).Unix(); stamina.ExchangeWeek != start {
		stamina.ExchangeWeek = start
		stamina.Exchanges = 0
	}
}

// total is WorldStaminaManager:GetTotalStamina.
func (stamina *worldStamina) total() uint32 {
	return stamina.Power + stamina.Extra
}

// consume is WorldStaminaManager:ConsumeStamina: the extra points go
// first; false (nothing taken) when there aren't enough.
func (stamina *worldStamina) consume(points uint32) bool {
	if stamina.total() < points {
		return false
	}
	if stamina.Extra >= points {
		stamina.Extra -= points
		return true
	}
	stamina.Power -= points - stamina.Extra
	stamina.Extra = 0
	return true
}

// plus is WorldStaminaManager:PlusStamina: what goes over the max is extra.
func (stamina *worldStamina) plus(points uint32) {
	total := stamina.total() + points
	stamina.Power = min(total, worldSettings.maxStamina)
	stamina.Extra = total - stamina.Power
}

// loadWorldState reads a commander's world, a world never activated when
// there is none; its stamina is recovered to now.
func loadWorldState(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*worldState, error) {
	state := &worldState{}
	raw, err := queries.GetWorldState(ctx, commanderID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(raw, state); err != nil {
			return nil, err
		}
	}
	state.Stamina.recover(now)
	return state, nil
}

// worldState is the commander's world in tx (loadWorldState).
func (tx *Tx) worldState(now time.Time) (*worldState, error) {
	return loadWorldState(tx.ctx, tx.Queries, tx.commanderID, now)
}

// saveWorldState stores the commander's world.
func (tx *Tx) saveWorldState(state *worldState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return tx.SaveWorldState(tx.ctx, db.SaveWorldStateParams{CommanderID: tx.commanderID, State: raw})
}

// clearWorld forgets the commander's world and its maps.
func (tx *Tx) clearWorld() error {
	if err := tx.DeleteWorldMaps(tx.ctx, tx.commanderID); err != nil {
		return err
	}
	return tx.DeleteWorldState(tx.ctx, tx.commanderID)
}

// The protocol views of the state.

func worldCellPos(row, column uint32) *protobuf.CHAPTERCELLPOS_P33 {
	return &protobuf.CHAPTERCELLPOS_P33{Row: proto.Uint32(row), Column: proto.Uint32(column)}
}

func worldBuffInfos(buffs []worldBuff) []*protobuf.BUFF_INFO {
	var infos []*protobuf.BUFF_INFO
	for _, buff := range buffs {
		infos = append(infos, &protobuf.BUFF_INFO{
			Id:        proto.Uint32(buff.ID),
			Stack:     proto.Uint32(buff.Stack),
			Timestamp: proto.Uint32(buff.Timestamp),
			Round:     proto.Uint32(buff.Round),
			Step:      proto.Uint32(buff.Step),
		})
	}
	return infos
}

func worldCommanderInfos(commanders []worldPos) []*protobuf.COMMANDERSINFO {
	var infos []*protobuf.COMMANDERSINFO
	for _, commander := range commanders {
		infos = append(infos, &protobuf.COMMANDERSINFO{Pos: proto.Uint32(commander.Pos), Id: proto.Uint32(commander.ID)})
	}
	return infos
}

func worldStrategyInfos(strategies []worldCount) []*protobuf.STRATEGYINFO_P33 {
	var infos []*protobuf.STRATEGYINFO_P33
	for _, strategy := range strategies {
		infos = append(infos, &protobuf.STRATEGYINFO_P33{Id: proto.Uint32(strategy.ID), Count: proto.Uint32(strategy.Count)})
	}
	return infos
}

func worldGoodsInfos(goods []worldCount) []*protobuf.GOODS_INFO_P33 {
	var infos []*protobuf.GOODS_INFO_P33
	for _, g := range goods {
		infos = append(infos, &protobuf.GOODS_INFO_P33{GoodsId: proto.Uint32(g.ID), Count: proto.Uint32(g.Count)})
	}
	return infos
}

// shipInfo is the ship as SHIPINCHAPTER_P33 (33104 ship_update reuses it).
func (ship worldShip) shipInfo() *protobuf.SHIPINCHAPTER_P33 {
	return &protobuf.SHIPINCHAPTER_P33{Id: proto.Uint32(ship.ID), HpRant: proto.Uint32(ship.HPRant), BuffList: worldBuffInfos(ship.Buffs)}
}

// groupInfo is the fleet as GROUPINCHAPTER_P33 (WorldMapFleet:Setup reads
// cmd_collection unconditionally, so it is always there).
func (fleet *worldFleet) groupInfo() *protobuf.GROUPINCHAPTER_P33 {
	group := &protobuf.GROUPINCHAPTER_P33{
		Id:               proto.Uint32(fleet.ID),
		Pos:              worldCellPos(fleet.Row, fleet.Column),
		LossFlag:         proto.Uint32(fleet.LossFlag),
		BoxStrategyList:  worldStrategyInfos(fleet.BoxStrategies),
		ShipStrategyList: worldStrategyInfos(fleet.ShipStrategies),
		StrategyIds:      fleet.StrategyIDs,
		Bullet:           proto.Uint32(fleet.Bullet),
		StartPos:         worldCellPos(fleet.StartRow, fleet.StartCol),
		AttachList:       fleet.Carries,
		DamageLevel:      proto.Uint32(fleet.Damage),
		BuffList:         worldBuffInfos(fleet.Buffs),
		CommanderList:    worldCommanderInfos(fleet.Commanders),
		KillCount:        proto.Uint32(fleet.KillCount),
		CmdCollection: &protobuf.CMDCOLLECTION{
			Progress:     proto.Uint32(fleet.Salvage.Progress),
			ProgressList: fleet.Salvage.List,
			RandomId:     proto.Uint32(fleet.Salvage.RandomID),
		},
		BulletMax: proto.Uint32(fleet.BulletMax),
	}
	for _, ship := range fleet.Ships {
		group.ShipList = append(group.ShipList, ship.shipInfo())
	}
	return group
}

// worldInfo is the world as WORLDINFO (WorldProxy:NetUpdateWorld). A world
// never activated has map_id 0 and no maps, fleets or entrance: the base
// state the client starts the fleet selection from.
func (state *worldState) worldInfo() *protobuf.WORLDINFO {
	info := &protobuf.WORLDINFO{
		MapId:                proto.Uint32(state.MapID),
		Time:                 proto.Uint32(state.ActivateTime),
		Round:                proto.Uint32(state.Round),
		TaskFinishCount:      proto.Uint32(state.TaskFinishCount),
		SubmarineState:       proto.Uint32(0),
		GoodsList:            worldGoodsInfos(state.Goods),
		ActionPower:          proto.Uint32(state.Stamina.Power),
		ActionPowerExtra:     proto.Uint32(state.Stamina.Extra),
		LastRecoverTimestamp: proto.Uint32(state.Stamina.LastRecover),
		// WorldStaminaManager:Setup reads it as staminaExchangeTimes
		ActionPowerFetchCount:    proto.Uint32(state.Stamina.Exchanges),
		LastChangeGroupTimestamp: proto.Uint32(state.ExpiredTime),
		EnterMapId:               proto.Uint32(state.EnterMapID),
		BuffList:                 worldBuffInfos(state.Buffs),
		SairenChapter:            state.SairenChapters,
		RandomMapIdListByAuto:    state.AutoMaps,
	}
	if state.Submarine {
		info.SubmarineState = proto.Uint32(1)
	}
	for _, fleet := range state.Fleets {
		info.GroupList = append(info.GroupList, fleet.groupInfo())
	}
	for _, task := range state.Tasks {
		info.TaskList = append(info.TaskList, &protobuf.TASK_INFO{
			Id:          proto.Uint32(task.ID),
			Progress:    proto.Uint32(task.Progress),
			AcceptTime:  proto.Uint32(task.AcceptTime),
			SubmiteTime: proto.Uint32(task.SubmitTime),
			EventMapId:  proto.Uint32(task.EventMapID),
		})
	}
	for _, item := range state.Items {
		info.ItemList = append(info.ItemList, &protobuf.WORLD_ITEM_INFO{Id: proto.Uint32(item.ID), Count: proto.Uint32(item.Count)})
	}
	for _, cd := range state.CDs {
		info.CdList = append(info.CdList, &protobuf.IDTIMEINFO{Id: proto.Uint32(cd.ID), Time: proto.Uint32(cd.Count)})
	}
	for _, m := range state.Maps {
		info.ChapterList = append(info.ChapterList, &protobuf.WORLDMAPID{RandomId: proto.Uint32(m.RandomID), TemplateId: proto.Uint32(m.TemplateID)})
	}
	for _, kv := range state.MonthBoss {
		info.MonthBoss = append(info.MonthBoss, &protobuf.KVDATA{Key: proto.Uint32(kv.ID), Value: proto.Uint32(kv.Count)})
	}
	return info
}

// countInfo is COUNTINFO.
func (state *worldState) countInfo() *protobuf.COUNTINFO {
	return &protobuf.COUNTINFO{
		StepCount:      proto.Uint32(state.Count.Step),
		TreasureCount:  proto.Uint32(state.Count.Treasure),
		TaskProgress:   proto.Uint32(state.Count.Progress),
		ActivateCount:  proto.Uint32(state.Count.Activations),
		CollectionList: state.Count.Collections,
	}
}

func (state *worldState) chapterAwards() []*protobuf.CHAPTERAWARDINFO {
	var awards []*protobuf.CHAPTERAWARDINFO
	for _, award := range state.ChapterAwards {
		awards = append(awards, &protobuf.CHAPTERAWARDINFO{Id: proto.Uint32(award.ID), Award: proto.Uint32(award.Award), Flag: proto.Uint32(award.Flag)})
	}
	return awards
}

// fullState is SC_33001 (WorldProxy:NetFullUpdate).
func (state *worldState) fullState() *protobuf.SC_33001 {
	reply := &protobuf.SC_33001{
		World:           state.worldInfo(),
		IsWorldOpen:     proto.Uint32(1),
		PortList:        state.PortMarks,
		Camp:            proto.Uint32(state.Camp),
		CountInfo:       state.countInfo(),
		OutShopBuyList:  worldGoodsInfos(state.OutShopBuys),
		CleanChapter:    state.CleanChapters,
		ChapterAward:    state.chapterAwards(),
		NewFlagPortList: state.NewPortMarks,
		GlobalFlagList:  state.GlobalFlags,
	}
	for _, target := range state.Targets {
		info := &protobuf.WORLDTARGET{Id: proto.Uint32(target.ID)}
		for _, process := range target.Process {
			info.ProcessList = append(info.ProcessList, &protobuf.WORLDTARGET_PROCESS{TriggerId: proto.Uint32(process.ID), Count: proto.Uint32(process.Count)})
		}
		reply.TargetList = append(reply.TargetList, info)
	}
	for _, fetch := range state.TargetFetches {
		reply.TargetFetchList = append(reply.TargetFetchList, &protobuf.WORLDTARGET_FETCH{Id: proto.Uint32(fetch.ID), StarList: fetch.Stars})
	}
	for _, fleet := range state.DefaultFleets {
		reply.FleetList = append(reply.FleetList, &protobuf.FLEETINFO{
			Id:         proto.Uint32(fleet.ID),
			ShipList:   fleet.Ships,
			Commanders: worldCommanderInfos(fleet.Commanders),
		})
	}
	return reply
}

// baseState is SC_33114, the world as the client knows it before it asks
// for the rest (World.TypeBase: the ships and meowfficers deployed, flagged
// "inWorld", and the progress).
func (state *worldState) baseState() *protobuf.SC_33114 {
	return &protobuf.SC_33114{
		IsWorldOpen: proto.Uint32(1),
		ShipIdList:  state.shipIDs(),
		CmdIdList:   state.commanderIDs(),
		Progress:    proto.Uint32(state.Count.Progress),
	}
}
