package game

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// registerPlayer installs the player area's handlers.
func (game *Game) registerPlayer() {
	packets.Handle(game.handleLoadPlayerData)
	// settings.go
	packets.Handle(game.handleSecondaryPassword)
	packets.Handle(game.handleChangeChatRoom)
	packets.Handle(game.handleClientReport)
	packets.Handle(game.handleCommanderHome)
	packets.Handle(game.handleWorldBoss)
	packets.Handle(game.handleMetaPTData)
	// minigames.go
	packets.Handle(game.handleAuctionGameInit)
	packets.Handle(game.handleAppreciationVersion)
	packets.Handle(game.handleMiniGameHubs)
	packets.Handle(game.handleEmojiInfo)
	packets.Handle(game.handleLimitChallenge)
	packets.Handle(game.handleJuusGroups)
	// educate.go
	packets.Handle(game.handleEducate)
	packets.Handle(game.handleNewEducate)
	packets.Handle(game.handleEducateEndings)
	// guild.go
	packets.Handle(game.handleGuildInfo)
	packets.Handle(game.handlePublicGuildTech)
	packets.Handle(game.handleGuildUserInfo)
	packets.Handle(game.handleGuildShop)
	// exercise.go
	packets.Handle(game.handleSeasonInfo)
	packets.Handle(game.handleMilitaryShop)
	// shops.go
	packets.Handle(game.handleChargeList)
	packets.Handle(game.handleMedalShop)
	packets.Handle(game.handleShopStreet)
	packets.Handle(game.handleMiniGameShop)
	// activities.go
	packets.Handle(game.handleActivityOperation)
}

// player_resource ids, next to ResourceGold / ResourceOil
const (
	ResourceOilField  = 5 // oil waiting in the academy's oil field
	ResourceGoldField = 7 // gold waiting in the academy's merchant
	ResourceGuildCoin = 8
)

// starterResources are a new commander's resources on the official servers
// (captured SC_11003.resource_list).
var starterResources = []struct{ id, amount uint32 }{
	{ResourceGold, 3000},
	{ResourceOil, 500},
	{ResourceOilField, 5},
	{ResourceGoldField, 5},
	{ResourceGuildCoin, 4000},
}

// starterFurniture is the backyard a new commander gets, the "Run-down"
// theme's free pieces (captured SC_19001.furniture_id_list).
var starterFurniture = []uint32{1001, 1002, 1102, 1103, 1104, 1105, 1106, 1107, 1111, 1112, 1301, 1302, 1304, 1306}

// grantPlayerStarter sets up what a new commander starts with besides ships:
// resources, backyard furniture, the starter ship as secretary and avatar.
// The capacities, rank, chat room... are column defaults (0005_player.sql).
func grantPlayerStarter(ctx context.Context, queries *db.Queries, commander db.Commander) error {
	for _, resource := range starterResources {
		if err := queries.AddResource(ctx, db.AddResourceParams{CommanderID: commander.ID, ResourceID: resource.id, Amount: resource.amount}); err != nil {
			return err
		}
	}
	for _, id := range starterFurniture {
		if err := queries.AddFurniture(ctx, db.AddFurnitureParams{CommanderID: commander.ID, FurnitureID: id, Count: 1}); err != nil {
			return err
		}
	}
	if err := queries.SetCommanderIcon(ctx, db.SetCommanderIconParams{Icon: commander.StarterShip, ID: commander.ID}); err != nil {
		return err
	}
	if err := queries.SetDormLoadTime(ctx, db.SetDormLoadTimeParams{DormLoadTime: uint32(commander.RegisteredAt), ID: commander.ID}); err != nil {
		return err
	}
	ships, err := queries.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return err
	}
	if ship, ok := starterShip(commander, ships); ok {
		return queries.SetSecretary(ctx, db.SetSecretaryParams{CommanderID: commander.ID, Position: 1, ShipID: ship.ID})
	}
	return nil
}

// starterShip finds the ship the commander picked at creation.
func starterShip(commander db.Commander, ships []db.Ship) (db.Ship, bool) {
	for _, ship := range ships {
		if ship.TemplateID == commander.StarterShip {
			return ship, true
		}
	}
	return db.Ship{}, false
}

