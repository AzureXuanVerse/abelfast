package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandUseDelegationTicket(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21427
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21428, err
	}

	response := &protobuf.SC_21428{Result: proto.Uint32(0), TimeList: []uint32{}}
	if payload.GetAreaId() == 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21428, response)
	}
	consumeReqs, ok := speedTicketConsumeFromProto(payload.GetTickets())
	if !ok || len(consumeReqs) == 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21428, response)
	}
	if err := ensureCommanderLoaded(client, "Island/UseDelegationTicket"); err != nil {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21428, response)
	}

	totalSpeed := uint32(0)
	now := nowUnix()
	for i := range consumeReqs {
		if consumeReqs[i].EndTime != 0 && consumeReqs[i].EndTime < now {
			response.Result = proto.Uint32(3)
			return client.SendMessage(21428, response)
		}
		seconds, found, err := loadIslandSpeedupSeconds(consumeReqs[i].SpeedID)
		if err != nil {
			response.Result = proto.Uint32(1)
			return client.SendMessage(21428, response)
		}
		if !found {
			response.Result = proto.Uint32(1)
			return client.SendMessage(21428, response)
		}
		totalSpeed += seconds * consumeReqs[i].Count
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandDelegationByAreaForUpdateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetAreaId())
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(4)
				return nil
			}
			response.Result = proto.Uint32(1)
			return err
		}
		if !slot.HasRole {
			response.Result = proto.Uint32(4)
			return nil
		}

		slot.CostTimeList = reduceIslandTimeline(slot.CostTimeList, totalSpeed, now)
		slot.SpeedTime += totalSpeed
		if len(slot.CostTimeList) > 0 {
			slot.RecoverTime = slot.CostTimeList[len(slot.CostTimeList)-1]
		}
		if err := orm.UpsertIslandDelegationTx(context.Background(), tx, slot); err != nil {
			response.Result = proto.Uint32(1)
			return err
		}
		if err := orm.ConsumeIslandSpeedupTicketsTx(context.Background(), tx, client.Commander.CommanderID, consumeReqs); err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(2)
				return err
			}
			response.Result = proto.Uint32(1)
			return err
		}
		response.TimeList = append(response.TimeList, slot.CostTimeList...)
		return nil
	})
	if err != nil {
		return client.SendMessage(21428, response)
	}
	return client.SendMessage(21428, response)
}

func reduceIslandTimeline(values []uint32, reduceBy uint32, now uint32) []uint32 {
	if len(values) == 0 || reduceBy == 0 {
		return values
	}
	out := make([]uint32, len(values))
	for i := range values {
		if values[i] <= now {
			out[i] = values[i]
			continue
		}
		if reduceBy >= values[i]-now {
			out[i] = now
			continue
		}
		out[i] = values[i] - reduceBy
	}
	return out
}
