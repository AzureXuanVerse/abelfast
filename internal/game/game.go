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
	DB *db.Queries
}

// Register installs the game server packet handlers.
func (game *Game) Register() {
	packets.Handle(game.handleLoadPlayerData)
}

// handleLoadPlayerData answers CS_11001 (LoadPlayerDataCommand): everything
// pushed before SC_11002 lands in the proxies the client just registered,
// SC_11002 then starts them.
func (game *Game) handleLoadPlayerData(client *connection.Client, req *protobuf.CS_11001) error {
	if client.CommanderID == 0 {
		return fmt.Errorf("CS_11001 without a commander")
	}
	commander, err := game.DB.GetCommander(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	if err := client.Send(playerInfo(commander)); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11002{
		Timestamp:               proto.Uint32(uint32(time.Now().Unix())),
		Monday_0OclockTimestamp: proto.Uint32(mondayReference),
		ShipCount:               proto.Uint32(0),
	})
}

// playerInfo is SC_11003, read by PlayerProxy into a Player (model/vo/player.lua).
func playerInfo(commander db.Commander) *protobuf.SC_11003 {
	return &protobuf.SC_11003{
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
		GuideIndex:                proto.Uint32(0),
		BuyOilCount:               proto.Uint32(0),
		ChatRoomId:                proto.Uint32(1),
		MaxRank:                   proto.Uint32(0),
		RegisterTime:              proto.Uint32(uint32(commander.RegisteredAt)),
		ShipCount:                 proto.Uint32(0),
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
		NewGuideIndex:             proto.Uint32(0),
		LoadingPicOpenFlag:        proto.Uint32(0),
	}
}
