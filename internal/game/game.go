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

type Game struct {
	DB *db.Store
}

// StartCommander creates what a new commander starts with (CS_10024), in
// dependency order: ships first, the secretary and fleets point at them.
// What a new commander starts with is no progress: the task events and
// pushes its Tx records are dropped.
func StartCommander(ctx context.Context, queries *db.Queries, commander db.Commander) error {
	tx := newTx(ctx, queries, commander.ID)
	for _, grant := range []func(*Tx, db.Commander) error{
		grantShipsStarter,
		grantPlayerStarter,
		grantFleetsStarter,
		grantTasksStarter,
	} {
		if err := grant(tx, commander); err != nil {
			return err
		}
	}
	return nil
}

// LoadGameData reads the config tables the game server uses, for what runs
// without Register (the mail command).
func LoadGameData() error {
	return loadGameData()
}

// Register loads the game data and installs the game server packet handlers.
// It panics when the game data can't be loaded.
func (game *Game) Register() {
	if err := loadGameData(); err != nil {
		panic(fmt.Sprintf("game data: %v", err))
	}
	// the player, settings and guild
	packets.Handle(game.handleLoadPlayerData)
	// the backyard
	packets.Handle(game.handleDormPopEvents)
	packets.Handle(game.handleBackyardVisitor)
	packets.Handle(game.handleDormShipExp)
	packets.Handle(game.handleDormAddShip)
	packets.Handle(game.handleDormShipExit)
	packets.Handle(game.handleDormIntimacy)
	packets.Handle(game.handleDormMoney)
	packets.Handle(game.handlePutFurniture)
	packets.Handle(game.handleDormRename)
	packets.Handle(game.handleBuyFurniture)
	packets.Handle(game.handleThemeTemplates)
	packets.Handle(game.handleSaveThemeTemplate)
	packets.Handle(game.handleDeleteThemeTemplate)
	packets.Handle(game.handleGuideIndex)
	packets.Handle(game.handleStoryRead)
	packets.Handle(game.handleCommonFlag)
	packets.Handle(game.handleHarvestResource)
	packets.Handle(game.handleHarvestClassResource)
	packets.Handle(game.handleSecondaryPassword)
	packets.Handle(game.handleChangeChatRoom)
	packets.Handle(game.handleClientReport)
	packets.Handle(game.handleCommanderHome)
	packets.Handle(game.handleWorldBoss)
	packets.Handle(game.handleWorldState)
	packets.Handle(game.handleMetaPTData)
	packets.Handle(game.handleMetaTacticsInfo)
	packets.Handle(game.handleMetaTactics)
	packets.Handle(game.handleMetaTacticsUnlock)
	packets.Handle(game.handleMetaTacticsSwitch)
	packets.Handle(game.handleStartLesson)
	packets.Handle(game.handleEndLesson)
	packets.Handle(game.handleQuickFinishLesson)
	packets.Handle(game.handleGuildInfo)
	packets.Handle(game.handlePublicGuildTech)
	packets.Handle(game.handleGuildUserInfo)
	packets.Handle(game.handleGuildShop)
	// ships, builds and inventory
	packets.Handle(game.handleBuildShip)
	packets.Handle(game.handleBuildQuickFinish)
	packets.Handle(game.handleBuildResults)
	packets.Handle(game.handleCollectBuilds)
	packets.Handle(game.handleConfirmBuilds)
	packets.Handle(game.handleShipLock)
	packets.Handle(game.handleSetShipSkin)
	packets.Handle(game.handleShipEquip)
	packets.Handle(game.handleRetireShips)
	packets.Handle(game.handleLoveLetters)
	packets.Handle(game.handleCollectionAward)
	packets.Handle(game.handleUpgradeShipEquipment)
	packets.Handle(game.handleUpgradeEquipment)
	packets.Handle(game.handleComposeEquipment)
	packets.Handle(game.handleDestroyEquipments)
	packets.HandleLenient(game.handleSellItems)
	packets.Handle(game.handleUseItem)
	packets.Handle(game.handleQuickExchangeBlueprint)
	packets.Handle(game.handleUseShipExpBooks)
	packets.Handle(game.handleExchangeRing)
	packets.Handle(game.handleComposeItem)
	packets.Handle(game.handleTracking)
	packets.Handle(game.handleTrack)
	packets.Handle(game.handleNewTracking)
	packets.Handle(game.handleMainSceneTracking)
	packets.Handle(game.handleUrExchangeTracking)
	// fleets, the campaign and battles
	packets.Handle(game.handleUpdateFleet)
	packets.Handle(game.handleTracking13101)
	packets.Handle(game.handleUpdateCustomFleet)
	packets.Handle(game.handleChapterOp)
	packets.Handle(game.handleChapterBattleResult)
	packets.Handle(game.handleRemasterInfo)
	packets.Handle(game.handleBeginStage)
	packets.Handle(game.handleFinishStage)
	// tasks and activities
	packets.Handle(game.handleSubmitTask)
	packets.Handle(game.handleSubmitTasks)
	packets.Handle(game.handleSubmitWeeklyTask)
	packets.Handle(game.handleSubmitWeeklyTasks)
	packets.Handle(game.handleSubmitWeeklyProgress)
	packets.Handle(game.handleClientTaskEvent)
	packets.Handle(game.handleSubmitActivityTasks)
	packets.Handle(game.handleHandbookGetTasks)
	packets.Handle(game.handleHandbookPtAward)
	packets.Handle(game.handleActivityOperation)
	// Juus
	packets.Handle(game.handleJuusPostOp)
	packets.Handle(game.handleJuusComment)
	packets.Handle(game.handleJuusPosts)
	packets.Handle(game.handleJuusGroups)
	packets.Handle(game.handleJuusChatReply)
	packets.Handle(game.handleJuusTopicsRead)
	packets.Handle(game.handleJuusActivateTopics)
	// minigames, shops, exercises, TB
	packets.Handle(game.handleAuctionGameInit)
	packets.Handle(game.handleAppreciationVersion)
	packets.Handle(game.handleMiniGameHubs)
	packets.Handle(game.handleGameRoomFirstCoin)
	packets.Handle(game.handleEmojiInfo)
	packets.Handle(game.handleLimitChallenge)
	packets.Handle(game.handleChargeList)
	packets.Handle(game.handleShopping)
	packets.Handle(game.handleChangeSecretaries)
	packets.Handle(game.handleRandomFlagship)
	packets.Handle(game.handleChangeRandomFlagshipMode)
	packets.Handle(game.handleChangeRandomFlagships)
	packets.Handle(game.handleChangeName)
	packets.Handle(game.handleChangeManifesto)
	packets.Handle(game.handleMedalShop)
	packets.Handle(game.handleMedalShopPurchase)
	packets.Handle(game.handleRequisition)
	packets.Handle(game.handleMonthShopping)
	packets.Handle(game.handleUnlockCryptolalia)
	packets.Handle(game.handleShopStreet)
	packets.Handle(game.handleMiniGameShop)
	packets.Handle(game.handleSeasonInfo)
	packets.Handle(game.handleMilitaryShop)
	packets.Handle(game.handleSkinPreviewUsage)
	packets.Handle(game.handleEducate)
	packets.Handle(game.handleNewEducate)
	packets.Handle(game.handleEducateEndings)
	// mail
	packets.Handle(game.handleMailList)
	packets.Handle(game.handleArchiveList)
	packets.Handle(game.handleMailOperation)
	packets.Handle(game.handleDeleteArchivedMail)
	packets.Handle(game.handleExtendMailStorage)
	packets.Handle(game.handleMailStorageWithdraw)
	packets.Handle(game.handleMailTitles)
	// ship comments
	packets.Handle(game.handleFetchDiscussion)
	packets.Handle(game.handlePostDiscussion)
	packets.Handle(game.handleReviewDiscussion)
	packets.Handle(game.handleLikeShip)
	packets.Handle(game.handleReportDiscussion)
}

