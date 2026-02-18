package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandInviteShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21609
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21610, err
	}

	response := &protobuf.SC_21610{Result: proto.Uint32(1), Ship: buildIslandShipProto(&orm.IslandShip{ShipID: payload.GetShipId(), Level: 1, BreakLv: 1, SkillLv: 1, ExtraAttrs: []orm.IslandShipAttr{}, Buffs: []orm.IslandShipBuff{}, CanFollow: true})}
	if err := ensureCommanderLoaded(client, "Island/InviteShip"); err != nil {
		return client.SendMessage(21610, response)
	}

	shipID := payload.GetShipId()
	if shipID == 0 {
		return client.SendMessage(21610, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		if _, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID); err == nil {
			return nil
		}

		hasInvite, err := orm.HasIslandShipInviteTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil || !hasInvite {
			return nil
		}

		template, found, err := loadIslandCharaTemplate(shipID)
		if err != nil || !found {
			return nil
		}

		ship := &orm.IslandShip{
			CommanderID:  client.Commander.CommanderID,
			ShipID:       shipID,
			Level:        1,
			Exp:          0,
			BreakLv:      1,
			SkillLv:      1,
			Power:        template.Power,
			RecoverTime:  0,
			UpLimitState: 0,
			CurSkinID:    0,
			ExtraAttrs:   []orm.IslandShipAttr{},
			Buffs:        []orm.IslandShipBuff{},
			CanFollow:    true,
		}
		if err := orm.UpsertIslandShipTx(context.Background(), tx, ship); err != nil {
			return err
		}
		if err := orm.DeleteIslandShipInviteTx(context.Background(), tx, client.Commander.CommanderID, shipID); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.Ship = buildIslandShipProto(ship)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21610, response)
}
