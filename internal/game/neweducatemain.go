package game

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"slices"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// child2_node types (NewEducateNodePanel.NODE_TYPE) and next types
const (
	neNodeDrop = 102

	neNextNormal = 1
	neNextOption = 2
	neNextRandom = 3
	neNextFlag   = 4
)

// handleNewEducate answers NewEducateRequestCommand, sent for each
// child2_data character on entering Project Identity: the running game, a
// new one the first time (as captured: round 1, nothing started, one game
// counted).
func (game *Game) handleNewEducate(client *connection.Client, req *protobuf.CS_29001) error {
	char := req.GetId()
	if _, ok := neChars[char]; !ok {
		return client.Send(&protobuf.SC_29002{Result: result(errRefused)})
	}
	ctx := context.Background()
	ne, err := loadNeGame(ctx, game.DB.Queries, client.CommanderID, char)
	if err != nil {
		return err
	}
	if ne == nil {
		ne = &neGame{neState: newNeGame(char, 0), perm: &nePermanent{Games: 1}}
		if err := saveNeGame(ctx, game.DB.Queries, client.CommanderID, ne); err != nil {
			return err
		}
	}
	return client.Send(&protobuf.SC_29002{Result: result(nil), Tb: ne.info(), Permanent: ne.perm.proto()})
}

// restart starts a new game of the character on a difficulty; hard mode
// opens once an ending was reached (wiki: Project Identity: Explorer).
func (ne *neGame) restart(difficulty uint32) error {
	if difficulty > 1 || difficulty == 1 && len(ne.perm.Finished) == 0 || len(neRoundIDs(ne.ID, difficulty)) == 0 {
		return errRefused
	}
	ne.neState = newNeGame(ne.ID, difficulty)
	return nil
}

// handleNewEducateReset answers NewEducateResetCommand (the reset button at
// an ending or in endless mode): a new game, one more game counted.
func (game *Game) handleNewEducateReset(client *connection.Client, req *protobuf.CS_29007) error {
	var tb *protobuf.TBINFO
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if err := ne.restart(req.GetDifficulty()); err != nil {
			return err
		}
		ne.perm.Games = max(ne.perm.Games, 1) + 1
		tb = ne.info()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29008{Result: result(err), Tb: tb} })
}

// handleNewEducateRefresh answers NewEducateRefreshCommand, the select
// scene's difficulty switch: the game restarts on the other difficulty
// (child2_switch_sure), the game count unchanged (NewEducateProxy:
// RefreshChar keeps it).
func (game *Game) handleNewEducateRefresh(client *connection.Client, req *protobuf.CS_29092) error {
	var tb *protobuf.TBINFO
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if err := ne.restart(req.GetDifficulty()); err != nil {
			return err
		}
		tb = ne.info()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29093{Result: result(err), Tb: tb} })
}

// handleNewEducateSetCall answers NewEducateSetCallCommand: what the child
// calls the commander.
func (game *Game) handleNewEducateSetCall(client *connection.Client, req *protobuf.CS_29009) error {
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		name := req.GetName()
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 14 {
			return errRefused
		}
		ne.Name = name
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29010{Result: result(err)} })
}

// handleNewEducateMainEvent answers NewEducateMainEventCommand, at each
// round's start: the round's main event for the personality
// (child2_round main_event_node_id), 0 when it has none.
func (game *Game) handleNewEducateMainEvent(client *connection.Client, req *protobuf.CS_29011) error {
	var node uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysInit && ne.System != neSysPhase && ne.System != neSysEvent {
			return errRefused
		}
		ne.System = neSysEvent
		if !ne.InTemp {
			node = neTagged(ne.roundConfig().MainEvent, ne.tag())
		}
		ne.startChain(node, nil, nil)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29012{Result: result(err), FirstNode: proto.Uint32(node)}
	})
}

