package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Project Identity, the newer child-raising game (NewEducateProxy, protocol
// module 29; Oceana and Explorer, the child2_data characters). A game runs
// for the character's rounds (child2_round, then endless rounds), each round
// driven by an FSM the client mirrors (NewEducateFSM, model/vo/NewEducate/
// FSM): main event, talent pick, tarot choice, then the free part (map,
// topic, mind), the schedule (plans), the assessment and the next round,
// and the endings after the last round. The content plays as node chains
// (child2_node) the client walks with CS_29030.
//
// The client applies every drop the server sends (NewEducateDropHelper.
// HandleDrops) and computes costs itself, so the server keeps the same state
// and applies the same drops in the same order. What the official server
// gives at a chain's drop node (child2_node type 102) is in no table:
// belfast gives the drops the client previews for what started the chain
// (plan result_display, site drop_display, favor_result_display), with the
// buffs' gain bonuses (child2_benefit effects 3 and 4, the mood one
// included) as benefit_drop; chains with no preview (main events, topics,
// random events, assessments) give nothing. The buffs' triggered effects
// (child2_benefit trigger / effect) follow the buff descriptions and the
// wiki (Project Identity: Oceana / Explorer); the condition types the client
// doesn't know (child2_condition type > 5) are guessed in neCondition.

// FSM systems (NewEducateFSM.SYSTEM)
const (
	neSysInit   = 0
	neSysEvent  = 1
	neSysTalent = 2
	neSysTopic  = 3
	neSysMap    = 4
	neSysPlan   = 5
	neSysAssess = 6
	neSysPhase  = 7
	neSysEnding = 8
	neSysMind   = 9
	neSysChoose = 10

	// priority states (NewEducatePriorityFSM.SYSTEM)
	neSysPickChoice = 100
	neSysUpEntry    = 101
)

// neBuff is an active buff (TBBF).
type neBuff struct {
	ID      uint32 `json:"id"`
	Round   uint32 `json:"round"`
	Pending bool   `json:"pending,omitempty"`
}

// nePriority is a priority state (TBFSM in priority_fsm): a choice among
// cards / readings (system 100) or a reading to level up (101).
type nePriority struct {
	System   uint32   `json:"system"`
	Pool     uint32   `json:"pool,omitempty"` // what the choice draws from, 0 the round's tarots
	Selects  []uint32 `json:"selects,omitempty"`
	Rerolls  []uint32 `json:"rerolls,omitempty"`
	FromShop bool     `json:"from_shop,omitempty"`
}

// neSiteCache is the round's map (TBFSMCACHESITE).
type neSiteCache struct {
	Events   []uint32          `json:"events"`
	Shops    []uint32          `json:"shops"`
	Buys     map[uint32]uint32 `json:"buys"`
	StateKey uint32            `json:"state_key"`
	StateVal uint32            `json:"state_value"`
	Selected []uint32          `json:"selected"`
	Refresh  uint32            `json:"refresh"`
}

// neState is a running game (TBINFO, its FSM and caches) and what only the
// server keeps of it.
type neState struct {
	ID          uint32            `json:"id"`
	Difficulty  uint32            `json:"difficulty"`
	Round       uint32            `json:"round"`
	InTemp      bool              `json:"in_temp,omitempty"`
	TempRounds  uint32            `json:"temp_rounds,omitempty"`
	EvalFail    bool              `json:"eval_fail,omitempty"`
	Attrs       map[uint32]int64  `json:"attrs"`
	Res         map[uint32]int64  `json:"res"`
	Plans       map[uint32]uint32 `json:"plans"` // plan group -> upgraded plan
	SiteShips   []uint32          `json:"site_ships"`
	Works       []uint32          `json:"works"` // current work / travel site levels
	WorkCounter map[uint32]uint32 `json:"work_counter"`
	EventCount  map[uint32]uint32 `json:"event_counter"`
	Evaluations map[uint32]uint32 `json:"evaluations"` // round -> rank
	Name        string            `json:"name"`
	FavorLv     uint32            `json:"favor_lv"`
	Buffs       []neBuff          `json:"buffs"`

	// FSM
	System   uint32      `json:"system"`
	Node     uint32      `json:"node"`
	PlanSet  bool        `json:"plan_set,omitempty"`
	PlanCur  uint32      `json:"plan_cur,omitempty"`
	PlanList [][2]uint32 `json:"plan_list,omitempty"` // {slot, plan}
	Talent   *struct {
		Finished  bool     `json:"finished"`
		Talents   []uint32 `json:"talents"`
		Retalents []uint32 `json:"retalents"`
	} `json:"talent,omitempty"`
	Site *neSiteCache `json:"site,omitempty"`
	Chat *struct {
		Finished bool     `json:"finished"`
		Chats    []uint32 `json:"chats"`
	} `json:"chat,omitempty"`
	End *struct {
		Ends   []uint32 `json:"ends"`
		Select uint32   `json:"select"`
	} `json:"end,omitempty"`
	Mind         bool         `json:"mind,omitempty"`
	Eval         *bool        `json:"eval,omitempty"` // assessment done
	Priority     []nePriority `json:"priority,omitempty"`
	TarotSelects []uint32     `json:"tarot_selects,omitempty"`

	// server only
	Pending    []neDrop          `json:"pending,omitempty"`       // what the running chain's drop node gives
	PendingFor []neDrop          `json:"pending_bonus,omitempty"` // and its bonuses
	Chose      bool              `json:"chose,omitempty"`         // the round's tarot choice was offered
	ExtraDone  bool              `json:"extra_done,omitempty"`    // the schedule's end effects were given
	ShipLevels map[uint32]uint32 `json:"ship_levels"`             // shipgirl group -> current level id
	SeenChats  []uint32          `json:"seen_chats,omitempty"`
}

// nePermanent is what outlives a game (TBPERMANENT).
type nePermanent struct {
	Games     uint32   `json:"games"`
	Polaroids []uint32 `json:"polaroids"`
	Endings   []uint32 `json:"endings"`   // reached (activated)
	Finished  []uint32 `json:"finished"`  // selected
	Archive   []uint32 `json:"archive"`   // every buff ever owned
	MaxRound  uint32   `json:"max_round"` // highest round reached
}

// neGame is a character's game and permanent data, as loaded.
type neGame struct {
	*neState
	perm *nePermanent
}

func (state *neState) init() {
	for _, m := range []*map[uint32]int64{&state.Attrs, &state.Res} {
		if *m == nil {
			*m = map[uint32]int64{}
		}
	}
	for _, m := range []*map[uint32]uint32{&state.Plans, &state.WorkCounter, &state.EventCount, &state.Evaluations, &state.ShipLevels} {
		if *m == nil {
			*m = map[uint32]uint32{}
		}
	}
}

// newNeGame is a new game of a character, as captured for a new commander:
// every attribute and resource at its default value, the action points and
// shop refreshes of round 1, the mood buff active from round 1.
func newNeGame(char, difficulty uint32) *neState {
	state := &neState{ID: char, Difficulty: difficulty, Round: 1}
	state.init()
	for id, attr := range neAttrs {
		if attr.Character == char {
			state.Attrs[id] = attr.Default
		}
	}
	for id, res := range neResources {
		if res.Character == char {
			state.Res[id] = res.Default
		}
	}
	state.Buffs = []neBuff{{ID: neStartBuff(char), Round: 1}}
	state.startRound()
	return state
}

// startRound sets what a round starts with (NewEducateChar:OnNextRound).
func (state *neState) startRound() {
	round := state.roundConfig()
	if id := state.resOfType(neResAction); id != 0 {
		state.Res[id] = round.Mobility
	}
	if id := state.resOfType(neResRefreshShop); id != 0 {
		state.Res[id] = round.RefreshRefill
	}
	state.clamp()
}

// resource types (NewEducateChar.RES_TYPE)
const (
	neResMoney         = 1
	neResMood          = 2
	neResAction        = 3
	neResFavor         = 4
	neResRefreshShop   = 5
	neResRefreshChoice = 6
)

func (state *neState) resOfType(kind uint32) uint32 {
	for id := range state.Res {
		if neResources[id].Type == kind {
			return id
		}
	}
	return 0
}

func (state *neState) clamp() {
	for id, value := range state.Attrs {
		state.Attrs[id] = min(max(value, neAttrs[id].Min), neAttrs[id].Max)
	}
	for id, value := range state.Res {
		state.Res[id] = min(max(value, neResources[id].Min), neResources[id].Max)
	}
}

// roundID is the current round's child2_round id (NewEducateRound:InitRoundId).
func (state *neState) roundID() uint32 {
	normal := neRoundIDs(state.ID, state.Difficulty)
	if id, ok := normal[state.Round]; ok {
		return id
	}
	endless := neEndlessIDs(state.ID, state.Difficulty)
	if len(endless) == 0 || state.Round <= uint32(len(normal)) {
		return 0
	}
	idx := (state.Round - uint32(len(normal))) % uint32(len(endless))
	if idx == 0 {
		idx = uint32(len(endless))
	}
	return endless[idx-1]
}

func (state *neState) roundConfig() neRoundConfig { return neRounds[state.roundID()] }

func (state *neState) gameRounds() uint32 {
	return uint32(len(neRoundIDs(state.ID, state.Difficulty)))
}

func (state *neState) endless() bool { return state.Round > state.gameRounds() }

// lastRound tells whether the round is the game's last normal one
// (NewEducateRound:IsEndRound).
func (state *neState) lastRound() bool {
	_, next := neRoundIDs(state.ID, state.Difficulty)[state.Round+1]
	return !next
}

// tag is the character's personality tag (NewEducateChar:GetPersonalityTag).
func (state *neState) tag() string {
	char := neChars[state.ID]
	value := state.Attrs[state.personalityID()]
	for _, tag := range char.tags {
		if tag.min <= value && value <= tag.max {
			return tag.name
		}
	}
	if len(char.tags) > 0 {
		return char.tags[0].name
	}
	return ""
}

// attribute types (NewEducateChar.ATTR_TYPE)
const (
	neAttrStat        = 1
	neAttrPersonality = 2
)

func (state *neState) personalityID() uint32 {
	for id := range state.Attrs {
		if neAttrs[id].Type == neAttrPersonality {
			return id
		}
	}
	return 0
}

func (state *neState) attrSum() int64 {
	var sum int64
	for id, value := range state.Attrs {
		if neAttrs[id].Type == neAttrStat {
			sum += value
		}
	}
	return sum
}

func (state *neState) unlocked(system string) bool {
	round, ok := neChars[state.ID].unlock[system]
	return !ok || round <= state.Round
}

func (state *neState) buff(id uint32) *neBuff {
	for i := range state.Buffs {
		if state.Buffs[i].ID == id {
			return &state.Buffs[i]
		}
	}
	return nil
}

func (state *neState) tarot() uint32 {
	for _, buff := range state.Buffs {
		if config := neBuffs[buff.ID]; config.Type == neBuffTarot && config.visible {
			return buff.ID
		}
	}
	return 0
}

// immediate tells whether a buff gained now is active right away
// (NewEducateFSM:IsImmediateBenefit).
func (state *neState) immediate() bool {
	return state.System != neSysPlan && state.System != neSysAssess && state.System != neSysPhase
}

// assessRank is the assessment's rank (NewEducateChar:GetAssessRankIdx), 0
// when the round has none.
func (state *neState) assessRank() int {
	round := state.roundConfig()
	target, ok := neTargets[round.Target]
	if round.Target == 0 || state.InTemp || !ok || len(target.Levels) == 0 {
		return 0
	}
	factor := state.extraFactor()
	sum := float64(state.attrSum())
	for i := len(target.Levels); i >= 1; i-- {
		if sum >= float64(target.Levels[i-1])*factor {
			return i
		}
	}
	return len(target.Levels)
}

// extraFactor is the endless rounds' target factor
// (NewEducateRound:GetExtraFactor).
func (state *neState) extraFactor() float64 {
	if !state.endless() {
		return 1
	}
	base := 1.0
	var parsed float64
	if _, err := fmt.Sscan(state.roundConfig().EndlessFactor, &parsed); err == nil {
		base = parsed
	}
	factors := neEndlessFactors[state.Difficulty]
	for _, f := range factors {
		if f[0] <= float64(state.Round) && float64(state.Round) <= f[1] {
			return f[2] * base
		}
	}
	if len(factors) > 0 {
		return factors[len(factors)-1][2] * base
	}
	return base
}

// nextRound starts the next round (NewEducateChar:OnNextRound and the
// proxies it calls): the FSM caches go, a pending extra turn is played
// before the round number moves.
func (game *neGame) nextRound() {
	state := game.neState
	state.SiteShips = nil
	state.Node = 0
	state.PlanSet, state.PlanCur, state.PlanList = false, 0, nil
	state.Talent, state.Site, state.Chat, state.End, state.Eval = nil, nil, nil, nil, nil
	state.Mind = false
	state.Priority = nil
	state.TarotSelects = nil
	state.Pending, state.PendingFor = nil, nil
	state.Chose, state.ExtraDone = false, false
	if state.TempRounds > 0 {
		state.TempRounds--
		state.InTemp = true
	} else {
		state.InTemp = false
		state.Round++
	}
	state.startRound()
	kept := state.Buffs[:0]
	for _, buff := range state.Buffs {
		if buff.Pending {
			buff.Pending = false
			buff.Round = state.Round
		}
		if during := neBuffs[buff.ID].During; during != -1 && int64(state.Round) == int64(buff.Round)+during {
			continue
		}
		kept = append(kept, buff)
	}
	state.Buffs = kept
	game.perm.MaxRound = max(game.perm.MaxRound, state.Round)
}

// loadNeGame reads a character's game, nil when it was never requested.
func loadNeGame(ctx context.Context, queries *db.Queries, commanderID, char uint32) (*neGame, error) {
	row, err := queries.GetNewEducate(ctx, db.GetNewEducateParams{CommanderID: commanderID, CharacterID: char})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	game := &neGame{neState: &neState{}, perm: &nePermanent{}}
	if err := json.Unmarshal([]byte(row.State), game.neState); err != nil {
		return nil, fmt.Errorf("commander %d new educate %d: %w", commanderID, char, err)
	}
	if err := json.Unmarshal([]byte(row.Permanent), game.perm); err != nil {
		return nil, fmt.Errorf("commander %d new educate %d permanent: %w", commanderID, char, err)
	}
	game.init()
	return game, nil
}

func saveNeGame(ctx context.Context, queries *db.Queries, commanderID uint32, game *neGame) error {
	state, err := json.Marshal(game.neState)
	if err != nil {
		return err
	}
	perm, err := json.Marshal(game.perm)
	if err != nil {
		return err
	}
	return queries.SaveNewEducate(ctx, db.SaveNewEducateParams{CommanderID: commanderID, CharacterID: game.ID, State: string(state), Permanent: string(perm)})
}

// neTx runs fn on a character's game in a transaction and saves it when fn
// succeeds. A character the client never requested is refused
// (NewEducateProxy:ReqDataCheck requests them all first).
func (game *Game) neTx(client *connection.Client, char uint32, fn func(tx *Tx, ne *neGame) error) error {
	ctx := context.Background()
	return game.tx(ctx, client, func(tx *Tx) error {
		ne, err := loadNeGame(ctx, tx.Queries, client.CommanderID, char)
		if err != nil {
			return err
		}
		if ne == nil {
			return errRefused
		}
		if err := fn(tx, ne); err != nil {
			return err
		}
		return saveNeGame(ctx, tx.Queries, client.CommanderID, ne)
	})
}

// neReply runs a request on a character's game and answers with reply: an
// error that is only a refusal goes into the reply's result.
func (game *Game) neReply(client *connection.Client, char uint32, fn func(tx *Tx, ne *neGame) error, reply func(err error) proto.Message) error {
	err := game.neTx(client, char, fn)
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(reply(err))
}

func kvs(m map[uint32]uint32) []*protobuf.KVDATA {
	keys := make([]uint32, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	list := make([]*protobuf.KVDATA, 0, len(keys))
	for _, key := range keys {
		list = append(list, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(m[key])})
	}
	return list
}

func kvsSigned(m map[uint32]int64) []*protobuf.KVDATA {
	keys := make([]uint32, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	// the capture lists them from the highest id down
	slices.SortFunc(keys, func(a, b uint32) int { return int(b) - int(a) })
	list := make([]*protobuf.KVDATA, 0, len(keys))
	for _, key := range keys {
		list = append(list, &protobuf.KVDATA{Key: proto.Uint32(key), Value: proto.Uint32(uint32(max(m[key], 0)))})
	}
	return list
}

func b2u(b bool) *uint32 {
	if b {
		return proto.Uint32(1)
	}
	return proto.Uint32(0)
}

// res is the TBRES of the game.
func (state *neState) res() *protobuf.TBRES {
	return &protobuf.TBRES{Attrs: kvsSigned(state.Attrs), Resource: kvsSigned(state.Res)}
}

// fsm is the TBFSM of the game. NewEducateFSM reads cache[1] unconditionally.
func (state *neState) fsm() *protobuf.TBFSM {
	cache := &protobuf.TBFSMCACHE{}
	if state.PlanSet {
		plan := &protobuf.TBFSMCACHEPLAN{CurIndex: proto.Uint32(state.PlanCur)}
		for _, kv := range state.PlanList {
			plan.Plans = append(plan.Plans, &protobuf.KVDATA{Key: proto.Uint32(kv[0]), Value: proto.Uint32(kv[1])})
		}
		cache.CachePlan = []*protobuf.TBFSMCACHEPLAN{plan}
	}
	if t := state.Talent; t != nil {
		cache.CacheTalent = []*protobuf.TBFSMCACHETALENT{{Finished: b2u(t.Finished), Talents: t.Talents, Retalents: t.Retalents}}
	}
	if site := state.Site; site != nil {
		cache.CacheSite = []*protobuf.TBFSMCACHESITE{state.siteCache()}
	}
	if chat := state.Chat; chat != nil {
		cache.CacheChat = []*protobuf.TBFSMCACHECHAT{{Finished: b2u(chat.Finished), Chats: chat.Chats}}
	}
	if end := state.End; end != nil {
		cache.CacheEnd = []*protobuf.TBFSMCACHEEND{{Ends: end.Ends, Select: proto.Uint32(end.Select)}}
	}
	if state.Mind {
		cache.CacheMind = []*protobuf.TBFSMCACHEMIND{{}}
	}
	if state.Eval != nil {
		cache.CacheEval = []*protobuf.TBFSMCACHEEVAL{{IsFinished: b2u(*state.Eval)}}
	}
	fsm := &protobuf.TBFSM{
		SystemNo:     proto.Uint32(state.System),
		CurrentNode:  proto.Uint32(state.Node),
		Cache:        []*protobuf.TBFSMCACHE{cache},
		TarotSelects: state.TarotSelects,
	}
	for _, p := range state.Priority {
		pc := &protobuf.TBFSMCACHE{}
		switch p.System {
		case neSysPickChoice:
			pc.CacheNin1 = []*protobuf.TBFSMCACHENIN1{p.nin1()}
		case neSysUpEntry:
			pc.CacheAffixUp = []*protobuf.TBFSMCACHEAFFIXUP{{}}
		}
		fsm.PriorityFsm = append(fsm.PriorityFsm, &protobuf.TBFSM{SystemNo: proto.Uint32(p.System), CurrentNode: proto.Uint32(0), Cache: []*protobuf.TBFSMCACHE{pc}})
	}
	return fsm
}

func (p *nePriority) nin1() *protobuf.TBFSMCACHENIN1 {
	cache := &protobuf.TBFSMCACHENIN1{RerollCount: p.Rerolls, IsFromShop: b2u(p.FromShop)}
	for _, id := range p.Selects {
		cache.Selects = append(cache.Selects, &protobuf.TBDROP{Type: proto.Uint32(neDropBuff), Id: proto.Uint32(id), Number: proto.Int32(1)})
	}
	return cache
}

func (state *neState) siteCache() *protobuf.TBFSMCACHESITE {
	site := state.Site
	return &protobuf.TBFSMCACHESITE{
		Events:             site.Events,
		Shops:              site.Shops,
		Buys:               kvs(site.Buys),
		State:              &protobuf.KVDATA{Key: proto.Uint32(site.StateKey), Value: proto.Uint32(site.StateVal)},
		CharacterThisRound: site.Selected,
		RefreshCount:       proto.Uint32(site.Refresh),
	}
}

// info is the TBINFO of the game.
func (state *neState) info() *protobuf.TBINFO {
	planIDs := make([]uint32, 0, len(state.Plans))
	for _, id := range state.Plans {
		planIDs = append(planIDs, id)
	}
	slices.Sort(planIDs)
	benefit := &protobuf.TBBENEFIT{}
	for _, buff := range state.Buffs {
		benefit.Actives = append(benefit.Actives, &protobuf.TBBF{Id: proto.Uint32(buff.ID), Round: proto.Uint32(buff.Round), IsPending: b2u(buff.Pending)})
	}
	return &protobuf.TBINFO{
		Id:          proto.Uint32(state.ID),
		Fsm:         state.fsm(),
		Round:       &protobuf.TBROUND{Round: proto.Uint32(state.Round), InTemp: b2u(state.InTemp), TempRound: proto.Uint32(state.TempRounds)},
		Res:         state.res(),
		Talent:      &protobuf.TBTALENT{},
		Plan:        &protobuf.TBPLAN{PlanUpgrade: planIDs},
		Site:        &protobuf.TBSITE{Characters: state.SiteShips, WorkCounter: kvs(state.WorkCounter), Works: state.Works, EventCounter: kvs(state.EventCount)},
		Evaluations: kvs(state.Evaluations),
		Name:        proto.String(state.Name),
		FavorLv:     proto.Uint32(state.FavorLv),
		Benefit:     benefit,
		Difficulty:  proto.Uint32(state.Difficulty),
		EvalFail:    b2u(state.EvalFail),
		Display:     &protobuf.TBDISPLAY{},
	}
}

func (perm *nePermanent) proto() *protobuf.TBPERMANENT {
	return &protobuf.TBPERMANENT{
		NgPlusCount:   proto.Uint32(max(perm.Games, 1)),
		Polaroids:     perm.Polaroids,
		Endings:       perm.Endings,
		ActiveEndings: perm.Finished,
		TarotArchive:  perm.Archive,
		MaxRound:      proto.Uint32(perm.MaxRound),
	}
}

// pick draws n distinct ids by weight.
func nePick(weights map[uint32]int64, n int) []uint32 {
	var picked []uint32
	for len(picked) < n {
		var total int64
		for id, w := range weights {
			if w > 0 && !slices.Contains(picked, id) {
				total += w
			}
		}
		if total <= 0 {
			break
		}
		roll := rand.Int64N(total)
		ids := make([]uint32, 0, len(weights))
		for id := range weights {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			w := weights[id]
			if w <= 0 || slices.Contains(picked, id) {
				continue
			}
			if roll < w {
				picked = append(picked, id)
				break
			}
			roll -= w
		}
	}
	return picked
}
