package answer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func StartShipBlueprintDevelopment(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63200
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63201, err
	}

	response := protobuf.SC_63201{Result: proto.Uint32(shipyardResultFailed), Time: proto.Uint32(0)}
	blueprintID := payload.GetBlueprintId()
	if blueprintID == 0 {
		return client.SendMessage(63201, &response)
	}
	if err := ensureCommanderLoadedForShipyard(client.Commander); err != nil {
		return 0, 63201, err
	}
	cfg, err := orm.GetShipDataBlueprintConfig(blueprintID)
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(63201, &response)
		}
		return 0, 63201, err
	}

	now := shipyardNowUnix()
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		state, err := orm.GetOrCreateCommanderShipyardStateTx(ctx, tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		if state.ColdTime > now {
			return db.ErrNotFound
		}
		for _, taskID := range cfg.UnlockTaskOpenCondition {
			ok, err := isShipyardTaskSatisfiedTx(ctx, tx, client.Commander.CommanderID, taskID)
			if err != nil {
				return err
			}
			if !ok {
				return db.ErrNotFound
			}
		}
		if len(cfg.UnlockTask) > 0 && len(cfg.UnlockTask[0]) > 0 {
			ok, err := isShipyardTaskSatisfiedTx(ctx, tx, client.Commander.CommanderID, cfg.UnlockTask[0][0])
			if err != nil {
				return err
			}
			if !ok {
				return db.ErrNotFound
			}
		}
		allRows, err := orm.ListCommanderShipyardBlueprints(client.Commander.CommanderID)
		if err != nil {
			return err
		}
		for _, row := range allRows {
			if row.BlueprintID == blueprintID {
				continue
			}
			if row.ShipID == 0 && row.StartTime > 0 {
				return db.ErrNotFound
			}
		}

		entry, err := getOrInitShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, blueprintID)
		if err != nil {
			return err
		}
		if entry.ShipID != 0 || entry.StartTime > 0 {
			return db.ErrNotFound
		}
		if entry.StartDuration > 0 {
			entry.StartTime = now - entry.StartDuration
		} else {
			entry.StartTime = now
		}
		if err := orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry); err != nil {
			return err
		}
		state.ColdTime = now + shipyardCooldownSeconds
		if err := orm.UpsertCommanderShipyardStateTx(ctx, tx, state); err != nil {
			return err
		}
		response.Time = proto.Uint32(entry.StartTime)
		return nil
	})
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(63201, &response)
		}
		return 0, 63201, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63201, &response)
}

func StopShipBlueprint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63206
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63207, err
	}
	response := protobuf.SC_63207{Result: proto.Uint32(shipyardResultFailed)}
	blueprintID := payload.GetBlueprintId()
	if blueprintID == 0 {
		return client.SendMessage(63207, &response)
	}

	now := shipyardNowUnix()
	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		cfg, err := orm.GetShipDataBlueprintConfig(blueprintID)
		if err != nil {
			return err
		}
		entry, err := orm.GetCommanderShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, blueprintID)
		if err != nil {
			return err
		}
		finished, err := isBlueprintDevelopmentFinished(entry, cfg)
		if err != nil {
			return err
		}
		if entry.ShipID != 0 || (entry.StartTime == 0 && !finished) {
			return db.ErrNotFound
		}
		elapsed := entry.StartDuration
		if entry.StartTime > 0 && now > entry.StartTime {
			elapsed = now - entry.StartTime
		}
		entry.StartTime = 0
		entry.StartDuration = elapsed
		return orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry)
	})
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(63207, &response)
		}
		return 0, 63207, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63207, &response)
}

func ResumeShipBlueprint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63208
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63209, err
	}
	response := protobuf.SC_63209{Result: proto.Uint32(shipyardResultFailed)}
	blueprintID := payload.GetBlueprintId()
	if blueprintID == 0 {
		return client.SendMessage(63209, &response)
	}
	now := shipyardNowUnix()
	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		allRows, err := orm.ListCommanderShipyardBlueprints(client.Commander.CommanderID)
		if err != nil {
			return err
		}
		for _, row := range allRows {
			if row.BlueprintID == blueprintID {
				continue
			}
			if row.ShipID == 0 && row.StartTime > 0 {
				return db.ErrNotFound
			}
		}
		entry, err := orm.GetCommanderShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, blueprintID)
		if err != nil {
			return err
		}
		if entry.ShipID != 0 || entry.StartTime > 0 {
			return db.ErrNotFound
		}
		if entry.StartDuration > now {
			entry.StartTime = 0
		} else {
			entry.StartTime = now - entry.StartDuration
		}
		return orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry)
	})
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(63209, &response)
		}
		return 0, 63209, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63209, &response)
}

