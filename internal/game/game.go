// Package game answers the game server packets sent once a commander is
// logged in, starting with the player data load (CS_11001).
package game

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// mondayReference is the "monday_0oclock_timestamp" the official EN servers
// send (SC_10801, SC_11002): Monday 2020-11-23 00:00 in UTC-7. The client
// derives its weekly resets from it.
const mondayReference = 1606114800

type Game struct {
	DB *db.Store
}

// Register installs the game server packet handlers.
func (game *Game) Register() {
	packets.Handle(game.handleLoadPlayerData)
	packets.Handle(game.handleAuctionGameInit)
	packets.Handle(game.handleAppreciationVersion)
	packets.Handle(game.handleGuideIndex)
	packets.Handle(game.handleStoryRead)
	packets.Handle(game.handleMetaPoints)
	packets.Handle(game.handleSeasonInfo)
	packets.Handle(game.handleGuildInfo)
	packets.Handle(game.handlePublicGuildTech)
	packets.Handle(game.handleMiniGameHubs)
	packets.Handle(game.handleEmojiInfo)
	packets.Handle(game.handleLimitChallenge)
	packets.Handle(game.handleEducate)
	packets.Handle(game.handleNewEducate)
	packets.Handle(game.handleJuusGroups)
	packets.Handle(game.handleSecondaryPassword)
	packets.Handle(game.handleGuildUserInfo)
	packets.Handle(game.handleEducateEndings)
	packets.Handle(game.handleChargeList)
	packets.Handle(game.handleChangeChatRoom)
	packets.Handle(game.handleClientReport)
	packets.Handle(game.handleTracking)
}

// handleLoadPlayerData answers CS_11001 (LoadPlayerDataCommand): everything
// pushed before SC_11002 lands in the proxies the client just registered,
// SC_11002 then starts them.
func (game *Game) handleLoadPlayerData(client *connection.Client, req *protobuf.CS_11001) error {
	if client.CommanderID == 0 {
		return fmt.Errorf("CS_11001 without a commander")
	}
	ctx := context.Background()
	commander, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	ships, err := game.DB.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return err
	}
	stories, err := game.DB.ListStories(ctx, commander.ID)
	if err != nil {
		return err
	}
	resources, err := game.DB.ListResources(ctx, commander.ID)
	if err != nil {
		return err
	}
	items, err := game.DB.ListItems(ctx, commander.ID)
	if err != nil {
		return err
	}
	info := playerInfo(commander, ships, stories)
	info.ResourceList = resourceList(resources)
	if err := client.Send(info); err != nil {
		return err
	}
	shipList := &protobuf.SC_12001{Shiplist: make([]*protobuf.SHIPINFO, len(ships))}
	for i, ship := range ships {
		shipList.Shiplist[i] = shipInfo(ship)
	}
	if err := client.Send(shipList); err != nil {
		return err
	}
	for _, push := range taskPushes() {
		if err := client.Send(push); err != nil {
			return err
		}
	}
	if err := client.Send(bag(items)); err != nil {
		return err
	}
	if err := client.Send(buildDocks()); err != nil {
		return err
	}
	if err := client.Send(technology()); err != nil {
		return err
	}
	// TechnologyProxy blueprints (PR ships): none
	if err := client.Send(&protobuf.SC_63100{
		ColdTime:                 proto.Uint32(0),
		DailyCatchupStrengthen:   proto.Uint32(0),
		DailyCatchupStrengthenUr: proto.Uint32(0),
	}); err != nil {
		return err
	}
	if err := client.Send(dorm()); err != nil {
		return err
	}
	if err := client.Send(navalAcademy()); err != nil {
		return err
	}
	// CommanderManualProxy: no handbook progress (every page starts at 0)
	if err := client.Send(&protobuf.SC_22300{}); err != nil {
		return err
	}
	// MailProxy: empty mailbox
	if err := client.Send(&protobuf.SC_30001{UnreadNumber: proto.Uint32(0), TotalNumber: proto.Uint32(0)}); err != nil {
		return err
	}
	// CompensateProxy: no compensation mail
	if err := client.Send(&protobuf.SC_30101{Number: proto.Uint32(0), MaxTimestamp: proto.Uint32(0)}); err != nil {
		return err
	}
	// ChapterAutoProxy reads its auto battle state from here
	if err := client.Send(&protobuf.SC_13001{
		Oil:          proto.Uint32(0),
		TimeAcc:      proto.Uint32(0),
		ExtraTimeMax: proto.Uint32(0),
	}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11002{
		Timestamp:               proto.Uint32(uint32(time.Now().Unix())),
		Monday_0OclockTimestamp: proto.Uint32(mondayReference),
		ShipCount:               proto.Uint32(uint32(len(ships))),
	})
}

