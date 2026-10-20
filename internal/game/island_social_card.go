package game

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The island card (IslandCard: IslandSelfCardLayer, IslandOtherCardLayer):
// the island's photo, greeting, the achievements it shows, its counts, and
// what visitors leave on it: likes (once a day each, island_card_like_done)
// and rating tags (island_card_label, once a week each,
// island_card_label_done). What shows to others follows the card settings
// (PB_ISLAND_PRIVATE.flag_list: SHOW_CARD_SOCIAL, SHOW_CARD_LABEL), which
// the client reads itself.

// IslandCard.PHOTO_TYPE_*: CS_21328.type.
const (
	islandPhotoID      = 1 // an island_card_diy the commander owns
	islandPhotoAddress = 2
)

// islandCardWordMax is the longest greeting kept, in characters: the
// client checks 4..60 width units (nameValidityCheck) itself.
const islandCardWordMax = 60

// islandFlag is a card setting, 0 when unset.
func islandFlag(state *islandState, kind uint32) uint32 {
	for _, flag := range state.private().GetFlagList() {
		if flag.GetType() == kind {
			return flag.GetFlag()
		}
	}
	return 0
}

// handleIslandCard answers IslandGetCardDataCommand (CS_21326): a
// commander's island card as user_id's island has it, and what the asking
// commander did with it: liked today (good_flag), tagged this week
// (label_flag), has the owner in its own whitelist or blacklist. Shown
// achievements are the best stage the island finished of each group it
// picked (IslandAchievementAgency.UpdataAchLv); the counts are the
// achievements finished, the visits, likes, characters invited and
// collection entries (view_book.book_list). A card's name is its island's,
// the commander's name with island_default_name until it is named. An
// island nobody has yet is a new one.
func (game *Game) handleIslandCard(client *connection.Client, req *protobuf.CS_21326) error {
	ctx := context.Background()
	var reply *protobuf.SC_21327
	err := game.tx(ctx, client, func(tx *Tx) error {
		state, err := tx.otherIsland(req.GetUserId())
		if err != nil {
			return err
		}
		if state == nil {
			state = newIsland(req.GetUserId(), uint32(serverNow().Unix()))
		}
		own, err := tx.island()
		if err != nil {
			return err
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		public, private := state.public(), state.private()
		name := public.GetName()
		if name == "" {
			commander, err := tx.GetCommander(tx.ctx, req.GetUserId())
			if err == nil {
				var suffix []string
				_ = islandSetList("island_default_name", &suffix)
				name = commander.Name
				if len(suffix) > 0 {
					name += suffix[0]
				}
			}
		}
		now := serverNow()
		id := client.CommanderID
		reply = &protobuf.SC_21327{
			Name: proto.String(name), Picture: proto.String(social.Picture), VisitWord: proto.String(social.Word),
			Lv: proto.Uint32(public.GetLevel()), SocialFlag: proto.Uint32(islandFlag(state, islandFlagShowSocial)),
			LabelViewFlag: proto.Uint32(islandFlag(state, islandFlagShowLabel)),
			AchieveNum:    proto.Uint32(uint32(len(private.GetAchievementSys().GetFinishList()))),
			VisitNum:      proto.Uint32(social.Visits), GoodNum: proto.Uint32(social.Likes),
			ShipNum:   proto.Uint32(uint32(len(public.GetShipSys().GetShipList()))),
			BookNum:   proto.Uint32(uint32(len(private.GetViewBook().GetBookList()))),
			LabelFlag: proto.Uint32(boolFlag(social.LabeledBy[id] >= periodStart(now, week).Unix())),
			GoodFlag:  proto.Uint32(boolFlag(social.LikedBy[id] >= startOfDay(now).Unix())),
			WhiteFlag: proto.Uint32(boolFlag(slices.Contains(own.private().GetWhiteList(), req.GetUserId()))),
			BlackFlag: proto.Uint32(boolFlag(slices.Contains(own.private().GetBlackList(), req.GetUserId()))),
		}
		labels := make([]uint32, 0, len(social.Labels))
		for label := range social.Labels {
			labels = append(labels, label)
		}
		slices.Sort(labels)
		for _, label := range labels {
			reply.LabelList = append(reply.LabelList, &protobuf.PB_ISLAND_LABEL{Id: proto.Uint32(label), Num: proto.Uint32(social.Labels[label])})
		}
		best := islandBestAchievements(state)
		for _, group := range social.Groups {
			if achievement, ok := best[group]; ok {
				reply.AchieveList = append(reply.AchieveList, achievement)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// islandBestAchievements is the highest stage achievement the island
// finished in each group (IslandAchievementAgency.GetGotGroupMaxStageList).
func islandBestAchievements(state *islandState) map[uint32]uint32 {
	best := map[uint32]uint32{}
	for _, id := range state.private().GetAchievementSys().GetFinishList() {
		config, ok := islandSocialConfig.achievements[id]
		if !ok {
			continue
		}
		if current, ok := best[config.Group]; !ok || islandSocialConfig.achievements[current].Stage < config.Stage {
			best[config.Group] = id
		}
	}
	return best
}

// handleIslandCardPhoto answers IslandSetCardPhotoCommand (CS_21328): the
// card's photo, an island_card_diy id (type 1) the commander owns
// (PB_ISLAND_PRIVATE.image_list, or island_card_photo_default which every
// island has: IslandCardDiyAgency).
func (game *Game) handleIslandCardPhoto(client *connection.Client, req *protobuf.CS_21328) error {
	reply := &protobuf.SC_21329{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		picture := req.GetPicture()
		switch req.GetType() {
		case islandPhotoID:
			id, err := strconv.ParseUint(picture, 10, 32)
			owned := err == nil && (uint32(id) == islandSocialConfig.defaultPhoto ||
				slices.ContainsFunc(state.private().GetImageList(), func(image *protobuf.PB_CARD_IMAGE) bool { return image.GetId() == uint32(id) }))
			if _, known := islandSocialConfig.photos[uint32(id)]; !owned || !known {
				reply.Result = result(errRefused)
				return nil
			}
		case islandPhotoAddress:
			// GUESS: never sent by this client (IslandSelfCardMediator
			// always sets an id); kept as given
		default:
			reply.Result = result(errRefused)
			return nil
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		social.Picture = picture
		reply.Result = result(nil)
		return state.setSocial(social)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandCardWord answers IslandSetCardWordCommand (CS_21330): the
// card's greeting (island_card_default_word while empty).
func (game *Game) handleIslandCardWord(client *connection.Client, req *protobuf.CS_21330) error {
	reply := &protobuf.SC_21331{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		word := req.GetVisitWord()
		if utf8.RuneCountInString(word) > islandCardWordMax {
			reply.Result = result(errRefused)
			return nil
		}
		state, err := tx.island()
		if err != nil {
			return err
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		social.Word = word
		reply.Result = result(nil)
		return state.setSocial(social)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandSettingFlags answers IslandSettingFlagCommand (CS_21332):
// the card settings changed (PB_SET_FLAG: SHOW_CARD_SOCIAL 1,
// SHOW_CARD_LABEL 2, flag 0 or 1), which the client applies on result 0.
func (game *Game) handleIslandSettingFlags(client *connection.Client, req *protobuf.CS_21332) error {
	reply := &protobuf.SC_21333{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		private := state.private()
		for _, change := range req.GetFlagList() {
			if change.GetType() < islandFlagShowSocial || change.GetType() > islandFlagShowLabel || change.GetFlag() > 1 {
				reply.Result = result(errRefused)
				return errIslandUndo
			}
			found := false
			for _, flag := range private.FlagList {
				if flag.GetType() == change.GetType() {
					flag.Flag = proto.Uint32(change.GetFlag())
					found = true
				}
			}
			if !found {
				private.FlagList = append(private.FlagList, &protobuf.PB_SET_FLAG{Type: proto.Uint32(change.GetType()), Flag: proto.Uint32(change.GetFlag())})
			}
		}
		reply.Result = result(nil)
		return nil
	})
	if err != nil && !errors.Is(err, errIslandUndo) {
		return err
	}
	return client.Send(reply)
}

// visitIslandCard changes another commander's card on behalf of the
// client's (a like, a tag): refused for its own and for an island nobody
// has.
func (game *Game) visitIslandCard(client *connection.Client, userID uint32, change func(social *islandSocial) error) (*uint32, error) {
	var outcome *uint32
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		if userID == client.CommanderID {
			outcome = result(errRefused)
			return nil
		}
		state, err := tx.otherIsland(userID)
		if err != nil {
			return err
		}
		if state == nil {
			outcome = result(errRefused)
			return nil
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		if err := change(social); err != nil {
			outcome = result(err)
			return nil
		}
		if err := state.setSocial(social); err != nil {
			return err
		}
		outcome = result(nil)
		return tx.saveIsland(state)
	})
	return outcome, err
}

// handleIslandCardLike answers IslandGiveCardLikeCommand (CS_21334): a like
// on another commander's card, once a day (island_card_like_done).
func (game *Game) handleIslandCardLike(client *connection.Client, req *protobuf.CS_21334) error {
	outcome, err := game.visitIslandCard(client, req.GetUserId(), func(social *islandSocial) error {
		now := serverNow()
		if social.LikedBy[client.CommanderID] >= startOfDay(now).Unix() {
			return errRefused
		}
		if social.LikedBy == nil {
			social.LikedBy = map[uint32]int64{}
		}
		social.LikedBy[client.CommanderID] = now.Unix()
		social.Likes++
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21335{Result: outcome})
}

// handleIslandCardLabel answers IslandGiveCardLabelCommand (CS_21336): a
// rating tag (island_card_label) on another commander's card, once a week
// (island_card_label_done). GUESS: the week starts Monday 00:00 like the
// game's other weekly counts.
func (game *Game) handleIslandCardLabel(client *connection.Client, req *protobuf.CS_21336) error {
	label := req.GetLabelId()
	outcome, err := game.visitIslandCard(client, req.GetUserId(), func(social *islandSocial) error {
		now := serverNow()
		if _, ok := islandSocialConfig.labelIDs[label]; !ok || social.LabeledBy[client.CommanderID] >= periodStart(now, week).Unix() {
			return errRefused
		}
		if social.LabeledBy == nil {
			social.LabeledBy = map[uint32]int64{}
		}
		if social.Labels == nil {
			social.Labels = map[uint32]uint32{}
		}
		social.LabeledBy[client.CommanderID] = now.Unix()
		social.Labels[label]++
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21337{Result: outcome})
}

// handleIslandCardAchievements answers IslandSetCardAchvsCommand
// (CS_21338): the achievement groups the card shows, in order, at most
// island_card_achievement_limit, each one the island finished a stage of.
func (game *Game) handleIslandCardAchievements(client *connection.Client, req *protobuf.CS_21338) error {
	reply := &protobuf.SC_21339{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		groups := req.GetGroupList()
		state, err := tx.island()
		if err != nil {
			return err
		}
		best := islandBestAchievements(state)
		if len(groups) > int(islandSocialConfig.cardAchvMax) {
			reply.Result = result(errRefused)
			return nil
		}
		for i, group := range groups {
			if _, ok := best[group]; !ok || slices.Contains(groups[:i], group) {
				reply.Result = result(errRefused)
				return nil
			}
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		social.Groups = slices.Clone(groups)
		reply.Result = result(nil)
		return state.setSocial(social)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}
