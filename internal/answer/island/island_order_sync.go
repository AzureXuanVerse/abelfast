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

func IslandOrderSync(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21024
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21025, err
	}

	response := &protobuf.SC_21025{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}, OrderSys: nil}
	if payload.GetType() != 0 && payload.GetType() != 1 {
		return client.SendMessage(21025, response)
	}
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		claims, err := orm.ListIslandOrderFavorClaimsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		slots, err := orm.ListIslandOrderSlotsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		actGroups, err := orm.ListIslandOrderActGroupsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		shipSlots, err := orm.ListIslandShipOrderSlotsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		appoints, err := orm.ListIslandShipOrderAppointsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.OrderSys = &protobuf.PB_ISLAND_ORDER_SYSTEM{
			Favor:        proto.Uint32(state.Favor),
			GetFavor_:    claims,
			DailySelect:  proto.Uint32(state.DailySelect),
			DailySlotNum: proto.Uint32(state.DailySlotNum),
			TimeSlotNum:  proto.Uint32(state.TimeSlotNum),
			SlotList:     slots,
			ShipSlotList: shipSlots,
			SpeedList:    []*protobuf.PB_SPEED_USE{},
			ShipRefresh:  proto.Uint32(state.ShipRefresh),
			AppointList:  appoints,
			ActGroup:     actGroups,
		}
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
		response.OrderSys = nil
	}

	return client.SendMessage(21025, response)
}
