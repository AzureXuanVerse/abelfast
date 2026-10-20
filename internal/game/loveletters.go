package game

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// loverCharacter is a lover_character_template row (LoveLetter's config,
// model/vo/loveletter.lua): a ship group's medal gains a level every exp_up
// exp, up to exp_upper_limit; relate_group_id are the other groups of the
// same character (retrofits, METAs...) whose Valentine gifts count for it.
type loverCharacter struct {
	ExpUp         uint32   `json:"exp_up"`
	ExpUpperLimit uint32   `json:"exp_upper_limit"`
	RelateGroupID []uint32 `json:"relate_group_id"`
}

// maxLevel is LoveLetter:GetMaxLevel.
func (character loverCharacter) maxLevel() uint32 {
	if character.ExpUp == 0 {
		return 0
	}
	return character.ExpUpperLimit / character.ExpUp
}

// loverLetter is a lover_letter_content row: the Valentine letter of a ship
// group for a year, and the gifts (items) of that year that realize it.
type loverLetter struct {
	LoveItem  []uint32 `json:"love_item"`
	ShipGroup uint32   `json:"ship_group"`
	Year      uint32   `json:"year"`
}

// loverReward is a lover_reward row: what the sum of the medals' levels
// unlocks once it reaches total_level.
type loverReward struct {
	ShowReward [][]uint32 `json:"show_reward"`
	TotalLevel uint32     `json:"total_level"`
}

// loveGiftKey is LoveLetterProxy:GetLoveLetterItemDic's "<item>_<group>"
// key: a gift item and the group recorded with it (0, the letter's group or
// one of its relate_group_id).
type loveGiftKey struct{ item, group uint32 }

var (
	loverCharacters map[uint32]loverCharacter
	loverLetters    map[uint32]loverLetter
	loverRewards    map[uint32]loverReward

	// loveGiftOwners is GetLoveLetterItemDic's letterItemDic: per gift key,
	// the ship group whose letter each year's gift realizes
	loveGiftOwners map[loveGiftKey]map[uint32]uint32
	// loveLetterByYear is its letterIdMap: the letter of a group and a year
	loveLetterByYear map[[2]uint32]uint32
	// loveLettersByGroup is lover_letter_content.get_id_list_by_ship_group
	loveLettersByGroup map[uint32][]uint32
	// loveLetterTexts are the letters' text, server content
	// (love_letters.json, {"<letter id>": "text"}), optional
	loveLetterTexts map[uint32]string
)

func loadLoveLetterData() error {
	if err := gamedata.LoadAll(
		gamedata.Load("lover_character_template", &loverCharacters),
		gamedata.Load("lover_letter_content", &loverLetters),
		gamedata.Load("lover_reward", &loverRewards),
	); err != nil {
		return err
	}
	loveGiftOwners = map[loveGiftKey]map[uint32]uint32{}
	loveLetterByYear = map[[2]uint32]uint32{}
	loveLettersByGroup = map[uint32][]uint32{}
	ids := make([]uint32, 0, len(loverLetters))
	for id := range loverLetters {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		letter := loverLetters[id]
		loveLettersByGroup[letter.ShipGroup] = append(loveLettersByGroup[letter.ShipGroup], id)
		character, ok := loverCharacters[letter.ShipGroup]
		if !ok {
			continue
		}
		loveLetterByYear[[2]uint32{letter.ShipGroup, letter.Year}] = id
		groups := append([]uint32{0, letter.ShipGroup}, character.RelateGroupID...)
		for _, item := range letter.LoveItem {
			for _, group := range groups {
				key := loveGiftKey{item, group}
				if loveGiftOwners[key] == nil {
					loveGiftOwners[key] = map[uint32]uint32{}
				}
				loveGiftOwners[key][letter.Year] = letter.ShipGroup
			}
		}
	}
	loveLetterTexts = map[uint32]string{}
	raw, err := gamedata.ServerFile("love_letters.json")
	if raw == nil || err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &loveLetterTexts); err != nil {
		return fmt.Errorf("love letters: %w", err)
	}
	return nil
}

