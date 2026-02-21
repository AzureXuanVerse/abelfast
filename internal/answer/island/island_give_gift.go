package island

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandGiveGift(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21613
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21614, err
	}

	response := &protobuf.SC_21614{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/GiveGift"); err != nil {
		return client.SendMessage(21614, response)
	}

	shipID := payload.GetShipId()
	giftID := payload.GetGiftId()
	if shipID == 0 || giftID == 0 {
		return client.SendMessage(21614, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			return nil
		}
		template, found, err := loadIslandCharaTemplate(ship.ShipID)
		if err != nil || !found {
			return nil
		}
		giftItem, found, err := loadIslandItemTemplate(giftID)
		if err != nil || !found {
			return nil
		}
		if giftItem.Usage != "usage_island_gift" {
			return nil
		}

		favorite := shipHasFavoriteGift(template, giftID)
		effect, ok := parseGiftEffects(giftItem.UsageArg, favorite)
		if !ok {
			return nil
		}

		if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, giftID, 1); err != nil {
			return err
		}

		now := uint32(time.Now().UTC().Unix())
		ship.Power += effect.Energy
		ship.Buffs = upsertShipBuffsWithConflict(ship.Buffs, effect.BuffIDs, now)
		if err := orm.UpsertIslandShipTx(context.Background(), tx, ship); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21614, response)
}
