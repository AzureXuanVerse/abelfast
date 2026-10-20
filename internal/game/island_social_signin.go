package game

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The daily supplies (IslandSignInAgency, the plaza's landing pad): once a
// day the commander signs in, which drops daily_gift_drop_num supplies on
// its plaza until midnight (PB_ISLAND_PUBLIC.tree_gift_*). It takes one
// itself, then may invite friends to take the others (tree_gift_invited) or
// open them to every visitor (OPEN_SIGNIN, CS_21002); a visitor takes one
// per island and daily_gift_get_max (plus the island abilities of type 21)
// a day. tree_gift_visitor holds, by position, who took each supply (0
// while it is there).

// islandTaskSignIn is IslandTaskTargetType.SIGN_IN.
const islandTaskSignIn = 13

// islandSupplies is SC_21314, an island's supplies as they are now.
func islandSupplies(state *islandState, cmd uint32) *protobuf.SC_21314 {
	public := state.public()
	return &protobuf.SC_21314{
		IslandId: proto.Uint32(state.commanderID), GiftCount: proto.Uint32(public.GetTreeGiftCount()),
		GiftVisitor: slices.Clone(public.TreeGiftVisitor), GiftTimestamp: proto.Uint32(public.GetTreeGiftTimestamp()),
		Cmd: proto.Uint32(cmd),
	}
}

// islandSupplyFetchMax is how many supplies the commander may take on other
// islands a day (IslandSignInAgency.GetMaxOtheFetchcnt).
func islandSupplyFetchMax(state *islandState) uint32 {
	total := islandSocialConfig.giftFetchMax
	for _, id := range state.public().GetAbilityList() {
		ability := islandSocialConfig.abilities[id]
		var effect uint32
		if ability.Type == islandAbilitySignGift && json.Unmarshal(ability.Effect, &effect) == nil {
			total += effect
		}
	}
	return total
}

// handleIslandSignIn answers IslandSignInCommand (CS_21310, island_id and
// pos 0) and IslandSelectgiftCommand (CS_21310, the island and the
// supply's position, 1..daily_gift_drop_num).
//
// A sign-in (once a day, DL_SIGNINSYS_CNT) drops the day's supplies, until
// midnight (the gift allocator's timer, GetZeroTime); the client only
// counts the sign-in, the supplies appear from the SC_21314 pushed to it
// and to the visitors. It counts for the SIGN_IN island tasks.
//
// Taking a supply on its own island is once a day after signing in
// (DL_SIGNINSYS_FETCH_CNT, which opens the invitations); on another island
// it must be shared with the commander (OPEN_SIGNIN or invited), not taken
// there yet (island_get_git_tip) and the day's count not reached
// (island_get_git_cnt_tip, DL_SIGNINSYS_OTHER_FETCH_CNT). The reply drops
// the supply's content; everyone on the island and the owner see it gone
// (SC_21314). GUESS: a supply holds one of island item daily_gift (4, the
// Building Permit); neither the Lua nor the wiki tells.
func (game *Game) handleIslandSignIn(client *connection.Client, req *protobuf.CS_21310) error {
	reply := &protobuf.SC_21311{}
	var update *protobuf.SC_21314
	islandID, pos := req.GetIslandId(), req.GetPos()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		own, err := tx.island()
		if err != nil {
			return err
		}
		now := serverNow()
		if islandID == 0 && pos == 0 {
			if own.daily(islandDailySignIn) != 0 {
				reply.Result = result(errRefused)
				return nil
			}
			own.setDaily(islandDailySignIn, 1)
			public := own.public()
			public.TreeGiftCount = proto.Uint32(islandSocialConfig.giftCount)
			public.TreeGiftVisitor = make([]uint32, islandSocialConfig.giftCount)
			public.TreeGiftTimestamp = proto.Uint32(uint32(nextDay(now).Unix()))
			update = islandSupplies(own, 1)
			tx.push(update)
			reply.Result = result(nil)
			return tx.islandTaskEvent(islandTaskSignIn, 1)
		}
		current, here := game.islandOf(client)
		if !here || current != islandID || pos == 0 || pos > islandSocialConfig.giftCount {
			reply.Result = result(errRefused)
			return nil
		}
		owner, err := tx.otherIsland(islandID)
		if err != nil {
			return err
		}
		if owner == nil {
			reply.Result = result(errRefused)
			return nil
		}
		public := owner.public()
		id := client.CommanderID
		taken := public.TreeGiftVisitor
		if int(pos) > len(taken) || taken[pos-1] != 0 || public.GetTreeGiftCount() == 0 ||
			public.GetTreeGiftTimestamp() <= uint32(now.Unix()) {
			reply.Result = result(errRefused)
			return nil
		}
		if owner == own {
			if own.daily(islandDailySignIn) == 0 || own.daily(islandDailyFetch) != 0 {
				reply.Result = result(errRefused)
				return nil
			}
			own.setDaily(islandDailyFetch, 1)
		} else {
			shared := slices.Contains(public.FlagList, islandOpenSignIn) || slices.Contains(public.TreeGiftInvited, id)
			if !shared || slices.Contains(taken, id) || own.daily(islandDailyOtherFetch) >= islandSupplyFetchMax(own) {
				reply.Result = result(errRefused)
				return nil
			}
			own.setDaily(islandDailyOtherFetch, own.daily(islandDailyOtherFetch)+1)
		}
		taken[pos-1] = id
		public.TreeGiftCount = proto.Uint32(public.GetTreeGiftCount() - 1)
		if err := tx.saveIsland(owner); err != nil {
			return err
		}
		drops := []*protobuf.DROPINFO{{
			Type: proto.Uint32(dropTypeIslandItem), Id: proto.Uint32(islandSocialConfig.giftItem), Number: proto.Uint32(1),
		}}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		update = islandSupplies(owner, 1)
		tx.push(update)
		reply.Result = result(nil)
		reply.DropList = drops
		return nil
	})
	if err != nil {
		return err
	}
	if update != nil {
		owner := update.GetIslandId()
		game.tellIsland(client, owner, update)
		if owner != client.CommanderID && !game.onIsland(owner, owner) {
			// the owner away keeps its supplies' tag (UpdateGiftTagCache)
			game.deliver(client, owner, update)
		}
	}
	return client.Send(islandStamped(reply))
}