// startChain starts a node chain and what its drop node gives.
func (state *neState) startChain(node uint32, base, bonus []neDrop) {
	state.Node = node
	state.Pending, state.PendingFor = nil, nil
	if node != 0 {
		state.Pending, state.PendingFor = base, bonus
	}
}

// startReward starts a chain on an action's reward: kept for the chain's
// drop node, or given now (the reply's drop) when the chain has none.
func (ne *neGame) startReward(node uint32, base []neDrop, trigger uint32, ctx *neCtx) *protobuf.TBDROPS {
	if ctx == nil {
		ctx = &neCtx{}
	}
	extra := ne.bonus(base, ctx)
	ctx.gains = map[[2]uint32]int64{}
	for _, list := range [][]neDrop{base, extra} {
		for _, drop := range list {
			ctx.gains[[2]uint32{drop.Type, drop.ID}] += drop.Number
		}
	}
	if trigger != 0 {
		extra = append(extra, ne.triggered(trigger, ctx)...)
	}
	if neChainDrops(node) {
		ne.startChain(node, base, extra)
		return neDrops(nil, nil)
	}
	ne.startChain(node, nil, nil)
	return ne.give(base, extra)
}

// neChainDrops tells whether a chain reaches a drop node.
func neChainDrops(first uint32) bool {
	seen := map[uint32]bool{}
	queue := []uint32{first}
	for len(queue) > 0 && len(seen) < 200 {
		id := queue[0]
		queue = queue[1:]
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		node, ok := neNodes[id]
		if !ok {
			continue
		}
		if node.Type == neNodeDrop {
			return true
		}
		queue = append(queue, neNextNodes(node)...)
	}
	return false
}

// neNextNodes are the nodes a node can lead to.
func neNextNodes(node neNodeConfig) []uint32 {
	switch node.NextType {
	case neNextNormal:
		return []uint32{neNextNode(node.Next)}
	case neNextOption:
		var ids []uint32
		_ = json.Unmarshal(node.Next, &ids)
		return ids
	case neNextRandom:
		var ids []uint32
		for _, pair := range nePairs(node.Next) {
			ids = append(ids, uint32(pair[0]))
		}
		return ids
	case neNextFlag:
		var ids []uint32
		for _, pair := range nePairs(node.Next) {
			ids = append(ids, uint32(pair[1]))
		}
		return ids
	}
	return nil
}

// neMindPersonality is how far a personality talk's choice moves the
// personality (guess).
const neMindPersonality = 5

// handleNewEducateTriggerNode answers NewEducateTriggerNodeCommand, sent as
// each node of a chain ends: the next node (child2_node next: the option
// picked, a weighted draw, the story's flag) and the drops of the node that
// ended (a drop node gives what started the chain). An option's cost
// (option_cost) is paid, as the client does.
func (game *Game) handleNewEducateTriggerNode(client *connection.Client, req *protobuf.CS_29030) error {
	var next uint32
	drops := neDrops(nil, nil)
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		node, ok := neNodes[ne.Node]
		if ne.Node == 0 || !ok {
			return errRefused
		}
		branch := req.GetBranch()
		switch node.NextType {
		case neNextNormal:
			next = neNextNode(node.Next)
		case neNextOption:
			if !slices.Contains(neNextNodes(node), branch) {
				return errRefused
			}
			if err := ne.pay(neDropsOf(neNodes[branch].OptionCost)); err != nil {
				return err
			}
			next = branch
		case neNextRandom:
			weights := map[uint32]int64{}
			for _, pair := range nePairs(node.Next) {
				weights[uint32(pair[0])] += pair[1]
			}
			if picked := nePick(weights, 1); len(picked) > 0 {
				next = picked[0]
			}
		case neNextFlag:
			pairs := nePairs(node.Next)
			for _, pair := range pairs {
				if uint32(pair[0]) == branch {
					next = uint32(pair[1])
				}
			}
			if next == 0 && len(pairs) > 0 {
				next = uint32(pairs[0][1])
			}
			if ne.System == neSysMind && ne.Pending == nil && (branch == 1 || branch == 2) {
				// the personality talk's options are tagged mild / rebellious
				// (the story's option type); what a choice moves is a guess,
				// the shipgirls' +5 (wiki: Project Identity: Oceana)
				delta := int64(neMindPersonality)
				if branch == 1 {
					delta = -delta
				}
				ne.Pending = []neDrop{{neDropAttr, ne.personalityID(), delta}}
			}
		}
		if _, ok := neNodes[next]; !ok {
			next = 0
		}
		if node.Type == neNodeDrop || next == 0 {
			if ne.Pending != nil || ne.PendingFor != nil {
				drops = ne.give(ne.Pending, ne.PendingFor)
			}
			ne.Pending, ne.PendingFor = nil, nil
		}
		ne.Node = next
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29031{Result: result(err), NextNode: proto.Uint32(next), Drop: drops}
	})
}