// playerInfo is SC_11003, read by PlayerProxy into a Player (model/vo/player.lua).
// The secretary (character) and the avatar (display.icon) are the first ship
// until they can be changed.
func playerInfo(commander db.Commander, ships []db.Ship, stories []uint32) *protobuf.SC_11003 {
	info := &protobuf.SC_11003{
		Id:                        proto.Uint32(commander.ID),
		Name:                      proto.String(commander.Name),
		Level:                     proto.Uint32(1), // user_level row
		Exp:                       proto.Uint32(0),
		AttackCount:               proto.Uint32(0),
		WinCount:                  proto.Uint32(0),
		Adv:                       proto.String(""),
		ShipBagMax:                proto.Uint32(0),
		EquipBagMax:               proto.Uint32(0),
		GmFlag:                    proto.Uint32(0),
		Rank:                      proto.Uint32(0),
		PvpAttackCount:            proto.Uint32(0),
		PvpWinCount:               proto.Uint32(0),
		CollectAttackCount:        proto.Uint32(0),
		GuideIndex:                proto.Uint32(commander.GuideIndex),
		BuyOilCount:               proto.Uint32(0),
		ChatRoomId:                proto.Uint32(1),
		MaxRank:                   proto.Uint32(0),
		RegisterTime:              proto.Uint32(uint32(commander.RegisteredAt)),
		ShipCount:                 proto.Uint32(uint32(len(ships))),
		AccPayLv:                  proto.Uint32(0),
		GuildWaitTime:             proto.Uint32(0),
		ChatMsgBanTime:            proto.Uint32(0),
		CommanderBagMax:           proto.Uint32(0),
		Rmb:                       proto.Uint32(0),
		Appreciation:              &protobuf.APPRECIATIONINFO{MusicNo: proto.Uint32(0), MusicMode: proto.Uint32(0)},
		ThemeUploadNotAllowedTime: proto.Uint32(0),
		RandomShipMode:            proto.Uint32(0),
		MarryShip:                 proto.Uint32(0),
		ChildDisplay:              proto.Uint32(0),
		Cover:                     &protobuf.LIVINGAREA_COVER{Id: proto.Uint32(0)},
		MailStoreroomLv:           proto.Uint32(0),
		BattleUi:                  proto.Uint32(0), // item_data_battleui "Standard"
		NewGuideIndex:             proto.Uint32(commander.NewGuideIndex),
		StoryList:                 stories,
		LoadingPicOpenFlag:        proto.Uint32(0),
	}
	if len(ships) > 0 {
		info.Character = []*protobuf.KVDATA{{Key: proto.Uint32(ships[0].ID), Value: proto.Uint32(0)}}
		info.Display = &protobuf.DISPLAYINFO{
			Icon: proto.Uint32(ships[0].TemplateID), Skin: proto.Uint32(0), IconFrame: proto.Uint32(0), ChatFrame: proto.Uint32(0),
			IconTheme: proto.Uint32(0), MarryFlag: proto.Uint32(0), TransformFlag: proto.Uint32(0),
		}
	}
	return info
}

// navalAcademy is SC_22001, NavalAcademyProxy's state: oil field, gold field
// and classroom at level 1 (their template tables start there), no lessons.
func navalAcademy() *protobuf.SC_22001 {
	return &protobuf.SC_22001{
		OilWellLevel:       proto.Uint32(1),
		OilWellLvUpTime:    proto.Uint32(0),
		GoldWellLevel:      proto.Uint32(1),
		GoldWellLvUpTime:   proto.Uint32(0),
		ClassLv:            proto.Uint32(1),
		ClassLvUpTime:      proto.Uint32(0),
		Class:              &protobuf.NAVALACADEMY_CLASS{Proficiency: proto.Uint32(0)},
		SkillClassNum:      proto.Uint32(0),
		DailyFinishBuffCnt: proto.Uint32(0),
	}
}

// technology is SC_63000, TechnologyProxy's research state: nothing started,
// no catch-up research.
func technology() *protobuf.SC_63000 {
	return &protobuf.SC_63000{
		RefreshFlag: proto.Uint32(0),
		Catchup:     &protobuf.TECHNOLOGYCATCHUP{Version: proto.Uint32(0), Target: proto.Uint32(0)},
	}
}

// dorm is SC_19001, the backyard (DormProxy): level 1, one floor, two
// training slots, no food, ships or furniture.
func dorm() *protobuf.SC_19001 {
	return &protobuf.SC_19001{
		Lv:                   proto.Uint32(1),
		Food:                 proto.Uint32(0),
		FoodMaxIncrease:      proto.Uint32(0),
		FoodMaxIncreaseCount: proto.Uint32(0),
		FloorNum:             proto.Uint32(1),
		ExpPos:               proto.Uint32(2),
		NextTimestamp:        proto.Uint32(0),
		LoadExp:              proto.Uint32(0),
		LoadFood:             proto.Uint32(0),
		LoadTime:             proto.Uint32(0),
		Name:                 proto.String(""),
	}
}
