package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// Config tables of Project Identity (Oceana, Explorer...: child2_*), read by
// NewEducateChar & co. in the client. Fields the tables write several ways
// (a list or "", a drop or a list of drops, a number or a [min, max]
// range) are kept raw and read with the helpers below.

// child2_data row: a character.
type neCharConfig struct {
	FavorExp     []int64           `json:"favor_exp"`
	FavorLevel   uint32            `json:"favor_level"`
	FavorDisplay json.RawMessage   `json:"favor_result_display"`
	Personality  []json.RawMessage `json:"personality_param"` // [tag, [min, max]]
	EventRefund  json.RawMessage   `json:"event_disclaim_refund"`
	ShopRefund   json.RawMessage   `json:"shop_disclaim_refund"`
	RawUnlock    []json.RawMessage `json:"unlock"` // [system, round]

	tags   []neTag
	unlock map[string]uint32
}

// neTag is a personality tag and the personality values it covers.
type neTag struct {
	name     string
	min, max int64
}

// child2_attr / child2_resource row
type neValueConfig struct {
	Character uint32 `json:"character"`
	Default   int64  `json:"default_value"`
	Min       int64  `json:"min_value"`
	Max       int64  `json:"max_value"`
	Type      uint32 `json:"type"`
}

// child2_round row
type neRoundConfig struct {
	Character     uint32          `json:"character"`
	Round         uint32          `json:"round"`
	RoundType     uint32          `json:"round_type"`
	Hard          uint32          `json:"is_hard_mode"`
	BenefitSelect json.RawMessage `json:"benefit_select"` // "" or [[benefit, weight]]
	MainEvent     json.RawMessage `json:"main_event_node_id"`
	ChatNode      json.RawMessage `json:"main_event_chat_node_id"`
	StageNode     uint32          `json:"stage_change_event_node_id"`
	Target        uint32          `json:"target_id"`
	Mobility      int64           `json:"map_mobility"`
	RefreshRefill int64           `json:"refresh_refill"`
	PlanGroups    []uint32        `json:"plan_group"`
	PlanNum       uint32          `json:"plan_num"`
	TarotSelect   json.RawMessage `json:"tarot_select"` // "" or [pool]
	EndlessFactor string          `json:"endless_factor"`
}

// child2_node row
type neNodeConfig struct {
	Type       uint32          `json:"type"`
	Next       json.RawMessage `json:"next"`
	NextType   uint32          `json:"next_type"`
	OptionCost json.RawMessage `json:"option_cost"`
}

// child2_plan row
type nePlanConfig struct {
	Group     uint32          `json:"group_id"`
	Level     uint32          `json:"level"`
	Condition json.RawMessage `json:"level_condition"`
	Cost      json.RawMessage `json:"cost"`
	Result    json.RawMessage `json:"result_display"`
	Node      uint32          `json:"result_node"`
}

// child2_benefit_list row: a buff (talent, status, tarot or reading).
type neBuffConfig struct {
	Character  uint32          `json:"character"`
	Type       uint32          `json:"type"`
	RawContent json.RawMessage `json:"content"`
	During     int64           `json:"during_time"`
	Show       uint32          `json:"is_show"`
	Hidden     uint32          `json:"ignore_on_collection"`
	Name       string          `json:"name"`
	NextLevel  uint32          `json:"next_level"`
	TarotTag   uint32          `json:"tarot_tag"`
	BenefitLvl uint32          `json:"benefit_level"`
	content    []uint32
	visible    bool
}

// child2_benefit row: one effect list of a buff.
type neBenefitConfig struct {
	Condition json.RawMessage `json:"condition"`
	Effect    json.RawMessage `json:"effect"`
	Trigger   uint32          `json:"trigger"`
	effects   []neEffect
}

// neEffect is a child2_benefit effect: {type, params...}.
type neEffect struct {
	kind   uint32
	params []json.RawMessage
}

// child2_condition row
type neConditionConfig struct {
	Type  uint32          `json:"type"`
	Param json.RawMessage `json:"param"`
}

// child2_ending row
type neEndingConfig struct {
	Character uint32          `json:"character"`
	Condition json.RawMessage `json:"condition"`
}

// child2_target row
type neTargetConfig struct {
	AttrSum   int64    `json:"attr_sum"`
	Levels    []int64  `json:"attr_sum_level"`
	Nodes     []uint32 `json:"result_node"`
	Character uint32   `json:"character"`
}

// child2_shop row
type neGoodsConfig struct {
	Goods     uint32          `json:"goods_id"`
	GoodsList json.RawMessage `json:"goods_id_list"`
	Num       int64           `json:"goods_num"`
	Type      uint32          `json:"goods_type"`
	Limit     int64           `json:"limit_num"`
	Price     int64           `json:"resource_num"`
	PriceType uint32          `json:"resource_type"`
	Refresh   uint32          `json:"is_refresh"`
}

