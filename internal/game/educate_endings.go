package game

import (
	"context"
	"slices"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The end of a Project Identity: TB game and what is kept from it: the
// ending reached at gameset child_end_data (child_ending), then a new game;
// the endings, memories (child_memory) and photos (child_polaroid) collected
// can also be bought with gold; an unlocked special secretary can be shown
// as the commander's (SC_11003.child_display).

// EducateBuyCollectCommand.TYPE
const (
	educateCollectEnding   = 1
	educateCollectMemory   = 2
	educateCollectPolaroid = 3
)

// meetsEnding is EducateChar:CheckEndCondition: [1, attr, value] an
// attribute of at least value, [1, attr] the personality, [2, resource,
// value] a resource of at least value.
func (state *educateState) meetsEnding(conditions [][]uint32) bool {
	for _, condition := range conditions {
		if len(condition) < 2 {
			continue
		}
		switch condition[0] {
		case childDropAttr:
			if len(condition) < 3 {
				if state.personality() != condition[1] {
					return false
				}
			} else if state.Attrs[condition[1]] < condition[2] {
				return false
			}
		case childDropRes:
			if len(condition) >= 3 && state.resource(condition[1]) < int64(condition[2]) {
				return false
			}
		}
	}
	return true
}

// handleEducateEnding answers EducateTriggerEndCommand (CS_27008,
// EducateSelEndingLayer at the end of the game): the ending picked among
// the ones TB qualifies for (EducateProxy:GetEndingResult, all sent). The
// game is over (is_ending, EducateConst.STATUES_RESET); the ending counts as
// reached and every qualified one as unlocked.
func (game *Game) handleEducateEnding(client *connection.Client, req *protobuf.CS_27008) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		id := req.GetEndingId()
		qualified := req.GetQualifiedId()
		if state.Ending || !state.endingTime() || !slices.Contains(qualified, id) {
			return errRefused
		}
		for _, ending := range qualified {
			config, ok := educateEndings[ending]
			if !ok || !state.meetsEnding(config.Condition) {
				return errRefused
			}
		}
		state.Ending = true
		if !slices.Contains(state.Completed, id) {
			state.Completed = append(state.Completed, id)
		}
		for _, ending := range qualified {
			if !slices.Contains(state.Endings, ending) {
				state.Endings = append(state.Endings, ending)
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27009{Result: result(err)})
}

// handleEducateReset answers EducateResetCommand (CS_27029, EducateBottomPanel
// once an ending was reached): a new game (new game+ counted), keeping the
// collections and the special secretary; the client asks for it again
// (CS_27000).
func (game *Game) handleEducateReset(client *connection.Client, req *protobuf.CS_27029) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		if !state.Ending {
			return errRefused
		}
		*state = *newEducateGame(state)
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27030{Result: result(err)})
}

// handleEducateRefresh answers EducateRefreshCommand (CS_27047,
// EducateTopPanel's refresh button, "Start growth from the beginning?"):
// the running game starts over from month 2 week 4, keeping the
// collections; unlike a reset after an ending it doesn't count as a new
// game+ (a guess: the run wasn't finished). The client asks for the game
// again (CS_27000).
func (game *Game) handleEducateRefresh(client *connection.Client, req *protobuf.CS_27047) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		restarted := newEducateGame(state)
		restarted.Games = state.Games
		*state = *restarted
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27048{Result: result(err)})
}

// handleEducateBuyCollect answers EducateBuyCollectCommand (CS_27049, the
// collection's unlock buttons): an ending or a memory for
// min(child_cg_max_price, child_cg_basic_price + bought * child_cg_add_price)
// gold, a photo (its whole group, offered from the second game) for
// min(child_polaroid_max_price, child_polaroid_basic_price + bought *
// child_polaroid_add_price).
func (game *Game) handleEducateBuyCollect(client *connection.Client, req *protobuf.CS_27049) error {
	err := game.educateTx(client, func(tx *Tx, state *educateState) error {
		id := req.GetIds()
		price := func(basic, bought uint32, prefix string) uint32 {
			return min(educatePrices[prefix+"_max_price"], basic+bought*educatePrices[prefix+"_add_price"])
		}
		switch req.GetType() {
		case educateCollectEnding:
			ending, ok := educateEndings[id]
			if !ok || slices.Contains(state.Endings, id) {
				return errRefused
			}
			if err := tx.SpendResource(ResourceGold, price(ending.Price, state.EndingBuys, "child_cg")); err != nil {
				return err
			}
			state.Endings = append(state.Endings, id)
			state.EndingBuys++
		case educateCollectMemory:
			memory, ok := educateMemories[id]
			if !ok || slices.Contains(state.Memories, id) {
				return errRefused
			}
			if err := tx.SpendResource(ResourceGold, price(memory.Price, state.MemoryBuys, "child_cg")); err != nil {
				return err
			}
			state.Memories = append(state.Memories, id)
			state.MemoryBuys++
		case educateCollectPolaroid:
			polaroid, ok := educatePolaroids[id]
			if _, owned := state.Polaroids[id]; !ok || owned || state.firstGame() {
				return errRefused
			}
			if err := tx.SpendResource(ResourceGold, price(educatePrices["child_polaroid_basic_price"], state.PolaroidBuys, "child_polaroid")); err != nil {
				return err
			}
			for _, other := range sortedKeys(educatePolaroids) {
				if _, owned := state.Polaroids[other]; educatePolaroids[other].Group == polaroid.Group && !owned {
					state.Polaroids[other] = state.Time
				}
			}
			state.PolaroidBuys++
		default:
			return errRefused
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_27050{Result: result(err)})
}

// handleEducateCharacter answers ChangeEducateCharacterCommand (CS_27041,
// the profile's special secretary slot): the secretary_special_ship shown
// (0 none), back in SC_11003.child_display. The client only offers the
// unlocked ones.
func (game *Game) handleEducateCharacter(client *connection.Client, req *protobuf.CS_27041) error {
	id := req.GetEndingId()
	if _, ok := educateSecretaries[id]; !ok && id != 0 {
		return client.Send(&protobuf.SC_27042{Result: result(errRefused)})
	}
	if err := game.DB.SetChildDisplay(context.Background(), db.SetChildDisplayParams{ChildDisplay: id, ID: client.CommanderID}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_27042{Result: result(nil)})
}
