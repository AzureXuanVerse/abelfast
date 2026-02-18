package answer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	commanderResultOK    = uint32(0)
	commanderResultError = uint32(1)

	commanderCatteryAllOps = uint32(1 | 2 | 4)
)

func CommanderCatteryOperation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25028
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25029, err
	}
	opType := packet.GetType()
	response := protobuf.SC_25029{
		Result: proto.Uint32(commanderResultError),
		Level:  proto.Uint32(0),
		Exp:    proto.Uint32(0),
		OpTime: proto.Uint32(0),
		Awards: []*protobuf.DROPINFO{},
	}
	if orm.CommanderCatteryOpBit(opType) == 0 {
		return client.SendMessage(25029, &response)
	}

	home, slots, err := orm.EnsureCommanderHome(client.Commander.CommanderID)
	if err != nil {
		return 0, 25029, err
	}
	response.Level = proto.Uint32(home.Level)
	response.Exp = proto.Uint32(home.Exp)

	now := uint32(time.Now().UTC().Unix())
	eligible := make([]*orm.CommanderHomeSlot, 0)
	for i := range slots {
		slot := &slots[i]
		if slot.AssignedCommanderID == 0 {
			continue
		}
		if !orm.CommanderHasCatteryOpFlag(slot.OpFlag, opType) {
			continue
		}
		if !commanderSupportsOperation(slot.AssignedCommanderID, opType) {
			continue
		}
		eligible = append(eligible, slot)
	}
	if len(eligible) == 0 {
		return client.SendMessage(25029, &response)
	}

	for _, slot := range eligible {
		slot.OpFlag = orm.CommanderClearCatteryOpFlag(slot.OpFlag, opType)
		slot.ExpTime = now
		slot.CacheExp = 0
		if err := orm.UpdateCommanderHomeSlot(slot); err != nil {
			return 0, 25029, err
		}
		if opType == orm.CommanderCatteryOpFeed {
			if assigned, ok := client.Commander.OwnedShipsMap[slot.AssignedCommanderID]; ok {
				feedExp := orm.GetCommanderHomeFeedExp(home.Level)
				if feedExp > 0 {
					if err := applyOwnedShipCommanderExp(assigned, feedExp); err != nil {
						return 0, 25029, err
					}
				}
			}
		}
	}

	if opType == orm.CommanderCatteryOpClean {
		home.Clean += 1
		if err := orm.UpdateCommanderHome(home); err != nil {
			return 0, 25029, err
		}
		response.Level = proto.Uint32(home.Level)
		response.Exp = proto.Uint32(home.Exp)
	}

	response.Result = proto.Uint32(commanderResultOK)
	response.OpTime = proto.Uint32(now)
	return client.SendMessage(25029, &response)
}

func CommanderCatteryAssign(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25030
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25031, err
	}

	response := protobuf.SC_25031{
		Result:         proto.Uint32(commanderResultError),
		Time:           proto.Uint32(0),
		CommanderLevel: proto.Uint32(0),
		CommanderExp:   proto.Uint32(0),
	}
	home, slots, err := orm.EnsureCommanderHome(client.Commander.CommanderID)
	if err != nil {
		return 0, 25031, err
	}
	_ = home

	slotIndex := packet.GetSlotidx()
	if slotIndex == 0 || int(slotIndex) > len(slots) {
		return client.SendMessage(25031, &response)
	}

	slot := &slots[slotIndex-1]
	now := uint32(time.Now().UTC().Unix())
	if slot.AssignedCommanderID != 0 {
		if assigned, ok := client.Commander.OwnedShipsMap[slot.AssignedCommanderID]; ok {
			response.CommanderLevel = proto.Uint32(assigned.Level)
			response.CommanderExp = proto.Uint32(assigned.Exp)
		}
	}

	if packet.GetCommanderId() == 0 {
		if slot.AssignedCommanderID == 0 {
			return client.SendMessage(25031, &response)
		}
		slot.AssignedCommanderID = 0
		slot.ExpTime = now
		if err := orm.UpdateCommanderHomeSlot(slot); err != nil {
			return 0, 25031, err
		}
		response.Result = proto.Uint32(commanderResultOK)
		response.Time = proto.Uint32(now)
		return client.SendMessage(25031, &response)
	}

	newCommanderID := packet.GetCommanderId()
	if _, ok := client.Commander.OwnedShipsMap[newCommanderID]; !ok {
		return client.SendMessage(25031, &response)
	}
	for i := range slots {
		if slots[i].SlotID == slot.SlotID {
			continue
		}
		if slots[i].AssignedCommanderID == newCommanderID {
			return client.SendMessage(25031, &response)
		}
	}

	slot.AssignedCommanderID = newCommanderID
	slot.ExpTime = now
	mask := commanderOperationMaskForCommander(newCommanderID)
	slot.OpFlag = commanderCatteryAllOps & mask
	if err := orm.UpdateCommanderHomeSlot(slot); err != nil {
		return 0, 25031, err
	}
	response.Result = proto.Uint32(commanderResultOK)
	response.Time = proto.Uint32(now)
	return client.SendMessage(25031, &response)
}