// handleNewEducateClearNodeChain answers NewEducateClearNodeChainCommand,
// the "Return" of the interrupted chain prompt: the chain is dropped, with
// what it would have given.
func (game *Game) handleNewEducateClearNodeChain(client *connection.Client, req *protobuf.CS_29032) error {
	var fsm *protobuf.TBFSM
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		ne.startChain(0, nil, nil)
		fsm = ne.fsm()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29033{Result: result(err), Fsm: fsm} })
}

// talentRound tells whether the round offers talents
// (NewEducateRound:IsTalentRound).
func (state *neState) talentRound() bool {
	return !state.InTemp && len(nePairs(state.roundConfig().BenefitSelect)) > 0
}

// talentPool are the talents the round can offer (child2_round
// benefit_select weights), without the ones owned or excluded.
func (state *neState) talentPool(exclude []uint32) map[uint32]int64 {
	weights := map[uint32]int64{}
	for _, pair := range nePairs(state.roundConfig().BenefitSelect) {
		id := uint32(pair[0])
		if state.buff(id) == nil && !slices.Contains(exclude, id) {
			weights[id] = pair[1]
		}
	}
	return weights
}

// neTalentChoices is how many talents a talent round offers (wiki: "pick an
// ability from a set of three").
const neTalentChoices = 3

// handleNewEducateGetTalents answers NewEducateGetTalentsCommand: the
// talents offered this round (none outside the talent rounds), drawn by
// benefit_select weight.
func (game *Game) handleNewEducateGetTalents(client *connection.Client, req *protobuf.CS_29019) error {
	var talents []uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysEvent && ne.System != neSysTalent || ne.Node != 0 {
			return errRefused
		}
		ne.System = neSysTalent
		if ne.Talent == nil {
			ne.Talent = &struct {
				Finished  bool     `json:"finished"`
				Talents   []uint32 `json:"talents"`
				Retalents []uint32 `json:"retalents"`
			}{}
			if ne.talentRound() {
				ne.Talent.Talents = nePick(ne.talentPool(nil), neTalentChoices)
			}
			ne.Talent.Finished = len(ne.Talent.Talents) == 0
		}
		talents = ne.Talent.Talents
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29020{Result: result(err), Talents: talents} })
}

// handleNewEducateRefreshTalent answers NewEducateRefreshTalentCommand: one
// redraw per offered talent (the layer greys the button out after it).
func (game *Game) handleNewEducateRefreshTalent(client *connection.Client, req *protobuf.CS_29021) error {
	var talent uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		t := ne.Talent
		old := req.GetTalent()
		if t == nil || t.Finished || !slices.Contains(t.Talents, old) || slices.Contains(t.Retalents, old) {
			return errRefused
		}
		picked := nePick(ne.talentPool(slices.Concat(t.Talents, t.Retalents)), 1)
		if len(picked) == 0 {
			return errRefused
		}
		talent = picked[0]
		t.Talents[slices.Index(t.Talents, old)] = talent
		t.Retalents = append(t.Retalents, talent)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29022{Result: result(err), Talent: proto.Uint32(talent)}
	})
}

