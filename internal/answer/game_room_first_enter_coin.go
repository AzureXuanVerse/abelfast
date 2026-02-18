package answer

import (
	"context"
	"errors"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func GameRoomFirstEnterCoinClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26128
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26129, err
	}

	response := &protobuf.SC_26129{Result: proto.Uint32(1)}
	if client == nil || client.Commander == nil {
		return client.SendMessage(26129, response)
	}
	if err := client.Commander.Load(); err != nil {
		return client.SendMessage(26129, response)
	}

	settings, err := loadGameRoomSettings()
	if err != nil {
		return 0, 26129, err
	}

	alreadyClaimed := errors.New("already claimed")
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadGameRoomStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}
		if state.FirstEnterClaimed {
			return alreadyClaimed
		}

		currentCoin, err := loadGameRoomResourceAmountForUpdateTx(context.Background(), tx, client.Commander.CommanderID, gameRoomCoinResourceID)
		if err != nil {
			return err
		}
		remaining := uint32(0)
		if currentCoin < settings.CoinMax {
			remaining = settings.CoinMax - currentCoin
		}
		grant := settings.CoinInitial
		if grant > remaining {
			grant = remaining
		}
		if grant > 0 {
			if err := client.Commander.AddResourceTx(context.Background(), tx, gameRoomCoinResourceID, grant); err != nil {
				return err
			}
		}

		state.FirstEnterClaimed = true
		return orm.SaveGameRoomStateTx(context.Background(), tx, state)
	})
	if err != nil {
		if errors.Is(err, alreadyClaimed) {
			return client.SendMessage(26129, response)
		}
		return 0, 26129, err
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(26129, response)
}
