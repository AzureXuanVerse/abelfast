package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandBuyDressColor(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21628
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21629, err
	}

	response := &protobuf.SC_21629{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/BuyDressColor"); err != nil {
		return client.SendMessage(21629, response)
	}

	if payload.GetColorId() == 0 {
		return client.SendMessage(21629, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		cfg, found, err := loadIslandDressColorConfig(payload.GetColorId())
		if err != nil || !found || cfg.BelongToDress == 0 {
			return nil
		}

		state, err := orm.GetIslandCommanderDressState(client.Commander.CommanderID, cfg.BelongToDress)
		if err != nil {
			state = &orm.IslandCommanderDressState{CommanderID: client.Commander.CommanderID, DressID: cfg.BelongToDress, ColorList: []uint32{}}
		}
		for i := range state.ColorList {
			if state.ColorList[i] == payload.GetColorId() {
				return nil
			}
		}

		for i := range cfg.Cost {
			if len(cfg.Cost[i]) < 3 {
				continue
			}
			if cfg.Cost[i][0] == 41 {
				if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, cfg.Cost[i][1], cfg.Cost[i][2]); err != nil {
					return err
				}
			}
		}

		state.State = 1
		state.Color = payload.GetColorId()
		state.ColorList = append(state.ColorList, payload.GetColorId())
		if err := orm.UpsertIslandCommanderDressState(state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21629, response)
}