func FinishShipBlueprint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63202
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63203, err
	}
	response := protobuf.SC_63203{Result: proto.Uint32(shipyardResultFailed)}
	blueprintID := payload.GetBlueprintId()
	if blueprintID == 0 {
		return client.SendMessage(63203, &response)
	}
	if err := ensureCommanderLoadedForShipyard(client.Commander); err != nil {
		return 0, 63203, err
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		cfg, err := orm.GetShipDataBlueprintConfig(blueprintID)
		if err != nil {
			return err
		}
		entry, err := orm.GetCommanderShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, blueprintID)
		if err != nil {
			return err
		}
		readyToFinish, err := isShipyardBlueprintReadyToFinishTx(ctx, tx, client.Commander.CommanderID, entry, cfg)
		if err != nil {
			return err
		}
		if !readyToFinish || entry.ShipID != 0 {
			return db.ErrNotFound
		}
		ship, err := client.Commander.AddShipTx(ctx, tx, cfg.ShipTemplateID())
		if err != nil {
			return err
		}
		entry.ShipID = ship.ID
		entry.StartTime = 0
		entry.StartDuration = 0
		if err := orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry); err != nil {
			return err
		}
		response.Ship = orm.ToProtoOwnedShip(*ship, nil, nil)
		return nil
	})
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(63203, &response)
		}
		return 0, 63203, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63203, &response)
}

func UseTechSpeedupItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63210
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63211, err
	}
	response := protobuf.SC_63211{Result: proto.Uint32(shipyardResultFailed)}
	if payload.GetBlueprintid() == 0 || payload.GetItemid() == 0 || payload.GetTaskId() == 0 || payload.GetNumber() == 0 {
		return client.SendMessage(63211, &response)
	}
	if err := ensureCommanderLoadedForShipyard(client.Commander); err != nil {
		return 0, 63211, err
	}
	cfg, err := orm.GetShipDataBlueprintConfig(payload.GetBlueprintid())
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(63211, &response)
		}
		return 0, 63211, err
	}
	if len(cfg.UnlockTask) < 4 || len(cfg.UnlockTask[0]) == 0 || len(cfg.UnlockTask[3]) == 0 {
		return client.SendMessage(63211, &response)
	}
	if payload.GetTaskId() != cfg.UnlockTask[0][0] && payload.GetTaskId() != cfg.UnlockTask[3][0] {
		return client.SendMessage(63211, &response)
	}
	itemID, itemExp, err := orm.GetTechnologyCatchupItem(cfg.BlueprintVersion)
	if err != nil || payload.GetItemid() != itemID {
		return client.SendMessage(63211, &response)
	}
	template, err := orm.GetShipyardTaskTemplateConfig(payload.GetTaskId())
	if err != nil {
		return client.SendMessage(63211, &response)
	}
	now := shipyardNowUnix()
	delta := payload.GetNumber() * itemExp
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := client.Commander.ConsumeItemTx(ctx, tx, payload.GetItemid(), payload.GetNumber()); err != nil {
			return err
		}
		return orm.UpsertCommanderTaskProgressTx(ctx, tx, client.Commander.CommanderID, payload.GetTaskId(), orm.TaskProgressAppend, delta, template.TargetNum, now)
	})
	if err != nil {
		if err.Error() == "not enough items" {
			response.Result = proto.Uint32(shipyardResultNoItems)
			return client.SendMessage(63211, &response)
		}
		return 0, 63211, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63211, &response)
}

func ModShipBlueprint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63204
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63205, err
	}
	response := protobuf.SC_63205{Result: proto.Uint32(shipyardResultFailed)}
	shipID := payload.GetShipId()
	count := payload.GetCount()
	if shipID == 0 || count == 0 {
		return client.SendMessage(63205, &response)
	}
	if err := ensureCommanderLoadedForShipyard(client.Commander); err != nil {
		return 0, 63205, err
	}
	ship, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(63205, &response)
	}
	blueprintID := ship.ShipID / 10
	cfg, err := orm.GetShipDataBlueprintConfig(blueprintID)
	if err != nil {
		return client.SendMessage(63205, &response)
	}
	itemExp, err := orm.LoadItemUsageExp(cfg.StrengthenItem)
	if err != nil {
		return client.SendMessage(63205, &response)
	}
	gain := count * itemExp
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		entry, err := orm.GetCommanderShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, blueprintID)
		if err != nil {
			return err
		}
		if entry.ShipID != ship.ID {
			return db.ErrNotFound
		}
		updated, err := applyBlueprintExpGain(entry, cfg, ship.Level, gain)
		if err != nil {
			return err
		}
		if !updated {
			return db.ErrNotFound
		}
		if err := client.Commander.ConsumeItemTx(ctx, tx, cfg.StrengthenItem, count); err != nil {
			return err
		}
		if err := orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry); err != nil {
			return err
		}
		return orm.UpsertOwnedShipStrengthTx(ctx, tx, &orm.OwnedShipStrength{OwnerID: client.Commander.CommanderID, ShipID: ship.ID, StrengthID: shipyardStrengthRecordSlot, Exp: entry.Exp})
	})
	if err != nil {
		if db.IsNotFound(err) || err.Error() == "not enough items" {
			return client.SendMessage(63205, &response)
		}
		return 0, 63205, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63205, &response)
}

