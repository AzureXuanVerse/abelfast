package answer

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func GameRoomSuccessSettlement(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26126
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26127, err
	}

	response := &protobuf.SC_26127{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if payload.GetTimes() == 0 || payload.GetRoomid() == 0 || client == nil || client.Commander == nil {
		return client.SendMessage(26127, response)
	}
	if err := client.Commander.Load(); err != nil {
		return client.SendMessage(26127, response)
	}

	room, found, err := loadGameRoomTemplate(payload.GetRoomid())
	if err != nil {
		return 0, 26127, err
	}
	if !found || payload.GetTimes() > room.CoinMax {
		return client.SendMessage(26127, response)
	}

	settings, err := loadGameRoomSettings()
	if err != nil {
		return 0, 26127, err
	}

	insufficientCoin := errors.New("insufficient coin")
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.LoadGameRoomStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID, time.Now().UTC())
		if err != nil {
			return err
		}

		if !client.Commander.HasEnoughResource(gameRoomCoinResourceID, payload.GetTimes()) {
			return insufficientCoin
		}
		if err := client.Commander.ConsumeResourceTx(context.Background(), tx, gameRoomCoinResourceID, payload.GetTimes()); err != nil {
			return err
		}

		rewardPerPlay := uint32(math.Floor(float64(room.AddBase) * gameRoomMultiplierForScore(room.AddNum, payload.GetScore())))
		reward := rewardPerPlay * payload.GetTimes()

		ticketResourceID := room.AddType
		if ticketResourceID == 0 {
			ticketResourceID = gameRoomTicketResourceID
		}
		currentTicket, err := loadGameRoomResourceAmountForUpdateTx(context.Background(), tx, client.Commander.CommanderID, ticketResourceID)
		if err != nil {
			return err
		}

		totalRemaining := uint32(0)
		if currentTicket < settings.TicketTotalMax {
			totalRemaining = settings.TicketTotalMax - currentTicket
		}
		monthlyRemaining := uint32(0)
		if state.MonthlyTicket < settings.TicketMonthlyMax {
			monthlyRemaining = settings.TicketMonthlyMax - state.MonthlyTicket
		}
		grant := reward
		if grant > totalRemaining {
			grant = totalRemaining
		}
		if grant > monthlyRemaining {
			grant = monthlyRemaining
		}

		if grant > 0 {
			if err := client.Commander.AddResourceTx(context.Background(), tx, ticketResourceID, grant); err != nil {
				return err
			}
			state.MonthlyTicket += grant
			response.DropList = []*protobuf.DROPINFO{
				{
					Type:   proto.Uint32(consts.DROP_TYPE_RESOURCE),
					Id:     proto.Uint32(ticketResourceID),
					Number: proto.Uint32(grant),
				},
			}
		}

		if err := orm.UpsertGameRoomScoreTx(context.Background(), tx, client.Commander.CommanderID, payload.GetRoomid(), payload.GetScore()); err != nil {
			return err
		}

		return orm.SaveGameRoomStateTx(context.Background(), tx, state)
	})
	if err != nil {
		if errors.Is(err, insufficientCoin) {
			return client.SendMessage(26127, response)
		}
		return 0, 26127, err
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(26127, response)
}