// child2_site_normal row: a work / travel site level.
type neNormalConfig struct {
	Character uint32          `json:"character"`
	Type      uint32          `json:"type"`
	Level     uint32          `json:"site_lv"`
	Cost      json.RawMessage `json:"cost"`
	Drops     json.RawMessage `json:"drop_display"`
	Upgrade   json.RawMessage `json:"special_args"`
	Node      uint32          `json:"node_id"`
}

// child2_site_character row: a shipgirl at a level.
type neShipConfig struct {
	Group uint32          `json:"group"`
	Level uint32          `json:"level"`
	Cost  json.RawMessage `json:"cost"`
	Drops json.RawMessage `json:"drop_display"`
	Node  uint32          `json:"node_id"`
}

// child2_site_display row
type neSiteConfig struct {
	Character uint32 `json:"character"`
	Type      uint32 `json:"type"`
	Param     uint32 `json:"param"`
	Bg        string `json:"bg"`
}

// child2_site_event_group row: a random event.
type neEventConfig struct {
	Condition json.RawMessage `json:"condition"`
	Cost      json.RawMessage `json:"event_cost"`
	Node      uint32          `json:"node_id"`
	Weight    int64           `json:"weight"`
}

// child2_chat row: a topic.
type neChatConfig struct {
	Node uint32 `json:"result_node"`
}

// NewEducateConst.SITE_TYPE, child2_site_display type
const (
	neSiteShip   = 1
	neSiteShop   = 2
	neSiteWork   = 3
	neSiteTravel = 4
	neSiteEvent  = 5
)

var (
	neChars      map[uint32]neCharConfig
	neAttrs      map[uint32]neValueConfig
	neResources  map[uint32]neValueConfig
	neRounds     map[uint32]neRoundConfig
	neNodes      map[uint32]neNodeConfig
	nePlans      map[uint32]nePlanConfig
	neBuffs      map[uint32]neBuffConfig
	neBenefits   map[uint32]neBenefitConfig
	neConditions map[uint32]neConditionConfig
	neEndings    map[uint32]neEndingConfig
	neTargets    map[uint32]neTargetConfig
	neGoods      map[uint32]neGoodsConfig
	neNormals    map[uint32]neNormalConfig
	neShips      map[uint32]neShipConfig
	neSites      map[uint32]neSiteConfig
	neEvents     map[uint32]neEventConfig
	neChats      map[uint32]neChatConfig

	// indexes
	nePlanGroups map[uint32][]uint32        // plan group -> plan ids by level
	neShipGroups map[uint32][]uint32        // shipgirl group -> ids by level
	neShipBg     map[uint32]string          // shipgirl group -> red / blue (site display)
	neCharShips  map[uint32][]uint32        // character -> shipgirl groups on its map
	neCharEvents map[uint32][]uint32        // character -> events with a map site
	neCharShop   map[uint32]bool            // character -> has a shop site
	neTarots     map[uint32]map[uint32]bool // tarot_tag -> tarot ids

	// gameset
	neShopRefreshPrice int64
	neShopRefreshCount uint32
	neTarotRerolls     uint32
	neEntryRerolls     uint32
	neExhausted        []uint32
	neEndlessFactors   map[uint32][][3]float64 // difficulty -> {from, to, factor}
)