// onIsland tells whether a commander is on an island.
func (game *Game) onIsland(commanderID, islandID uint32) bool {
	world := &game.islands
	world.mu.Lock()
	defer world.mu.Unlock()
	where, ok := world.where[commanderID]
	return ok && where == islandID
}

// handleIslandSignInInvite answers IslandSignInInvitationCommand
// (CS_21312): friends invited to take the commander's supplies, once it
// took its own (IslandSignInAgency.CanInvite). Each invited friend online is
// told (SC_21314 cmd 2: a "has invited you to collect resources" note, and
// it may take one when it is on the island already). GUESS: only friends
// may be invited, as the invitation page lists them.
func (game *Game) handleIslandSignInInvite(client *connection.Client, req *protobuf.CS_21312) error {
	reply := &protobuf.SC_21313{}
	var notice *protobuf.SC_21314
	var invited []uint32
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		if state.daily(islandDailyFetch) == 0 || len(req.GetFriendList()) == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		public := state.public()
		for _, id := range req.GetFriendList() {
			friends, err := tx.CountFriend(tx.ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: id})
			if err != nil {
				return err
			}
			if friends == 0 {
				reply.Result = result(errRefused)
				return errIslandUndo
			}
			if !slices.Contains(public.TreeGiftInvited, id) {
				public.TreeGiftInvited = append(public.TreeGiftInvited, id)
				invited = append(invited, id)
			}
		}
		notice = islandSupplies(state, 2)
		reply.Result = result(nil)
		return nil
	})
	if errors.Is(err, errIslandUndo) {
		return client.Send(islandStamped(reply))
	}
	if err != nil {
		return err
	}
	for _, id := range invited {
		game.deliver(client, id, notice)
	}
	return client.Send(islandStamped(reply))
}

// handleIslandGiftTags answers IslandGetGiftTagCommand (CS_21315): for the
// players the island's friend list shows, whether their supplies wait for
// the commander (IslandGiftTagInfo: key the player, value1 until when,
// value2 how many). Supplies not shared with the commander, or already
// taken by it, count 0; players without an island are left out.
func (game *Game) handleIslandGiftTags(client *connection.Client, req *protobuf.CS_21315) error {
	reply := &protobuf.SC_21316{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		for _, id := range req.GetUserIdList() {
			state, err := tx.otherIsland(id)
			if err != nil {
				return err
			}
			if state == nil {
				continue
			}
			public := state.public()
			count := public.GetTreeGiftCount()
			shared := id == client.CommanderID || slices.Contains(public.FlagList, islandOpenSignIn) ||
				slices.Contains(public.TreeGiftInvited, client.CommanderID)
			if !shared || slices.Contains(public.TreeGiftVisitor, client.CommanderID) {
				count = 0
			}
			reply.GiftList = append(reply.GiftList, &protobuf.KVDATA2{
				Key: proto.Uint32(id), Value1: proto.Uint32(public.GetTreeGiftTimestamp()), Value2: proto.Uint32(count),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(islandStamped(reply))
}
