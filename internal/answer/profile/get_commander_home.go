package profile

import (
	"fmt"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

// A get with a type?
func GetCommanderHome(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25026
	err := proto.Unmarshal(*buffer, &packet)
	if err != nil {
		return 0, 25027, err
	}
	logger.LogEvent("Client", "CS_25026", fmt.Sprintf("client asked for type=%d", packet.GetType()), logger.LOG_LEVEL_DEBUG)
	home, slots, err := orm.EnsureCommanderHome(client.Commander.CommanderID)
	if err != nil {
		return 0, 25027, err
	}
	protobufSlots := make([]*protobuf.COMMANDERHOMESLOT, 0, len(slots))
	for _, slot := range slots {
		protobufSlot := &protobuf.COMMANDERHOMESLOT{
			Id:          proto.Uint32(slot.SlotID),
			OpFlag:      proto.Uint32(slot.OpFlag),
			ExpTime:     proto.Uint32(slot.ExpTime),
			CommanderId: proto.Uint32(slot.AssignedCommanderID),
			Style:       proto.Uint32(slot.Style),
			CacheExp:    proto.Uint32(slot.CacheExp),
		}
		if slot.AssignedCommanderID != 0 {
			if assigned, ok := client.Commander.OwnedShipsMap[slot.AssignedCommanderID]; ok {
				protobufSlot.CommanderLevel = proto.Uint32(assigned.Level)
				protobufSlot.CommanderExp = proto.Uint32(assigned.Exp)
			}
		}
		protobufSlots = append(protobufSlots, protobufSlot)
	}

	response := protobuf.SC_25027{
		Level: proto.Uint32(home.Level),
		Exp:   proto.Uint32(home.Exp),
		Slots: protobufSlots,
		Clean: proto.Uint32(home.Clean),
	}
	// Answer with default valid SC_25027
	return client.SendMessage(25027, &response)
}