// giftLetter is the ship group a recorded gift raises and the letter it
// realizes; ok is false for a record that matches no letter.
func giftLetter(gift *protobuf.PT_OLD_LOVER_ITEM) (group, letter uint32, ok bool) {
	group, ok = loveGiftOwners[loveGiftKey{gift.GetItemId(), gift.GetGroupId()}][gift.GetYear()]
	if !ok {
		return 0, 0, false
	}
	letter, ok = loveLetterByYear[[2]uint32{group, gift.GetYear()}]
	return group, letter, ok
}

// loveLetterState is a commander's LoveLetterProxy, as stored.
type loveLetterState struct {
	medals   map[uint32]db.ListLoveLetterMedalsRow
	unlocked map[uint32]bool
	gifts    []*protobuf.PT_OLD_LOVER_ITEM
	rewarded map[uint32]bool
}

func (tx *Tx) loveLetters() (*loveLetterState, error) {
	state := &loveLetterState{medals: map[uint32]db.ListLoveLetterMedalsRow{}, unlocked: map[uint32]bool{}, rewarded: map[uint32]bool{}}
	medals, err := tx.ListLoveLetterMedals(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	for _, medal := range medals {
		state.medals[medal.GroupID] = medal
	}
	letters, err := tx.ListLoveLetters(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	for _, id := range letters {
		state.unlocked[id] = true
	}
	gifts, err := tx.ListLoveLetterGifts(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	for _, gift := range gifts {
		state.gifts = append(state.gifts, &protobuf.PT_OLD_LOVER_ITEM{ItemId: proto.Uint32(gift.ItemID), GroupId: proto.Uint32(gift.GroupID), Year: proto.Uint32(gift.Year)})
	}
	rewards, err := tx.ListLoveLetterRewards(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	for _, id := range rewards {
		state.rewarded[id] = true
	}
	return state, nil
}

// totalLevel is LoveLetterProxy.levelAll: every medal's GetDisplayLevel.
func (state *loveLetterState) totalLevel() uint32 {
	total := uint32(0)
	for group, medal := range state.medals {
		total += min(medal.Level, loverCharacters[group].maxLevel())
	}
	return total
}

// giftLetters is LoveLetterProxy:GetRecordGiftLetters: the letters the
// recorded gifts realized for group, in the order they were recorded.
func (state *loveLetterState) giftLetters(group uint32) []uint32 {
	var letters []uint32
	for _, gift := range state.gifts {
		if owner, letter, ok := giftLetter(gift); ok && owner == group {
			letters = append(letters, letter)
		}
	}
	return letters
}

// letterList is LoveLetter:GetLetterList: the gift letters first, then the
// group's other letters.
func (state *loveLetterState) letterList(group uint32) []uint32 {
	letters := state.giftLetters(group)
	for _, id := range loveLettersByGroup[group] {
		if !slices.Contains(letters, id) {
			letters = append(letters, id)
		}
	}
	return letters
}

// handleLoveLetters answers GetAllLoveLetterLevelDataCommand (CS_12406, the
// main scene's MainRequestLoveLetterDataSequence), LoveLetterProxy:
// SetGroupList: medal_list the medals (a ship group's exp and level; those
// with exp are shown in the Medals gallery's love letter page),
// letter_list the letters each group has open (LoveLetter.unlockLetterDic
// counts them: the opened ones once, plus once per recorded gift, which
// opens its letter as UpdateRealizeGift does), converted_list the recorded
// gifts, rewarded_list the lover_reward ids claimed. converted_letter_list
// is not read by the client; it gets the gift letters. The official
// commander's capture is empty (no love letters).
func (game *Game) handleLoveLetters(client *connection.Client, req *protobuf.CS_12406) error {
	reply := &protobuf.SC_12407{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.loveLetters()
		if err != nil {
			return err
		}
		reply.ConvertedList = state.gifts
		for id := range state.rewarded {
			reply.RewardedList = append(reply.RewardedList, id)
		}
		slices.Sort(reply.RewardedList)
		letters := map[uint32][]uint32{}
		for id := range state.unlocked {
			if letter, ok := loverLetters[id]; ok {
				letters[letter.ShipGroup] = append(letters[letter.ShipGroup], id)
			}
		}
		groups := make([]uint32, 0, len(state.medals))
		for group := range state.medals {
			groups = append(groups, group)
		}
		for group := range letters {
			if _, ok := state.medals[group]; !ok {
				groups = append(groups, group)
			}
		}
		slices.Sort(groups)
		for _, group := range groups {
			if medal, ok := state.medals[group]; ok {
				reply.MedalList = append(reply.MedalList, &protobuf.PT_LOVE_LETTER_MEDAL{GroupId: proto.Uint32(group), Exp: proto.Uint32(medal.Exp), Level: proto.Uint32(medal.Level)})
			}
			opened := letters[group]
			slices.Sort(opened)
			gifted := state.giftLetters(group)
			if list := append(opened, gifted...); len(list) > 0 {
				reply.LetterList = append(reply.LetterList, &protobuf.PT_SHIP_LOVE_LETTER{GroupId: proto.Uint32(group), LetterIdList: list})
			}
			if len(gifted) > 0 {
				reply.ConvertedLetterList = append(reply.ConvertedLetterList, &protobuf.PT_SHIP_LOVE_LETTER{GroupId: proto.Uint32(group), LetterIdList: gifted})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleUnlockLoveLetter answers UnlockLoveLetterCommand (CS_12400, tapping
// a sealed letter in LoveLetterDisplayLayer, from the Memories album's love
// letter page): opens a lover_letter_content letter. As the client checks
// (LoveLetter:CanUnlockLetter), the letter's place in its group's
// GetLetterList must be within the medal's level. Opening one already open
// does nothing. Built from the Lua alone.
func (game *Game) handleUnlockLoveLetter(client *connection.Client, req *protobuf.CS_12400) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		letter, ok := loverLetters[req.GetId()]
		if !ok {
			return errRefused
		}
		state, err := tx.loveLetters()
		if err != nil {
			return err
		}
		index := slices.Index(state.letterList(letter.ShipGroup), req.GetId())
		if index < 0 || uint32(index) >= state.medals[letter.ShipGroup].Level {
			return errRefused
		}
		return tx.UnlockLoveLetter(tx.ctx, db.UnlockLoveLetterParams{CommanderID: tx.commanderID, LetterID: req.GetId()})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12401{Result: result(err)})
}

// handleLoveLetterLevelReward answers GetLoveLetterLevelRewardCommand
// (CS_12402, "claim all" on the total level of the Medals gallery's love
// letter page): claims lover_reward ids, each once and only when the sum of
// the medals' levels reaches its total_level (LoveLetterProxy:
// CanGetReward). drop_list is their show_reward, the only reward the
// tables have, granted here. Built from the Lua alone.
func (game *Game) handleLoveLetterLevelReward(client *connection.Client, req *protobuf.CS_12402) error {
	reply := &protobuf.SC_12403{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.loveLetters()
		if err != nil {
			return err
		}
		total := state.totalLevel()
		var drops []*protobuf.DROPINFO
		for _, id := range req.GetIdList() {
			reward, ok := loverRewards[id]
			if !ok || total < reward.TotalLevel {
				return errRefused
			}
			added, err := tx.AddLoveLetterReward(tx.ctx, db.AddLoveLetterRewardParams{CommanderID: tx.commanderID, RewardID: id})
			if err != nil {
				return err
			}
			if added == 0 {
				return errRefused
			}
			drops = append(drops, configDrops(reward.ShowReward)...)
		}
		if err := tx.grantDrops(drops); err != nil {
			return err
		}
		reply.DropList = drops
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleRealizeLoveLetterGift answers RealizeLoveLetterGiftCommand
// (CS_12404, LoveLetterGiftCollectLayer, which the Medals gallery's love
// letter page opens when the bag holds Valentine gifts not recorded yet):
// item_list replaces the recorded gifts, at most one per year, each an item
// of the bag (as many records of an item as owned) matching a letter
// (GetLoveLetterItemDic). The items are kept. As UpdateRealizeGift does,
// each gift the list adds raises its group's medal by exp_up exp and one
// level and opens that year's letter; each it drops lowers them back.
// Built from the Lua alone.
func (game *Game) handleRealizeLoveLetterGift(client *connection.Client, req *protobuf.CS_12404) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		items, err := tx.ListItems(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		owned := map[uint32]uint32{}
		for _, item := range items {
			owned[item.ItemID] = item.Count
		}
		years := map[uint32]bool{}
		delta := map[uint32]int64{}
		for _, gift := range req.GetItemList() {
			group, _, ok := giftLetter(gift)
			if !ok || years[gift.GetYear()] || owned[gift.GetItemId()] == 0 {
				return errRefused
			}
			years[gift.GetYear()] = true
			owned[gift.GetItemId()]--
			delta[group]++
		}
		state, err := tx.loveLetters()
		if err != nil {
			return err
		}
		for _, gift := range state.gifts {
			if group, _, ok := giftLetter(gift); ok {
				delta[group]--
			}
		}
		for group, n := range delta {
			if n == 0 {
				continue
			}
			medal := state.medals[group]
			exp := max(int64(medal.Exp)+n*int64(loverCharacters[group].ExpUp), 0)
			level := max(int64(medal.Level)+n, 0)
			if err := tx.SetLoveLetterMedal(tx.ctx, db.SetLoveLetterMedalParams{CommanderID: tx.commanderID, GroupID: group, Exp: uint32(exp), Level: uint32(level)}); err != nil {
				return err
			}
		}
		if err := tx.ClearLoveLetterGifts(tx.ctx, tx.commanderID); err != nil {
			return err
		}
		for i, gift := range req.GetItemList() {
			if err := tx.AddLoveLetterGift(tx.ctx, db.AddLoveLetterGiftParams{CommanderID: tx.commanderID, Pos: uint32(i), ItemID: gift.GetItemId(), GroupID: gift.GetGroupId(), Year: gift.GetYear()}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12405{Result: result(err)})
}

// handleLoveLetterLevelUp answers LoveLetterLevelUpCommand (CS_12408,
// opening a ship's card in the Medals gallery's love letter page,
// LoveLetterGiftLevelDisplayMediator): the medal takes every level its exp
// pays for (LoveLetter:MaxLevelUp), free, when it has at least the next
// one's (CanLevelUp). Built from the Lua alone.
func (game *Game) handleLoveLetterLevelUp(client *connection.Client, req *protobuf.CS_12408) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		character, ok := loverCharacters[req.GetGroupId()]
		if !ok || character.ExpUp == 0 {
			return errRefused
		}
		medals, err := tx.ListLoveLetterMedals(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		for _, medal := range medals {
			if medal.GroupID != req.GetGroupId() {
				continue
			}
			if medal.Exp < (medal.Level+1)*character.ExpUp {
				return errRefused
			}
			return tx.SetLoveLetterMedal(tx.ctx, db.SetLoveLetterMedalParams{CommanderID: tx.commanderID, GroupID: medal.GroupID, Exp: medal.Exp, Level: medal.Exp / character.ExpUp})
		}
		return errRefused
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_12409{Ret: result(err)})
}

// handleLoveLetterContent answers RequestLoveLetterContentCommand
// (CS_12410, LoveLetterDisplayMediator, for each letter of the group shown
// whose text it doesn't have): the letter's text. No client table has it
// (lover_letter_content only links letters to ships, years and gifts): it
// is the official server's, so belfast sends the text of server content
// love_letters.json when given, an empty letter otherwise. The client
// strips backslashes.
func (game *Game) handleLoveLetterContent(client *connection.Client, req *protobuf.CS_12410) error {
	return client.Send(&protobuf.SC_12411{Content: proto.String(loveLetterTexts[req.GetLetterId()])})
}
