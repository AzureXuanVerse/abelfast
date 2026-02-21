package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandShipSkillUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21611
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21612, err
	}

	response := &protobuf.SC_21612{Result: proto.Uint32(1)}
	if err := ensureCommanderLoaded(client, "Island/ShipSkillUpgrade"); err != nil {
		return client.SendMessage(21612, response)
	}

	shipID := payload.GetShipId()
	if shipID == 0 {
		return client.SendMessage(21612, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			return nil
		}
		template, found, err := loadIslandCharaTemplate(ship.ShipID)
		if err != nil || !found || template.SkillID == 0 {
			return nil
		}
		if template.SkillUnlock > 0 && ship.BreakLv < template.SkillUnlock {
			return nil
		}
		skillCfg, found, err := loadIslandCharaSkill(template.SkillID)
		if err != nil || !found {
			return nil
		}
		maxLevel := skillMaxLevel(skillCfg)
		if ship.SkillLv >= maxLevel {
			return nil
		}

		materials, ok := skillMaterialAtLevel(skillCfg, ship.SkillLv)
		if !ok {
			return nil
		}
		for i := range materials {
			if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, materials[i][0], materials[i][1]); err != nil {
				return err
			}
		}
		ship.SkillLv++
		if err := orm.UpsertIslandShipTx(context.Background(), tx, ship); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21612, response)
}