func loadNewEducateData() error {
	gameset := map[string]uint32{}
	var easy, hard [][3]float64
	err := gamedata.LoadAll(
		gamedata.Load("child2_data", &neChars),
		gamedata.Load("child2_attr", &neAttrs),
		gamedata.Load("child2_resource", &neResources),
		gamedata.Load("child2_round", &neRounds),
		gamedata.Load("child2_node", &neNodes),
		gamedata.Load("child2_plan", &nePlans),
		gamedata.Load("child2_benefit_list", &neBuffs),
		gamedata.Load("child2_benefit", &neBenefits),
		gamedata.Load("child2_condition", &neConditions),
		gamedata.Load("child2_ending", &neEndings),
		gamedata.Load("child2_target", &neTargets),
		gamedata.Load("child2_shop", &neGoods),
		gamedata.Load("child2_site_normal", &neNormals),
		gamedata.Load("child2_site_character", &neShips),
		gamedata.Load("child2_site_display", &neSites),
		gamedata.Load("child2_site_event_group", &neEvents),
		gamedata.Load("child2_chat", &neChats),
		// TW's gameset has neither (its client would fail opening the choose
		// layer or ending a round past the last): no exhausted tokens, no
		// endless factors there
		func() error {
			_, err := gamedata.Feature(func() error {
				return gamedata.GamesetDescription("child2_pool_exhausted_token", &neExhausted)
			})
			return err
		},
		func() error {
			_, err := gamedata.Feature(
				func() error { return gamedata.GamesetDescription("child2_endless_loop_extra_factor", &easy) },
				func() error { return gamedata.GamesetDescription("child2_endless_loop_extra_factor_hard", &hard) },
			)
			return err
		},
		func() (err error) {
			values, err := gamedata.Gameset()
			for _, key := range []string{"child2_shop_refresh_price", "child2_shop_refresh_count", "child2_tarot_refresh_limit", "child2_effect_refresh_limit"} {
				gameset[key] = values[key]
			}
			return err
		},
	)
	if err != nil {
		return err
	}
	neShopRefreshPrice = int64(gameset["child2_shop_refresh_price"])
	neShopRefreshCount = gameset["child2_shop_refresh_count"]
	neTarotRerolls = gameset["child2_tarot_refresh_limit"]
	neEntryRerolls = gameset["child2_effect_refresh_limit"]
	neEndlessFactors = map[uint32][][3]float64{0: easy, 1: hard}

	for id, char := range neChars {
		for _, raw := range char.Personality {
			var pair []json.RawMessage
			var name string
			var span []int64
			if json.Unmarshal(raw, &pair) != nil || len(pair) != 2 || json.Unmarshal(pair[0], &name) != nil || json.Unmarshal(pair[1], &span) != nil || len(span) != 2 {
				return gamedata.Missing("child2_data", "personality_param of character %d", id)
			}
			char.tags = append(char.tags, neTag{name, span[0], span[1]})
		}
		char.unlock = map[string]uint32{}
		for _, raw := range char.RawUnlock {
			var pair []json.RawMessage
			var name string
			var round uint32
			if json.Unmarshal(raw, &pair) == nil && len(pair) == 2 && json.Unmarshal(pair[0], &name) == nil && json.Unmarshal(pair[1], &round) == nil {
				char.unlock[name] = round
			}
		}
		neChars[id] = char
	}
	for id, buff := range neBuffs {
		buff.content = gamedata.IDs(buff.RawContent)
		buff.visible = buff.Hidden != 1 && buff.Show != 0
		neBuffs[id] = buff
	}
	for id, benefit := range neBenefits {
		var effects []json.RawMessage
		_ = json.Unmarshal(benefit.Effect, &effects)
		for _, raw := range effects {
			var parts []json.RawMessage
			var kind uint32
			if json.Unmarshal(raw, &parts) == nil && len(parts) > 0 && json.Unmarshal(parts[0], &kind) == nil {
				benefit.effects = append(benefit.effects, neEffect{kind, parts[1:]})
			}
		}
		neBenefits[id] = benefit
	}

	nePlanGroups = map[uint32][]uint32{}
	for id, plan := range nePlans {
		nePlanGroups[plan.Group] = append(nePlanGroups[plan.Group], id)
	}
	for _, ids := range nePlanGroups {
		slices.SortFunc(ids, func(a, b uint32) int { return int(nePlans[a].Level) - int(nePlans[b].Level) })
	}
	neShipGroups = map[uint32][]uint32{}
	for id, ship := range neShips {
		neShipGroups[ship.Group] = append(neShipGroups[ship.Group], id)
	}
	for _, ids := range neShipGroups {
		slices.SortFunc(ids, func(a, b uint32) int { return int(neShips[a].Level) - int(neShips[b].Level) })
	}
	neShipBg = map[uint32]string{}
	neCharShips = map[uint32][]uint32{}
	neCharEvents = map[uint32][]uint32{}
	neCharShop = map[uint32]bool{}
	for _, site := range neSites {
		switch site.Type {
		case neSiteShip:
			ship, ok := neShips[site.Param]
			if !ok {
				continue
			}
			if !slices.Contains(neCharShips[site.Character], ship.Group) {
				neCharShips[site.Character] = append(neCharShips[site.Character], ship.Group)
			}
			neShipBg[ship.Group] = site.Bg
		case neSiteEvent:
			if _, ok := neEvents[site.Param]; ok {
				neCharEvents[site.Character] = append(neCharEvents[site.Character], site.Param)
			}
		case neSiteShop:
			neCharShop[site.Character] = true
		}
	}
	for char := range neCharShips {
		slices.Sort(neCharShips[char])
	}
	for char := range neCharEvents {
		slices.Sort(neCharEvents[char])
	}
	neTarots = map[uint32]map[uint32]bool{}
	for id, buff := range neBuffs {
		if buff.Type == neBuffTarot && buff.visible {
			if neTarots[buff.TarotTag] == nil {
				neTarots[buff.TarotTag] = map[uint32]bool{}
			}
			neTarots[buff.TarotTag][id] = true
		}
	}
	for id := range neChars {
		if neStartBuff(id) == 0 {
			return gamedata.Missing("child2_benefit_list", "mood benefit of character %d", id)
		}
		if len(neRoundIDs(id, 0)) == 0 {
			return gamedata.Missing("child2_round", "rounds of character %d", id)
		}
	}
	return nil
}

