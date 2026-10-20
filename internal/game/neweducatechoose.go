package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The priority states (NewEducatePriorityFSM): picking a tarot card or a
// reading among a few (NewEducateChooseLayer), replacing the tarot card
// (only one is held), leveling a reading up (NewEducateTarotEntryLayer).

// neChoices is how many cards / readings a choice offers (the choose
// layer's three slots).
const neChoices = 3

// topPriority is the first priority state of a system.
func (state *neState) topPriority(system uint32) *nePriority {
	for i := range state.Priority {
		if state.Priority[i].System == system {
			return &state.Priority[i]
		}
	}
	return nil
}

func (state *neState) popPriority(p *nePriority) {
	for i := range state.Priority {
		if &state.Priority[i] == p {
			state.Priority = slices.Delete(state.Priority, i, i+1)
			return
		}
	}
}

// choicePool is what a choice draws from: pool 0 is the round's tarot
// choice (child2_round tarot_select: the character's tarot cards but the
// one held); a "Tarot Reading - X" pool the readings of that card
// (child2_benefit_list tarot_tag); another pool every reading. Readings
// owned (or their level-up) and hidden ones are left out. The pools
// themselves are server content: these are guesses from their names.
func (state *neState) choicePool(p *nePriority) []uint32 {
	var pool []uint32
	held := state.tarot()
	if p.Pool == 0 {
		for id, buff := range neBuffs {
			if buff.Character == state.ID && buff.Type == neBuffTarot && buff.visible && id != held {
				pool = append(pool, id)
			}
		}
	} else {
		tag := uint32(0)
		if name, ok := strings.CutPrefix(neBuffs[p.Pool].Name, "Tarot Reading - "); ok {
			for _, buff := range neBuffs {
				if buff.Character == state.ID && buff.Type == neBuffTarot && buff.Name == "Tarot - "+name {
					tag = buff.TarotTag
				}
			}
		}
		for id, buff := range neBuffs {
			if buff.Character != state.ID || buff.Type != neBuffEntry || !buff.visible || buff.BenefitLvl > 1 {
				continue
			}
			if tag != 0 && buff.TarotTag != tag || state.buff(id) != nil || buff.NextLevel != 0 && state.buff(buff.NextLevel) != nil {
				continue
			}
			pool = append(pool, id)
		}
	}
	pool = slices.DeleteFunc(pool, func(id uint32) bool { return slices.Contains(p.Selects, id) })
	slices.Sort(pool)
	return pool
}

// draw draws n choices from the pool, the "pool exhausted" tokens (gameset
// child2_pool_exhausted_token) when it's empty.
func (state *neState) draw(p *nePriority, n int) []uint32 {
	pool := state.choicePool(p)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	picked := pool[:min(n, len(pool))]
	for i := 0; len(picked) < n && i < len(neExhausted); i++ {
		if !slices.Contains(p.Selects, neExhausted[i]) {
			picked = append(picked, neExhausted[i])
		}
	}
	return slices.Clone(picked)
}

// handleNewEducateRequestChoices answers NewEducateRequestChoicesCommand,
// as the choose layer opens: the choice's cards / readings, drawn the
// first time.
func (game *Game) handleNewEducateRequestChoices(client *connection.Client, req *protobuf.CS_29107) error {
	var cache *protobuf.TBFSMCACHENIN1
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		p := ne.topPriority(neSysPickChoice)
		if p == nil {
			return errRefused
		}
		if len(p.Selects) == 0 {
			p.Selects = ne.draw(p, neChoices)
			p.Rerolls = make([]uint32, len(p.Selects))
		}
		cache = p.nin1()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29108{Result: result(err), Cache: cache} })
}

// handleNewEducateRefreshChoice answers NewEducateRefreshChoiceCommand: one
// choice redrawn, paid with a redraw (RES_TYPE.REFRESH_CHOICE), within the
// redraws a slot allows (gameset child2_tarot_refresh_limit /
// child2_effect_refresh_limit).
func (game *Game) handleNewEducateRefreshChoice(client *connection.Client, req *protobuf.CS_29105) error {
	var cache *protobuf.TBFSMCACHENIN1
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		p := ne.topPriority(neSysPickChoice)
		idx := int(req.GetIndex()) - 1
		if p == nil || idx < 0 || idx >= len(p.Selects) {
			return errRefused
		}
		limit := neEntryRerolls
		if neBuffs[p.Selects[0]].Type == neBuffTarot {
			limit = neTarotRerolls
		}
		if p.Rerolls[idx] >= limit {
			return errRefused
		}
		picked := ne.draw(p, 1)
		if len(picked) == 0 || slices.Contains(neExhausted, picked[0]) {
			return errRefused
		}
		if err := ne.pay([]neDrop{{neDropRes, ne.resOfType(neResRefreshChoice), 1}}); err != nil {
			return err
		}
		p.Selects[idx] = picked[0]
		p.Rerolls[idx]++
		cache = p.nin1()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29106{Result: result(err), Cache: cache} })
}