// login is what the login burst is built from, loaded once per CS_11001.
type login struct {
	ctx       context.Context
	queries   *db.Queries
	commander db.Commander
	ships     []db.Ship
	now       time.Time
}

// loginPush builds one push of the login burst.
type loginPush func(game *Game, l *login) (proto.Message, error)

// empty is a login push with nothing to report.
func empty[M any, PM interface {
	*M
	proto.Message
}]() loginPush {
	return func(*Game, *login) (proto.Message, error) {
		return PM(new(M)), nil
	}
}

// loginBurst is what the official server pushes for CS_11001, in its order
// (capture #010 to #049): everything before SC_11002 lands in the proxies the
// client just registered, SC_11002 then starts them.
var loginBurst = []loginPush{
	// PlayerProxy: server clock
	func(game *Game, l *login) (proto.Message, error) {
		return &protobuf.SC_11000{Timestamp: proto.Uint32(uint32(l.now.Unix())), Monday_0OclockTimestamp: proto.Uint32(mondayReference)}, nil
	},
	func(game *Game, l *login) (proto.Message, error) { return game.playerInfo(l.ctx, l.commander, l.ships) },
	// PlayerProxy: no benefit buffs
	empty[protobuf.SC_11015](),
	// RefluxProxy: not a returning player
	empty[protobuf.SC_11752](),
	// SystemTipProxy (island delegation tips): nothing to collect
	empty[protobuf.SC_21536](),
	func(game *Game, l *login) (proto.Message, error) {
		return navalAcademy(l.ctx, l.queries, l.commander, l.now)
	},
	func(game *Game, l *login) (proto.Message, error) {
		return gameRoomInfo(l.ctx, l.queries, l.commander.ID)
	},
	// CommanderProxy: no cat commanders, boxes or presets
	empty[protobuf.SC_25001](),
	func(game *Game, l *login) (proto.Message, error) {
		return collectionState(l.ctx, l.queries, l.commander.ID)
	},
	func(game *Game, l *login) (proto.Message, error) { return game.handbookLoginPush(l.ctx, l.commander) },
	func(game *Game, l *login) (proto.Message, error) {
		return buildDocksState(l.ctx, l.queries, l.commander.ID)
	},
	func(game *Game, l *login) (proto.Message, error) {
		return shipList(l.ctx, l.queries, l.commander.ID, l.ships)
	},
	func(game *Game, l *login) (proto.Message, error) { return fleetList(l.ctx, l.queries, l.commander.ID) },
	func(game *Game, l *login) (proto.Message, error) {
		return skinList(l.ctx, l.queries, l.commander.ID, l.now)
	},
	// TechnologyProxy: nothing started, no catch-up research
	empty[protobuf.SC_63000](),
	// TechnologyProxy blueprints (PR ships): none
	empty[protobuf.SC_63100](),
	// TechnologyNationProxy: no fleet tech
	empty[protobuf.SC_64000](),
	func(game *Game, l *login) (proto.Message, error) {
		return chapterList(l.ctx, l.queries, l.commander.ID, l.ships)
	},
	func(game *Game, l *login) (proto.Message, error) {
		return currentChapter(l.ctx, l.queries, l.commander.ID)
	},
	func(game *Game, l *login) (proto.Message, error) {
		return commissionList(l.ctx, l.queries, l.commander.ID, l.commander.Level)
	},
	// DailyLevelProxy: no daily challenge played
	empty[protobuf.SC_13201](),
	// ShopsProxy: the monthly shops
	func(game *Game, l *login) (proto.Message, error) {
		return monthShopInfo(l.ctx, l.queries, l.commander.ID, l.now)
	},
	// WorldProxy: Operation Siren open, not started (captured)
	func(*Game, *login) (proto.Message, error) {
		return &protobuf.SC_33114{IsWorldOpen: proto.Uint32(1)}, nil
	},
	func(game *Game, l *login) (proto.Message, error) {
		return equipmentDepot(l.ctx, l.queries, l.commander.ID)
	},
	// EquipmentProxy: no equipment skins
	empty[protobuf.SC_14101](),
	func(game *Game, l *login) (proto.Message, error) { return bag(l.ctx, l.queries, l.commander.ID) },
	func(game *Game, l *login) (proto.Message, error) { return taskList(l.ctx, l.queries, l.commander.ID) },
	func(game *Game, l *login) (proto.Message, error) {
		weekly, err := game.weeklyInfo(l.ctx, l.queries, l.commander.ID)
		return &protobuf.SC_20101{Info: weekly}, err
	},
	func(game *Game, l *login) (proto.Message, error) {
		return game.activityTasksList(l.ctx, l.queries, l.commander.ID, l.now)
	},
	func(game *Game, l *login) (proto.Message, error) {
		return game.dorm(l.ctx, l.queries, l.commander, l.ships)
	},
	func(game *Game, l *login) (proto.Message, error) { return energyTick(l.commander, l.now.Unix()), nil },
	// ApartmentProxy / Dorm3dInsProxy: no dorm 3D
	empty[protobuf.SC_28000](),
	func(game *Game, l *login) (proto.Message, error) {
		return mailCounts(l.ctx, l.queries, l.commander.ID)
	},
	// CompensateProxy: no compensation mail
	empty[protobuf.SC_30101](),
	// FriendProxy: no friends or requests
	empty[protobuf.SC_50000](),
	func(game *Game, l *login) (proto.Message, error) { return game.activityList(l.ctx, l.commander) },
	// ActivityPermanentProxy: no permanent (archived) event running
	empty[protobuf.SC_11210](),
	// ActivityRemasterProxy: no remaster ticket timer
	empty[protobuf.SC_11213](),
	// ServerNoticeProxy: the bulletin board
	func(game *Game, l *login) (proto.Message, error) { return boardNotices(l.ctx, l.queries) },
	// PlayerProxy: everything is loaded, starts the proxies
	func(game *Game, l *login) (proto.Message, error) {
		return &protobuf.SC_11002{
			Timestamp:               proto.Uint32(uint32(l.now.Unix())),
			Monday_0OclockTimestamp: proto.Uint32(mondayReference),
			ShipCount:               proto.Uint32(uint32(len(l.ships))),
		}, nil
	},
}

// handleLoadPlayerData answers CS_11001 (LoadPlayerDataCommand) with the
// login burst.
func (game *Game) handleLoadPlayerData(client *connection.Client, req *protobuf.CS_11001) error {
	if client.CommanderID == 0 {
		return fmt.Errorf("CS_11001 without a commander")
	}
	ctx := context.Background()
	commander, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := expireSkins(ctx, game.DB, commander.ID, now); err != nil {
		return err
	}
	if err := game.syncTechPoints(ctx, commander.ID); err != nil {
		return err
	}
	ships, err := game.DB.ListShipsByCommander(ctx, commander.ID)
	if err != nil {
		return err
	}
	l := &login{ctx: ctx, queries: game.DB.Queries, commander: commander, ships: ships, now: now}
	for _, build := range loginBurst {
		push, err := build(game, l)
		if err != nil {
			return err
		}
		if err := client.Send(push); err != nil {
			return err
		}
	}
	return nil
}
