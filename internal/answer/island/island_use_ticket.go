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

const (
	islandTicketTypeOrderCD         = uint32(1)
	islandTicketTypeShipOrder       = uint32(2)
	islandTicketTypeManage          = uint32(3)
	islandTicketTypeAppoint         = uint32(4)
	islandTicketTypeShipOrderReload = uint32(5)
)

func IslandUseTicket(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21423
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21424, err
	}

	response := &protobuf.SC_21424{Result: proto.Uint32(0)}
	if err := ensureCommanderLoaded(client, "Island/UseTicket"); err != nil {
		response.Result = proto.Uint32(5)
		return client.SendMessage(21424, response)
	}

	ticketType := payload.GetType()
	if ticketType == islandTicketTypeAppoint || (ticketType != islandTicketTypeOrderCD && ticketType != islandTicketTypeShipOrder && ticketType != islandTicketTypeManage && ticketType != islandTicketTypeShipOrderReload) {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21424, response)
	}
	consumeReqs, ok := speedTicketConsumeFromProto(payload.GetTickets())
	if !ok || len(consumeReqs) == 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21424, response)
	}

	totalSpeed := uint32(0)
	now := nowUnix()
	for i := range consumeReqs {
		if consumeReqs[i].EndTime != 0 && consumeReqs[i].EndTime < now {
			response.Result = proto.Uint32(3)
			return client.SendMessage(21424, response)
		}
		speedSeconds, found, err := loadIslandSpeedupSeconds(consumeReqs[i].SpeedID)
		if err != nil {
			response.Result = proto.Uint32(5)
			return client.SendMessage(21424, response)
		}
		if !found {
			response.Result = proto.Uint32(1)
			return client.SendMessage(21424, response)
		}
		totalSpeed += speedSeconds * consumeReqs[i].Count
	}

	targetID := payload.GetTargetId()
	if ticketType == islandTicketTypeShipOrderReload {
		targetID = 0
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		newEndTime, err := orm.ReduceIslandSpeedupTargetTx(context.Background(), tx, client.Commander.CommanderID, ticketType, targetID, now, totalSpeed)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(4)
				return nil
			}
			response.Result = proto.Uint32(5)
			return err
		}
		if ticketType == islandTicketTypeShipOrder {
			slot, err := orm.GetIslandRuntimeShipOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, targetID)
			if err == nil {
				if slot.EndTime > now {
					slot.EndTime = newEndTime
				}
				if err := orm.UpsertIslandRuntimeShipOrderSlotTx(context.Background(), tx, slot); err != nil {
					response.Result = proto.Uint32(5)
					return err
				}
			}
		}
		if ticketType == islandTicketTypeManage {
			trade, presell, totalSales, err := orm.GetIslandManageTradeForUpdateTx(context.Background(), tx, client.Commander.CommanderID, targetID)
			if err != nil {
				response.Result = proto.Uint32(5)
				return err
			}
			if trade.GetEndTime() > now {
				trade.EndTime = proto.Uint32(newEndTime)
				if err := orm.UpsertIslandManageTradeTx(context.Background(), tx, client.Commander.CommanderID, trade, presell, totalSales); err != nil {
					response.Result = proto.Uint32(5)
					return err
				}
			}
		}
		if err := orm.ConsumeIslandSpeedupTicketsTx(context.Background(), tx, client.Commander.CommanderID, consumeReqs); err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(2)
				return err
			}
			response.Result = proto.Uint32(5)
			return err
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21424, response)
	}
	return client.SendMessage(21424, response)
}