// takeTarot is the drops that make a tarot card the one held.
func (state *neState) takeTarot(id uint32) []neDrop {
	var drops []neDrop
	if held := state.tarot(); held != 0 {
		if held == id {
			return nil
		}
		drops = append(drops, neDrop{neDropBuff, held, -1})
	}
	return append(drops, neDrop{neDropBuff, id, 1})
}

// handleNewEducateMakeChoice answers NewEducateMakeChoiceCommand: the card
// or reading picked is gained (a card replaces the one held, the layer
// confirmed it), with its "when obtained" effects.
func (game *Game) handleNewEducateMakeChoice(client *connection.Client, req *protobuf.CS_29103) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		p := ne.topPriority(neSysPickChoice)
		idx := int(req.GetIndex()) - 1
		if p == nil || idx < 0 || idx >= len(p.Selects) {
			return errRefused
		}
		id := p.Selects[idx]
		ne.popPriority(p)
		base := []neDrop{{neDropBuff, id, 1}}
		if neBuffs[id].Type == neBuffTarot {
			base = ne.takeTarot(id)
		}
		drops = ne.give(base, nil)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29104{Result: result(err), Drop: drops} })
}

// handleNewEducateGiveUpChoice answers NewEducateGiveUpChoiceCommand: the
// choice is dropped for the refund the layer shows (child2_data
// event_disclaim_refund / shop_disclaim_refund, its second entry).
func (game *Game) handleNewEducateGiveUpChoice(client *connection.Client, req *protobuf.CS_29101) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		p := ne.topPriority(neSysPickChoice)
		if p == nil || ne.tarot() == 0 {
			return errRefused
		}
		raw := neChars[ne.ID].EventRefund
		if p.FromShop {
			raw = neChars[ne.ID].ShopRefund
		}
		ne.popPriority(p)
		var refunds []json.RawMessage
		_ = json.Unmarshal(raw, &refunds)
		var base []neDrop
		if len(refunds) >= 2 {
			if drop, ok := neDropOf(refunds[1]); ok {
				base = append(base, drop)
			}
		}
		drops = ne.give(base, nil)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29102{Result: result(err), Drop: drops} })
}

// handleNewEducateReplaceTarot answers NewEducateReplaceTarotCommand, when
// a tarot card was gained while one is held: the card kept (the held one
// or the new one); the queue of new cards is cleared.
func (game *Game) handleNewEducateReplaceTarot(client *connection.Client, req *protobuf.CS_29120) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		id := req.GetTarotId()
		if len(ne.TarotSelects) == 0 || id != ne.tarot() && id != ne.TarotSelects[0] {
			return errRefused
		}
		ne.TarotSelects = nil
		drops = ne.give(ne.takeTarot(id), nil)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29121{Result: result(err), Drop: drops} })
}

// handleNewEducateUpgradeEntry answers NewEducateUpgradeEntryCommand: a
// reading held goes to its next level (child2_benefit_list next_level),
// with what the buffs give on a level-up.
func (game *Game) handleNewEducateUpgradeEntry(client *connection.Client, req *protobuf.CS_29122) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		p := ne.topPriority(neSysUpEntry)
		id := req.GetAffixid()
		config := neBuffs[id]
		if p == nil || ne.buff(id) == nil || config.Type != neBuffEntry || config.NextLevel == 0 {
			return errRefused
		}
		ne.popPriority(p)
		drops = ne.give([]neDrop{{neDropBuff, id, -1}, {neDropBuff, config.NextLevel, 1}}, ne.triggered(neTrigLevelUp, nil))
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29123{Result: result(err), Drop: drops} })
}

// handleNewEducateGiveUpEntryUp answers NewEducateGiveUpEntryUpCommand: the
// level-up is dropped.
func (game *Game) handleNewEducateGiveUpEntryUp(client *connection.Client, req *protobuf.CS_29124) error {
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		p := ne.topPriority(neSysUpEntry)
		if p == nil {
			return errRefused
		}
		ne.popPriority(p)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29125{Result: result(err)} })
}
