// Package game answers the game server packets sent once a commander is
// logged in, starting with the player data load (CS_11001).
package game

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// mondayReference is the "monday_0oclock_timestamp" the official EN servers
// send (SC_11000, SC_11002): Monday 2020-11-23 00:00 in UTC-7. The client
// derives its weekly resets from it.
const mondayReference = 1606114800

type Game struct {
	DB *db.Store
}

// StartCommander creates what a new commander starts with (CS_10024), in
// dependency order: ships first, the secretary and fleets point at them.
func StartCommander(ctx context.Context, queries *db.Queries, commander db.Commander) error {
	for _, grant := range []func(context.Context, *db.Queries, db.Commander) error{
		grantShipsStarter,
		grantPlayerStarter,
		grantFleetsStarter,
	} {
		if err := grant(ctx, queries, commander); err != nil {
			return err
		}
	}
	return nil
}

// Register installs the game server packet handlers.
func (game *Game) Register() {
	game.registerPlayer()
	game.registerShips()
	game.registerFleets()
	// tasks area, until its registerTasks lands
	packets.Handle(game.handleGuideIndex)
	packets.Handle(game.handleStoryRead)
	packets.Handle(game.handleTracking)
}

// loginBurstOrder is the order the official server pushes the login burst
// in (CS_11001 up to SC_11002), with the area each push belongs to.
var loginBurstOrder = []int{
	11000, // player
	11003, // player
	11015, // player
	11752, // player
	21536, // player
	22001, // player
	26120, // player
	25001, // player
	17001, // collection
	22300, // player
	12024, // ships (build)
	12001, // ships
	12101, // ships (fleets)
	12201, // player
	63000, // player
	63100, // player
	64000, // player
	13001, // chapters
	13000, // chapters
	13002, // chapters
	13201, // player
	16200, // player
	33114, // player
	14001, // equipment
	14101, // equipment
	15001, // ships (bag)
	20001, // tasks
	20101, // tasks
	20201, // tasks
	19001, // player
	12031, // ships
	28000, // player
	30001, // player
	30101, // player
	50000, // player
	11200, // player
	11210, // player
	11213, // player
	11300, // player
	11002, // player, starts the proxies: always last
}

// handleLoadPlayerData answers CS_11001 (LoadPlayerDataCommand): everything
// pushed before SC_11002 lands in the proxies the client just registered,
// SC_11002 then starts them. Every area hands its pushes over, they go out
// in loginBurstOrder.
func (game *Game) handleLoadPlayerData(client *connection.Client, req *protobuf.CS_11001) error {
	if client.CommanderID == 0 {
		return fmt.Errorf("CS_11001 without a commander")
	}
	ctx := context.Background()
	commander, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	burst := make(map[int][]proto.Message)
	add := func(pushes []proto.Message, err error) error {
		if err != nil {
			return err
		}
		for _, push := range pushes {
			id, err := protocol.ServerPacketID(push)
			if err != nil {
				return err
			}
			burst[id] = append(burst[id], push)
		}
		return nil
	}
	if err := add(game.playerLoginPushes(ctx, commander)); err != nil {
		return err
	}
	if err := add(game.shipsLoginPushes(ctx, commander)); err != nil {
		return err
	}
	if err := add(game.fleetsLoginPushes(ctx, commander)); err != nil {
		return err
	}
	// tasks area, until its tasksLoginPushes lands
	if err := add(taskPushes(), nil); err != nil {
		return err
	}
	for _, id := range loginBurstOrder {
		for _, push := range burst[id] {
			if err := client.Send(push); err != nil {
				return err
			}
		}
		delete(burst, id)
	}
	for id := range burst {
		return fmt.Errorf("login push SC_%d has no place in loginBurstOrder", id)
	}
	return nil
}
