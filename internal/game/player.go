package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// registerPlayer installs the player area's handlers.
func (game *Game) registerPlayer() {
	packets.Handle(game.handleDormPopEvents)
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
// resources, backyard furniture, the starter ship as secretary (the avatar is the starter ship).
// The capacities, rank, chat room... are column defaults (0001_commanders.sql).
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
	return &protobuf.SC_11003{
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
		Display:         &protobuf.DISPLAYINFO{Icon: proto.Uint32(commander.StarterShip)}, // can't be changed yet
		MailStoreroomLv: proto.Uint32(commander.MailStoreroomLv),
		BattleUi:        proto.Uint32(0), // item_data_battleui "Standard"
		NewGuideIndex:   proto.Uint32(commander.NewGuideIndex),
		StoryList:       stories,
	}, nil
}

// navalAcademy is SC_22001, NavalAcademyProxy's state: oil field, merchant
// and classroom at level 1 (their template tables start there), two skill
// classes (captured), no lessons.
func navalAcademy() *protobuf.SC_22001 {
	return &protobuf.SC_22001{
		OilWellLevel:  proto.Uint32(1),
		GoldWellLevel: proto.Uint32(1),
		ClassLv:       proto.Uint32(1),
		SkillClassNum: proto.Uint32(2),
	}
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
	return &protobuf.SC_19001{
		Lv:              proto.Uint32(1),
		FurnitureIdList: list,
		FloorNum:        proto.Uint32(1),
		ExpPos:          proto.Uint32(2),
		// nothing is ever collected yet: the backyard was last loaded when
		// the commander was created
		LoadTime: proto.Uint32(uint32(commander.RegisteredAt)),
	}, nil
}

// handleDormPopEvents answers BackYardRequestPopEventCommand (CS_19009): the
// affection and coins waiting above the ships resting in the backyard. No
// ship rests there yet.
func (game *Game) handleDormPopEvents(client *connection.Client, req *protobuf.CS_19009) error {
	return client.Send(&protobuf.SC_19010{})
}