func PursueShipBlueprint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63212
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63213, err
	}
	response := protobuf.SC_63213{Result: proto.Uint32(shipyardResultFailed)}
	shipID := payload.GetShipId()
	count := payload.GetCount()
	if shipID == 0 || count == 0 {
		return client.SendMessage(63213, &response)
	}
	if err := ensureCommanderLoadedForShipyard(client.Commander); err != nil {
		return 0, 63213, err
	}
	ship, ok := client.Commander.OwnedShipsMap[shipID]
	if !ok {
		return client.SendMessage(63213, &response)
	}
	blueprintID := ship.ShipID / 10
	cfg, err := orm.GetShipDataBlueprintConfig(blueprintID)
	if err != nil || cfg.IsPursuing != 1 {
		return client.SendMessage(63213, &response)
	}
	itemExp, err := orm.LoadItemUsageExp(cfg.StrengthenItem)
	if err != nil {
		return client.SendMessage(63213, &response)
	}
	useURDiscount := cfg.BlueprintVersion >= 4

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		state, err := orm.GetOrCreateCommanderShipyardStateTx(ctx, tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}
		entry, err := orm.GetCommanderShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, blueprintID)
		if err != nil {
			return err
		}
		if entry.ShipID != ship.ID {
			return db.ErrNotFound
		}
		discounts, err := orm.GetShipyardPursueDiscounts(useURDiscount)
		if err != nil {
			return err
		}
		counter := state.DailyCatchupStrengthen
		if useURDiscount {
			counter = state.DailyCatchupStrengthenUR
		}
		totalCost := uint32(0)
		for i := uint32(0); i < count; i++ {
			index := int(counter + i)
			if index >= len(discounts) {
				index = len(discounts) - 1
			}
			discount := discounts[index]
			if discount > 100 {
				discount = 100
			}
			totalCost += (cfg.Price * (100 - discount)) / 100
		}
		if !client.Commander.HasEnoughGold(totalCost) {
			return errors.New("not enough resources")
		}
		gain := count * itemExp
		updated, err := applyBlueprintExpGain(entry, cfg, ship.Level, gain)
		if err != nil {
			return err
		}
		if !updated {
			return db.ErrNotFound
		}
		if err := client.Commander.ConsumeResourceTx(ctx, tx, 1, totalCost); err != nil {
			return err
		}
		if useURDiscount {
			state.DailyCatchupStrengthenUR += count
		} else {
			state.DailyCatchupStrengthen += count
		}
		if err := orm.UpsertCommanderShipyardStateTx(ctx, tx, state); err != nil {
			return err
		}
		if err := orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry); err != nil {
			return err
		}
		return orm.UpsertOwnedShipStrengthTx(ctx, tx, &orm.OwnedShipStrength{OwnerID: client.Commander.CommanderID, ShipID: ship.ID, StrengthID: shipyardStrengthRecordSlot, Exp: entry.Exp})
	})
	if err != nil {
		if db.IsNotFound(err) || err.Error() == "not enough resources" {
			return client.SendMessage(63213, &response)
		}
		return 0, 63213, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63213, &response)
}

func ItemUnlockShipBlueprint(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63214
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63215, err
	}
	response := protobuf.SC_63215{Result: proto.Uint32(shipyardResultFailed)}
	group := payload.GetGroup()
	itemID := payload.GetItemid()
	if group == 0 || itemID == 0 {
		return client.SendMessage(63215, &response)
	}
	if err := ensureCommanderLoadedForShipyard(client.Commander); err != nil {
		return 0, 63215, err
	}
	cfg, err := orm.GetShipDataBlueprintConfig(group)
	if err != nil {
		return client.SendMessage(63215, &response)
	}
	allowed := false
	for _, candidate := range cfg.GainItemID {
		if candidate == itemID {
			allowed = true
			break
		}
	}
	if !allowed {
		return client.SendMessage(63215, &response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		entry, err := getOrInitShipyardBlueprintTx(ctx, tx, client.Commander.CommanderID, group)
		if err != nil {
			return err
		}
		if entry.ShipID != 0 {
			return db.ErrNotFound
		}
		if err := client.Commander.ConsumeItemTx(ctx, tx, itemID, 1); err != nil {
			return err
		}
		ship, err := client.Commander.AddShipTx(ctx, tx, cfg.ShipTemplateID())
		if err != nil {
			return err
		}
		entry.ShipID = ship.ID
		entry.StartTime = 0
		entry.StartDuration = 0
		if err := orm.UpsertCommanderShipyardBlueprintTx(ctx, tx, entry); err != nil {
			return err
		}
		response.Ship = orm.ToProtoOwnedShip(*ship, nil, nil)
		return nil
	})
	if err != nil {
		if db.IsNotFound(err) || err.Error() == "not enough items" {
			if err.Error() == "not enough items" {
				response.Result = proto.Uint32(shipyardResultNoItems)
			}
			return client.SendMessage(63215, &response)
		}
		return 0, 63215, err
	}
	response.Result = proto.Uint32(shipyardResultOK)
	return client.SendMessage(63215, &response)
}