// handleNewEducateSelTalent answers NewEducateSelTalentCommand: the talent
// picked is gained, with its "when obtained" effects.
func (game *Game) handleNewEducateSelTalent(client *connection.Client, req *protobuf.CS_29023) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		t := ne.Talent
		if t == nil || t.Finished || !slices.Contains(t.Talents, req.GetTalent()) {
			return errRefused
		}
		t.Finished = true
		drops = ne.give([]neDrop{{neDropBuff, req.GetTalent(), 1}}, nil)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29024{Result: result(err), Drop: drops} })
}

// handleNewEducateGetChoose answers NewEducateGetChooseCommand, after the
// talents: the FSM in its choose step, with the round's tarot choice
// (child2_round tarot_select) pushed as a priority state the first time.
func (game *Game) handleNewEducateGetChoose(client *connection.Client, req *protobuf.CS_29126) error {
	var fsm *protobuf.TBFSM
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.System != neSysTalent && ne.System != neSysChoose || ne.Node != 0 {
			return errRefused
		}
		ne.System = neSysChoose
		var pools []uint32
		_ = json.Unmarshal(ne.roundConfig().TarotSelect, &pools)
		if !ne.Chose && !ne.InTemp && len(pools) > 0 {
			ne.Priority = slices.Insert(ne.Priority, 0, nePriority{System: neSysPickChoice})
		}
		ne.Chose = true
		fsm = ne.fsm()
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29127{Result: result(err), Fsm: fsm} })
}

// neChatCharacter is the character a topic (child2_chat, which has no
// character column) belongs to: Explorer's ids are 31xxxxx, Oceana's
// below.
func neChatCharacter(id uint32) uint32 {
	if id >= 1000000 {
		return 2
	}
	return 1
}

// handleNewEducateGetTopics answers NewEducateGetTopicsCommand: the round's
// topic (the main scene offers the first one), one of the character's
// topics not yet talked about this game, none in an extra turn.
func (game *Game) handleNewEducateGetTopics(client *connection.Client, req *protobuf.CS_29015) error {
	var chats []uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if ne.Chat == nil {
			var pool []uint32
			for id := range neChats {
				if neChatCharacter(id) == ne.ID && !slices.Contains(ne.SeenChats, id) {
					pool = append(pool, id)
				}
			}
			slices.Sort(pool)
			ne.Chat = &struct {
				Finished bool     `json:"finished"`
				Chats    []uint32 `json:"chats"`
			}{}
			if len(pool) > 0 && !ne.InTemp {
				ne.Chat.Chats = []uint32{pool[rand.IntN(len(pool))]}
			}
			ne.Chat.Finished = len(ne.Chat.Chats) == 0
		}
		if ne.Node == 0 && ne.System != neSysPlan && ne.System != neSysAssess {
			ne.System = neSysTopic
		}
		chats = ne.Chat.Chats
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29016{Result: result(err), Chats: chats} })
}

// handleNewEducateSelTopic answers NewEducateSelTopicCommand: the topic's
// chain (child2_chat result_node).
func (game *Game) handleNewEducateSelTopic(client *connection.Client, req *protobuf.CS_29017) error {
	var node uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		chat := req.GetChatId()
		if ne.Chat == nil || ne.Chat.Finished || !slices.Contains(ne.Chat.Chats, chat) || ne.Node != 0 {
			return errRefused
		}
		ne.Chat.Finished = true
		ne.SeenChats = append(ne.SeenChats, chat)
		ne.System = neSysTopic
		node = neChats[chat].Node
		ne.startChain(node, nil, nil)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29018{Result: result(err), FirstNode: proto.Uint32(node), Drop: neDrops(nil, nil)}
	})
}