// serverNow is the current time on the EN servers' clock.
func serverNow() time.Time {
	return time.Now().In(serverZone)
}

// nextMonth returns the start of the month after now, server time: when the
// monthly shops refresh.
func nextMonth(now time.Time) time.Time {
	local := now.In(serverZone)
	return time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, serverZone)
}

// nextDay returns the next midnight, server time.
func nextDay(now time.Time) time.Time {
	local := now.In(serverZone)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, serverZone)
}

// playerLoginPushes are the player area's pushes of the login burst
// (CS_11001), in the official order (see loginBurstOrder).
func (game *Game) playerLoginPushes(ctx context.Context, commander db.Commander) ([]proto.Message, error) {
	ships, err := game.DB.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	info, err := game.playerInfo(ctx, commander, ships)
	if err != nil {
		return nil, err
	}
	backyard, err := game.dorm(ctx, commander)
	if err != nil {
		return nil, err
	}
	activities, err := game.activityList(ctx, commander)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	timestamp := proto.Uint32(uint32(now.Unix()))
	return []proto.Message{
		// PlayerProxy: server clock
		&protobuf.SC_11000{Timestamp: timestamp, Monday_0OclockTimestamp: proto.Uint32(mondayReference)},
		info,
		// PlayerProxy: no benefit buffs
		&protobuf.SC_11015{},
		// RefluxProxy: not a returning player
		&protobuf.SC_11752{Active: proto.Uint32(0)},
		// SystemTipProxy (island delegation tips): nothing to collect
		protocol.FillRequired(&protobuf.SC_21536{}),
		navalAcademy(),
		// GameRoomProxy: no game room coins or scores
		protocol.FillRequired(&protobuf.SC_26120{}),
		// CommanderProxy: no cat commanders, boxes or presets
		protocol.FillRequired(&protobuf.SC_25001{}),
		// ShipSkinProxy: no skins owned
		&protobuf.SC_12201{},
		technology(),
		// TechnologyProxy blueprints (PR ships): none
		protocol.FillRequired(&protobuf.SC_63100{}),
		// TechnologyNationProxy: no fleet tech
		&protobuf.SC_64000{},
		// DailyLevelProxy: no daily challenge played
		protocol.FillRequired(&protobuf.SC_13201{}),
		// ShopsProxy: nothing bought this month
		&protobuf.SC_16200{Month: proto.Uint32(uint32(serverNow().Month()))},
		// WorldProxy: Operation Siren open, not started (captured)
		&protobuf.SC_33114{IsWorldOpen: proto.Uint32(1), Progress: proto.Uint32(0)},
		backyard,
		// ApartmentProxy / Dorm3dInsProxy: no dorm 3D
		protocol.FillRequired(&protobuf.SC_28000{}),
		// MailProxy: empty mailbox
		&protobuf.SC_30001{UnreadNumber: proto.Uint32(0), TotalNumber: proto.Uint32(0)},
		// CompensateProxy: no compensation mail
		&protobuf.SC_30101{Number: proto.Uint32(0), MaxTimestamp: proto.Uint32(0)},
		// FriendProxy: no friends or requests
		&protobuf.SC_50000{},
		activities,
		// ActivityPermanentProxy: no permanent (archived) event running
		&protobuf.SC_11210{},
		// ActivityRemasterProxy: no remaster ticket timer
		protocol.FillRequired(&protobuf.SC_11213{}),
		// ServerNoticeProxy: belfast has no announcements
		&protobuf.SC_11300{},
		// PlayerProxy: everything is loaded
		&protobuf.SC_11002{
			Timestamp:               timestamp,
			Monday_0OclockTimestamp: proto.Uint32(mondayReference),
			ShipCount:               proto.Uint32(uint32(len(ships))),
		},
	}, nil
}

