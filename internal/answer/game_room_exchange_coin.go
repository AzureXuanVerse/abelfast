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

func GameRoomExchangeCoin(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26124
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26125, err
	}

	response := &protobuf.SC_26125{Result: proto.Uint32(1)}
	if payload.GetTimes() == 0 || client == nil || client.Commander == nil {
		return client.SendMessage(26125, response)
	}
	if err := client.Commander.Load(); err != nil {
		return client.SendMessage(26125, response)
	}

	settings, err := loadGameRoomSettings()
	if err != nil {
		return 0, 26125, err
	}

	insufficientGold := errors.New("insufficient gold")
	noCapacity := errors.New("no capacity")
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadGameRoomStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		currentCoin, err := loadGameRoomResourceAmountForUpdateTx(context.Background(), tx, client.Commander.CommanderID, gameRoomCoinResourceID)
		if err != nil {
			return err
		}
		remaining := uint32(0)
		if currentCoin < settings.CoinMax {
			remaining = settings.CoinMax - currentCoin
		}
		grantCount := payload.GetTimes()
		if grantCount > remaining {
			grantCount = remaining
		}
		if grantCount == 0 {
			return noCapacity
		}

		totalGold := uint32(0)
		for i := uint32(1); i <= grantCount; i++ {
			totalGold += gameRoomExchangePriceByCount(settings.CoinGoldTiers, state.PayCoinCount+i)
		}
		if !client.Commander.HasEnoughGold(totalGold) {
			return insufficientGold
		}
		if err := client.Commander.ConsumeResourceTx(context.Background(), tx, 1, totalGold); err != nil {
			if isNotEnoughResourcesError(err) {
				return insufficientGold
			}
			return err
		}
		if err := client.Commander.AddResourceTx(context.Background(), tx, gameRoomCoinResourceID, grantCount); err != nil {
			return err
		}

		state.PayCoinCount += grantCount
		return orm.SaveGameRoomStateTx(context.Background(), tx, state)
	})
	if err != nil {
		if errors.Is(err, insufficientGold) || errors.Is(err, noCapacity) {
			return client.SendMessage(26125, response)
		}
		return 0, 26125, err
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(26125, response)
}