// handleNewEducateSelMind answers NewEducateSelMindCommand, the main
// scene's personality talk: the round's chain for the personality
// (child2_round main_event_chat_node_id), once a round.
func (game *Game) handleNewEducateSelMind(client *connection.Client, req *protobuf.CS_29090) error {
	var node uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		node = neTagged(ne.roundConfig().ChatNode, ne.tag())
		if ne.Mind || ne.InTemp || node == 0 || ne.Node != 0 {
			return errRefused
		}
		ne.Mind = true
		ne.System = neSysMind
		ne.startChain(node, nil, nil)
		return nil
	}, func(err error) proto.Message {
		return &protobuf.SC_29091{Result: result(err), FirstNode: proto.Uint32(node), Drop: neDrops(nil, nil)}
	})
}

// handleNewEducateUpgradeFavor answers NewEducateUpgradeFavorCommand: the
// next favor level once the favor resource covers its child2_data
// favor_exp, with the level's favor_result_display.
func (game *Game) handleNewEducateUpgradeFavor(client *connection.Client, req *protobuf.CS_29027) error {
	var drops *protobuf.TBDROPS
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		char := neChars[ne.ID]
		var spent int64
		for i := uint32(0); i < ne.FavorLv && int(i) < len(char.FavorExp); i++ {
			spent += char.FavorExp[i]
		}
		if int(ne.FavorLv) >= len(char.FavorExp) || ne.FavorLv+1 >= char.FavorLevel+1 {
			return errRefused
		}
		if ne.Res[ne.resOfType(neResFavor)]-spent < char.FavorExp[ne.FavorLv] {
			return ErrNotEnough
		}
		ne.FavorLv++
		var displays []json.RawMessage
		_ = json.Unmarshal(char.FavorDisplay, &displays)
		var base []neDrop
		if int(ne.FavorLv) <= len(displays) {
			base = neDropsOf(displays[ne.FavorLv-1])
		}
		drops = ne.give(base, nil)
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29028{Result: result(err), Drop: drops} })
}

// handleNewEducateGetEndings answers NewEducateGetEndingsCommand, after the
// last round's assessment: the endings whose child2_ending condition holds
// (the character's first one when none does, so a game always ends), kept
// as reached.
func (game *Game) handleNewEducateGetEndings(client *connection.Client, req *protobuf.CS_29003) error {
	var endings []uint32
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		if !ne.lastRound() || ne.InTemp || ne.TempRounds > 0 {
			return errRefused
		}
		if ne.End == nil {
			var ids, all []uint32
			for id, ending := range neEndings {
				if ending.Character != ne.ID {
					continue
				}
				all = append(all, id)
				if ne.match(ending.Condition, nil) {
					ids = append(ids, id)
				}
			}
			slices.Sort(ids)
			slices.Sort(all)
			if len(ids) == 0 && len(all) > 0 {
				ids = all[:1]
			}
			ne.End = &struct {
				Ends   []uint32 `json:"ends"`
				Select uint32   `json:"select"`
			}{Ends: ids}
		}
		ne.System = neSysEnding
		for _, id := range ne.End.Ends {
			if !slices.Contains(ne.perm.Endings, id) {
				ne.perm.Endings = append(ne.perm.Endings, id)
			}
		}
		endings = ne.End.Ends
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29004{Result: result(err), Endings: endings} })
}

// handleNewEducateSelEnding answers NewEducateSelEndingCommand: the ending
// picked among the reached ones.
func (game *Game) handleNewEducateSelEnding(client *connection.Client, req *protobuf.CS_29005) error {
	return game.neReply(client, req.GetId(), func(tx *Tx, ne *neGame) error {
		ending := req.GetEndingId()
		if ne.End == nil || ne.End.Select != 0 || !slices.Contains(ne.End.Ends, ending) {
			return errRefused
		}
		ne.End.Select = ending
		if !slices.Contains(ne.perm.Finished, ending) {
			ne.perm.Finished = append(ne.perm.Finished, ending)
		}
		return nil
	}, func(err error) proto.Message { return &protobuf.SC_29006{Result: result(err)} })
}