// buff types (NewEducateBuff.TYPE)
const (
	neBuffTalent = 1
	neBuffStatus = 2
	neBuffTarot  = 3
	neBuffEntry  = 4
)

// neStartBuff is the hidden buff a character starts every game with (the
// mood benefit, NewEducateBenefit.IsMoodBenefit's range: 10000 for Oceana
// and 10001 for Explorer in the capture).
func neStartBuff(char uint32) uint32 {
	for id := uint32(10000); id <= 10100; id++ {
		if buff, ok := neBuffs[id]; ok && buff.Character == char {
			return id
		}
	}
	return 0
}

// neRoundIDs are a character's normal rounds (round number -> child2_round
// id) on a difficulty (NewEducateRound:InitConfig).
func neRoundIDs(char, difficulty uint32) map[uint32]uint32 {
	ids := map[uint32]uint32{}
	for id, round := range neRounds {
		if round.Character == char && round.RoundType == 1 && round.Hard == difficulty {
			ids[round.Round] = id
		}
	}
	return ids
}

// neEndlessIDs are a character's endless rounds on a difficulty, in order.
func neEndlessIDs(char, difficulty uint32) []uint32 {
	var ids []uint32
	for id, round := range neRounds {
		if round.Character == char && round.RoundType == 2 && round.Hard == difficulty {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// neDrop is a CHILD2 drop: {type, id, number}.
type neDrop struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Number int64  `json:"number"`
}

// drop types (NewEducateConst.DROP_TYPE)
const (
	neDropAttr      = 1
	neDropRes       = 2
	neDropPolaroid  = 3
	neDropBuff      = 4
	neDropChoose    = 5
	neDropUpEntry   = 6
	neDropTempRound = 7
	neDropTarot     = 10000
)

// neNumber reads a number, or a [min, max] range drawn at random.
func neNumber(raw json.RawMessage) int64 {
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return int64(n)
	}
	var span []float64
	if json.Unmarshal(raw, &span) == nil && len(span) == 2 && span[1] >= span[0] {
		return int64(span[0]) + rand.Int64N(int64(span[1])-int64(span[0])+1)
	}
	return 0
}

// neDropOf reads one {type, id, number} drop.
func neDropOf(raw json.RawMessage) (neDrop, bool) {
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil || len(parts) < 3 {
		return neDrop{}, false
	}
	var kind, id uint32
	if json.Unmarshal(parts[0], &kind) != nil || json.Unmarshal(parts[1], &id) != nil {
		return neDrop{}, false
	}
	return neDrop{kind, id, neNumber(parts[2])}, true
}

// neDropsOf reads a drop field written as one drop or a list of them ([]
// and "" are none).
func neDropsOf(raw json.RawMessage) []neDrop {
	if drop, ok := neDropOf(raw); ok {
		return []neDrop{drop}
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var drops []neDrop
	for _, item := range list {
		if drop, ok := neDropOf(item); ok {
			drops = append(drops, drop)
		}
	}
	return drops
}

// neTagged reads a [[tag, value]...] field (main_event_node_id...): the
// value for a personality tag, 0 when the field has none.
func neTagged(raw json.RawMessage, tag string) uint32 {
	var list [][]json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return 0
	}
	for _, pair := range list {
		var name string
		var value uint32
		if len(pair) == 2 && json.Unmarshal(pair[0], &name) == nil && name == tag && json.Unmarshal(pair[1], &value) == nil {
			return value
		}
	}
	return 0
}

// nePairs reads a [[a, b]...] field of numbers.
func nePairs(raw json.RawMessage) [][2]int64 {
	var list [][]json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var pairs [][2]int64
	for _, pair := range list {
		if len(pair) == 2 {
			pairs = append(pairs, [2]int64{neNumber(pair[0]), neNumber(pair[1])})
		}
	}
	return pairs
}

// neNext reads child2_node.next as a single node ("" is none).
func neNextNode(raw json.RawMessage) uint32 {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		id, _ := strconv.ParseUint(text, 10, 32)
		return uint32(id)
	}
	var id uint32
	if json.Unmarshal(raw, &id) == nil {
		return id
	}
	return 0
}
