package game

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// resourceGameCoin is GameRoomProxy.coin_res_id, the Arcade's coins.
const resourceGameCoin = 11

// gameCoins is gameset game_coin_initial (the first visit's coins) and
// game_coin_max (the most one can hold).
var gameCoins = sync.OnceValues(func() ([2]uint32, error) {
	gameset, err := gamedata.Gameset()
	if err != nil {
		return [2]uint32{}, err
	}
	initial, ok := gameset["game_coin_initial"]
	if !ok {
		return [2]uint32{}, gamedata.Missing("gameset", "game_coin_initial")
	}
	limit, ok := gameset["game_coin_max"]
	if !ok {
		return [2]uint32{}, gamedata.Missing("gameset", "game_coin_max")
	}
	return [2]uint32{initial, limit}, nil
})

// gameRoomInfo is SC_26120, GameRoomProxy's Arcade state: whether the first
// visit's coins were taken. No game played, ticket earned or coin bought
// yet, as the official capture's new account.
func gameRoomInfo(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_26120, error) {
	firstEnter, err := queries.GetGameRoomFirstEnter(ctx, commanderID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return &protobuf.SC_26120{FirstEnter: proto.Uint32(firstEnter)}, nil
}

// handleGameRoomFirstCoin answers GameRoomFirstCoinCommand
// (controller/command/gameroom/gameroomfirstcoincommand.lua), sent when the
// Arcade opens while SC_26120.first_enter is 0: on result 0 the client
// marks the visit and adds game_coin_initial coins itself, fewer when that
// would go past game_coin_max; the server gives as many, once. No official
// capture has it: the reply is built from the Lua alone.
func (game *Game) handleGameRoomFirstCoin(client *connection.Client, req *protobuf.CS_26128) error {
	coins, err := gameCoins()
	if err != nil {
		return err
	}
	err = game.tx(context.Background(), client, func(tx *Tx) error {
		set, err := tx.SetGameRoomFirstEnter(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if set == 0 {
			return errRefused
		}
		resources, err := tx.ListResources(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		owned := uint32(0)
		for _, row := range resources {
			if row.ResourceID == resourceGameCoin {
				owned = row.Amount
			}
		}
		return tx.AddResource(resourceGameCoin, min(coins[0], coins[1]-min(owned, coins[1])))
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_26129{Result: result(err)})
}