// playerInfo is SC_11003, read by PlayerProxy into a Player (model/vo/player.lua).
func (game *Game) playerInfo(ctx context.Context, commander db.Commander, ships []db.Ship) (*protobuf.SC_11003, error) {
	stories, err := game.DB.ListStories(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	resources, err := game.DB.ListResources(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	secretaries, err := game.DB.ListSecretaries(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	characters := make([]*protobuf.KVDATA, len(secretaries))
	for i, secretary := range secretaries {
		characters[i] = &protobuf.KVDATA{Key: proto.Uint32(secretary.ShipID), Value: proto.Uint32(secretary.PhantomID)}
	}
	if len(characters) == 0 {
		// commanders created before secretaries were stored
		if ship, ok := starterShip(commander, ships); ok {
			characters = []*protobuf.KVDATA{{Key: proto.Uint32(ship.ID), Value: proto.Uint32(0)}}
		}
	}
	icon := commander.Icon
	if icon == 0 {
		icon = commander.StarterShip
	}
	return protocol.FillRequired(&protobuf.SC_11003{
		Id:              proto.Uint32(commander.ID),
		Name:            proto.String(commander.Name),
		Level:           proto.Uint32(commander.Level),
		Exp:             proto.Uint32(commander.Exp),
		ResourceList:    resourceList(resources),
		Adv:             proto.String(commander.Manifesto),
		Character:       characters,
		ShipBagMax:      proto.Uint32(commander.ShipBagMax),
		EquipBagMax:     proto.Uint32(commander.EquipBagMax),
		Rank:            proto.Uint32(commander.Rank),
		GuideIndex:      proto.Uint32(commander.GuideIndex),
		ChatRoomId:      proto.Uint32(commander.ChatRoomID),
		RegisterTime:    proto.Uint32(uint32(commander.RegisteredAt)),
		ShipCount:       proto.Uint32(uint32(len(ships))),
		CommanderBagMax: proto.Uint32(commander.CommanderBagMax),
		Display:         protocol.FillRequired(&protobuf.DISPLAYINFO{Icon: proto.Uint32(icon)}),
		Appreciation:    protocol.FillRequired(&protobuf.APPRECIATIONINFO{}),
		MailStoreroomLv: proto.Uint32(commander.MailStoreroomLv),
		BattleUi:        proto.Uint32(0), // item_data_battleui "Standard"
		NewGuideIndex:   proto.Uint32(commander.NewGuideIndex),
		StoryList:       stories,
	}), nil
}

// navalAcademy is SC_22001, NavalAcademyProxy's state: oil field, merchant
// and classroom at level 1 (their template tables start there), two skill
// classes (captured), no lessons.
func navalAcademy() *protobuf.SC_22001 {
	return protocol.FillRequired(&protobuf.SC_22001{
		OilWellLevel:  proto.Uint32(1),
		GoldWellLevel: proto.Uint32(1),
		ClassLv:       proto.Uint32(1),
		SkillClassNum: proto.Uint32(2),
	})
}

// technology is SC_63000, TechnologyProxy's research state: nothing started,
// no catch-up research.
func technology() *protobuf.SC_63000 {
	return protocol.FillRequired(&protobuf.SC_63000{})
}

// dorm is SC_19001, the backyard (DormProxy): level 1 (dorm_data_template 1),
// one floor, two training slots, the owned furniture, no ships or food.
func (game *Game) dorm(ctx context.Context, commander db.Commander) (*protobuf.SC_19001, error) {
	furniture, err := game.DB.ListFurniture(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	list := make([]*protobuf.FURNITUREINFO, len(furniture))
	for i, row := range furniture {
		list[i] = &protobuf.FURNITUREINFO{Id: proto.Uint32(row.FurnitureID), Count: proto.Uint32(row.Count)}
	}
	loadTime := commander.DormLoadTime
	if loadTime == 0 {
		loadTime = uint32(commander.RegisteredAt)
	}
	return protocol.FillRequired(&protobuf.SC_19001{
		Lv:              proto.Uint32(1),
		FurnitureIdList: list,
		FloorNum:        proto.Uint32(1),
		ExpPos:          proto.Uint32(2),
		LoadTime:        proto.Uint32(loadTime),
	}), nil
}
