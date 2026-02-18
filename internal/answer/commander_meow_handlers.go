package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	commanderResultOK   = 0
	commanderResultFail = 1
)

func CommanderBuildBoxStart(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25002
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25003, err
	}

	if _, err := orm.EnsureCommanderBoxes(client.Commander.CommanderID); err != nil {
		return 0, 25003, err
	}
	box, err := orm.GetCommanderBox(client.Commander.CommanderID, packet.GetBoxid())
	if err != nil {
		return sendCommanderBuildBoxStartResult(client, commanderResultFail, orm.CommanderBox{BoxID: packet.GetBoxid()})
	}
	if box.PoolID != 0 {
		return sendCommanderBuildBoxStartResult(client, commanderResultFail, *box)
	}

	material, err := orm.GetCommanderCreateMaterialConfig(packet.GetBoxid())
	if err != nil {
		return sendCommanderBuildBoxStartResult(client, commanderResultFail, *box)
	}

	now := uint32(time.Now().Unix())
	updatedBox := orm.CommanderBox{
		CommanderID: client.Commander.CommanderID,
		BoxID:       packet.GetBoxid(),
		PoolID:      packet.GetBoxid(),
		BeginTime:   now,
		FinishTime:  now + 3600,
	}

	ctx := context.Background()
	err = db.DefaultStore.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if err := client.Commander.ConsumeItemTx(ctx, tx, material.UseItem, material.Number1); err != nil {
			return err
		}
		return orm.UpsertCommanderBoxTx(ctx, tx, updatedBox)
	})
	if err != nil {
		return sendCommanderBuildBoxStartResult(client, commanderResultFail, *box)
	}
	return sendCommanderBuildBoxStartResult(client, commanderResultOK, updatedBox)
}

func sendCommanderBuildBoxStartResult(client *connection.Client, result uint32, box orm.CommanderBox) (int, int, error) {
	response := protobuf.SC_25003{
		Result: proto.Uint32(result),
		Box:    orm.ToProtoCommanderBox(box),
	}
	return client.SendMessage(25003, &response)
}

func CommanderClaimBox(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25004
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25005, err
	}
	if _, err := orm.EnsureCommanderBoxes(client.Commander.CommanderID); err != nil {
		return 0, 25005, err
	}
	box, err := orm.GetCommanderBox(client.Commander.CommanderID, packet.GetBoxid())
	if err != nil {
		return sendCommanderClaimBoxResult(client, commanderResultFail, nil, 0)
	}
	now := uint32(time.Now().Unix())
	if box.PoolID == 0 || box.FinishTime > now {
		return sendCommanderClaimBoxResult(client, commanderResultFail, nil, box.FinishTime)
	}
	current, err := orm.ListCommanderMeows(client.Commander.CommanderID)
	if err != nil {
		return 0, 25005, err
	}
	if len(current) >= 200 {
		return sendCommanderClaimBoxResult(client, commanderResultFail, nil, box.FinishTime)
	}

	templateID, err := orm.RollCommanderTemplateForPool(box.PoolID)
	if err != nil {
		return sendCommanderClaimBoxResult(client, commanderResultFail, nil, box.FinishTime)
	}

	var meow *orm.CommanderMeow
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		var createErr error
		meow, createErr = orm.CreateCommanderMeowTx(context.Background(), tx, client.Commander.CommanderID, templateID)
		if createErr != nil {
			return createErr
		}
		box.PoolID = 0
		box.BeginTime = 0
		box.FinishTime = 0
		return orm.UpsertCommanderBoxTx(context.Background(), tx, *box)
	})
	if err != nil {
		return sendCommanderClaimBoxResult(client, commanderResultFail, nil, box.FinishTime)
	}
	return sendCommanderClaimBoxResult(client, commanderResultOK, meow, now)
}

func sendCommanderClaimBoxResult(client *connection.Client, result uint32, meow *orm.CommanderMeow, finishTime uint32) (int, int, error) {
	response := protobuf.SC_25005{
		Result:     proto.Uint32(result),
		FinishTime: proto.Uint32(finishTime),
	}
	if meow == nil {
		response.Commander = orm.ToProtoCommanderInfo(orm.CommanderMeow{})
	} else {
		response.Commander = orm.ToProtoCommanderInfo(*meow)
	}
	return client.SendMessage(25005, &response)
}

func CommanderFleetEquip(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25006
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25007, err
	}
	if packet.GetCommanderid() != 0 {
		if _, err := orm.GetCommanderMeow(client.Commander.CommanderID, packet.GetCommanderid()); err != nil {
			return sendCommanderSimpleResult(client, 25007, commanderResultFail)
		}
	}
	if err := orm.UpdateFleetMeowfficerSlot(client.Commander, packet.GetGroupid(), packet.GetPos(), packet.GetCommanderid()); err != nil {
		return sendCommanderSimpleResult(client, 25007, commanderResultFail)
	}
	return sendCommanderSimpleResult(client, 25007, commanderResultOK)
}

func CommanderUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25008
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25009, err
	}
	target, err := orm.GetCommanderMeow(client.Commander.CommanderID, packet.GetTargetid())
	if err != nil {
		return sendCommanderSimpleResult(client, 25009, commanderResultFail)
	}
	if len(packet.GetMaterialid()) == 0 {
		return sendCommanderSimpleResult(client, 25009, commanderResultFail)
	}
	if slices.Contains(packet.GetMaterialid(), packet.GetTargetid()) {
		return sendCommanderSimpleResult(client, 25009, commanderResultFail)
	}

	seen := make(map[uint32]struct{}, len(packet.GetMaterialid()))
	materials := make([]*orm.CommanderMeow, 0, len(packet.GetMaterialid()))
	totalGold := uint32(0)
	totalExp := uint32(0)
	targetTpl, err := orm.GetCommanderDataTemplateConfig(target.TemplateID)
	if err != nil {
		return sendCommanderSimpleResult(client, 25009, commanderResultFail)
	}
	sameRate, _, _ := orm.GetCommanderUpgradeRates()
	for _, materialID := range packet.GetMaterialid() {
		if _, ok := seen[materialID]; ok {
			return sendCommanderSimpleResult(client, 25009, commanderResultFail)
		}
		seen[materialID] = struct{}{}
		if orm.IsCommanderInAnyFleet(client.Commander, materialID) {
			return sendCommanderSimpleResult(client, 25009, commanderResultFail)
		}
		material, err := orm.GetCommanderMeow(client.Commander.CommanderID, materialID)
		if err != nil {
			return sendCommanderSimpleResult(client, 25009, commanderResultFail)
		}
		materialTpl, err := orm.GetCommanderDataTemplateConfig(material.TemplateID)
		if err != nil {
			return sendCommanderSimpleResult(client, 25009, commanderResultFail)
		}
		totalGold += materialTpl.ExpCost
		gain := materialTpl.Exp
		if materialTpl.GroupType == targetTpl.GroupType {
			gain = uint32((uint64(gain) * uint64(sameRate)) / 10000)
		}
		totalExp += gain
		materials = append(materials, material)
	}
	if !client.Commander.HasEnoughGold(totalGold) {
		return sendCommanderSimpleResult(client, 25009, commanderResultFail)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := client.Commander.ConsumeResourceTx(ctx, tx, 1, totalGold); err != nil {
			return err
		}
		if err := orm.UpdateCommanderMeowExpTx(ctx, tx, client.Commander.CommanderID, target.ID, target.Exp+totalExp); err != nil {
			return err
		}
		materialIDs := make([]uint32, len(materials))
		for i, m := range materials {
			materialIDs[i] = m.ID
		}
		if err := orm.DeleteCommanderMeowsTx(ctx, tx, client.Commander.CommanderID, materialIDs); err != nil {
			return err
		}
		fleetChanged := false
		for _, fleet := range client.Commander.Fleets {
			for pos, value := range fleet.MeowfficerList {
				if slices.Contains(materialIDs, uint32(value)) {
					fleet.MeowfficerList[pos] = 0
					fleetChanged = true
				}
			}
		}
		if fleetChanged {
			for _, fleet := range client.Commander.Fleets {
				jsonList, err := json.Marshal(fleet.MeowfficerList)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `
UPDATE fleets
SET meowfficer_list = $2
WHERE id = $1
`, int64(fleet.ID), jsonList); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return sendCommanderSimpleResult(client, 25009, commanderResultFail)
	}
	return sendCommanderSimpleResult(client, 25009, commanderResultOK)
}

func CommanderRefreshBoxes(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25034
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25035, err
	}
	_ = packet
	boxes, err := orm.EnsureCommanderBoxes(client.Commander.CommanderID)
	if err != nil {
		return 0, 25035, err
	}
	out := make([]*protobuf.COMMANDERBOXINFO, len(boxes))
	for i := range boxes {
		out[i] = orm.ToProtoCommanderBox(boxes[i])
	}
	response := protobuf.SC_25035{BoxList: out}
	return client.SendMessage(25035, &response)
}

func CommanderQuicklyFinishBoxes(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25037
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25038, err
	}
	boxes, err := orm.EnsureCommanderBoxes(client.Commander.CommanderID)
	if err != nil {
		return 0, 25038, err
	}
	now := uint32(time.Now().Unix())
	balance := client.Commander.GetItemCount(20010)
	expected := orm.ComputeCommanderQuickFinishCounts(boxes, now, balance)
	if expected.ItemCnt == 0 {
		return sendCommanderSimpleResult(client, 25038, commanderResultFail)
	}
	if packet.GetItemCnt() != expected.ItemCnt || packet.GetFinishCnt() != expected.FinishCnt || packet.GetAffectCnt() != expected.AffectCnt {
		return sendCommanderSimpleResult(client, 25038, commanderResultFail)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := client.Commander.ConsumeItemTx(ctx, tx, 20010, expected.ItemCnt); err != nil {
			return err
		}
		_, applyErr := orm.ApplyCommanderQuickFinishTx(ctx, tx, boxes, now, expected.ItemCnt)
		return applyErr
	})
	if err != nil {
		return sendCommanderSimpleResult(client, 25038, commanderResultFail)
	}
	return sendCommanderSimpleResult(client, 25038, commanderResultOK)
}

func sendCommanderSimpleResult(client *connection.Client, packetID int, result uint32) (int, int, error) {
	switch packetID {
	case 25007:
		response := protobuf.SC_25007{Result: proto.Uint32(result)}
		return client.SendMessage(25007, &response)
	case 25009:
		response := protobuf.SC_25009{Result: proto.Uint32(result)}
		return client.SendMessage(25009, &response)
	case 25038:
		response := protobuf.SC_25038{Result: proto.Uint32(result)}
		return client.SendMessage(25038, &response)
	default:
		return 0, packetID, fmt.Errorf("unsupported packet result %d", packetID)
	}
}

func isNotFoundError(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}