func CommanderCatteryStyle(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25032
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25033, err
	}

	response := protobuf.SC_25033{Result: proto.Uint32(commanderResultError)}
	home, slots, err := orm.EnsureCommanderHome(client.Commander.CommanderID)
	if err != nil {
		return 0, 25033, err
	}

	slotIndex := packet.GetSlotidx()
	if slotIndex == 0 || int(slotIndex) > len(slots) {
		return client.SendMessage(25033, &response)
	}
	styleID := packet.GetStyleidx()
	if !isCommanderHomeStyleAllowed(home.Level, styleID) {
		return client.SendMessage(25033, &response)
	}
	if !isCommanderHomeStyleKnown(styleID) {
		return client.SendMessage(25033, &response)
	}

	slot := &slots[slotIndex-1]
	if slot.Style != styleID {
		slot.Style = styleID
		if err := orm.UpdateCommanderHomeSlot(slot); err != nil {
			return 0, 25033, err
		}
	}

	response.Result = proto.Uint32(commanderResultOK)
	return client.SendMessage(25033, &response)
}

func CommanderCatterySceneState(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25036
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 0, err
	}
	home, _, err := orm.EnsureCommanderHome(client.Commander.CommanderID)
	if err != nil {
		return 0, 0, err
	}
	switch packet.GetIsOpen() {
	case 0:
		home.SceneOpen = true
		if err := orm.ClearCommanderHomeCacheExp(home.CommanderID); err != nil {
			return 0, 0, err
		}
	case 1:
		home.SceneOpen = false
	default:
		logger.LogEvent("Commander/Cattery", "SceneState", fmt.Sprintf("unsupported is_open=%d commander=%d", packet.GetIsOpen(), client.Commander.CommanderID), logger.LOG_LEVEL_DEBUG)
		return 0, 0, nil
	}
	if err := orm.UpdateCommanderHome(home); err != nil {
		return 0, 0, err
	}
	return 0, 0, nil
}

func commanderOperationMaskForCommander(commanderID uint32) uint32 {
	entry, err := orm.GetConfigEntry("ShareCfg/commander_data_template.json", strconv.FormatUint(uint64(commanderID), 10))
	if err != nil {
		return commanderCatteryAllOps
	}
	var payload struct {
		Ability []uint32 `json:"ability"`
	}
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return commanderCatteryAllOps
	}
	if len(payload.Ability) == 0 {
		return commanderCatteryAllOps
	}
	mask := uint32(0)
	for _, opType := range payload.Ability {
		mask |= orm.CommanderCatteryOpBit(opType)
	}
	if mask == 0 {
		return commanderCatteryAllOps
	}
	return mask
}

func commanderSupportsOperation(commanderID uint32, opType uint32) bool {
	return orm.CommanderHasCatteryOpFlag(commanderOperationMaskForCommander(commanderID), opType)
}

func applyOwnedShipCommanderExp(owned *orm.OwnedShip, gain uint32) error {
	if gain == 0 {
		return nil
	}
	if owned.Level >= owned.MaxLevel {
		return nil
	}
	exp := owned.Exp + gain
	for exp >= 100 && owned.Level < owned.MaxLevel {
		exp -= 100
		owned.Level++
	}
	if owned.Level >= owned.MaxLevel {
		exp = 0
	}
	owned.Exp = exp
	return owned.Update()
}

func isCommanderHomeStyleAllowed(level uint32, styleID uint32) bool {
	for _, ownedStyleID := range orm.GetCommanderHomeStyleList(level) {
		if ownedStyleID == styleID {
			return true
		}
	}
	return false
}

func isCommanderHomeStyleKnown(styleID uint32) bool {
	if styleID == 1 {
		return true
	}
	_, err := orm.GetConfigEntry("ShareCfg/commander_home_style.json", strconv.FormatUint(uint64(styleID), 10))
	return err == nil
}

func CommanderBoxesRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25034
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25035, err
	}
	if packet.GetType() != 0 {
		logger.LogEvent("Commander/Boxes", "Refresh", fmt.Sprintf("unsupported type=%d commander=%d", packet.GetType(), client.Commander.CommanderID), logger.LOG_LEVEL_DEBUG)
	}
	boxes, err := orm.EnsureCommanderBoxes(client.Commander.CommanderID)
	if err != nil {
		return 0, 25035, err
	}
	boxList := make([]*protobuf.COMMANDERBOXINFO, len(boxes))
	for i := range boxes {
		boxList[i] = orm.ToProtoCommanderBox(boxes[i])
	}
	return client.SendMessage(25035, &protobuf.SC_25035{BoxList: boxList})
}
