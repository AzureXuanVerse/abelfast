package answer

import (
	"errors"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func Dorm3dChatTriggerEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28023
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28024, err
	}
	if client.Commander == nil {
		return 0, 28024, errors.New("missing commander")
	}
	now := uint32(time.Now().Unix())
	events := make([]orm.Dorm3dEventInfo, 0, len(payload.GetEventList()))
	for _, entry := range payload.GetEventList() {
		events = append(events, orm.Dorm3dEventInfo{
			EventType: entry.GetEventType(),
			Value:     entry.GetValue(),
			ShipGroup: entry.GetShipId(),
		})
	}
	result := uint32(0)
	unlocks, err := orm.ApplyDorm3dTriggerEvents(client.Commander.CommanderID, events, now)
	if err != nil {
		result = 1
	}
	if _, _, sendErr := client.SendMessage(28024, &protobuf.SC_28024{Result: proto.Uint32(result)}); sendErr != nil {
		return 0, 28024, sendErr
	}
	if result != 0 || len(unlocks) == 0 {
		return 0, 28024, nil
	}
	actList := make([]*protobuf.ACT_INFO, 0, len(unlocks))
	for _, unlock := range unlocks {
		actList = append(actList, &protobuf.ACT_INFO{
			ShipId: proto.Uint32(unlock.ShipGroup),
			Type:   proto.Uint32(unlock.Type),
			ActId:  proto.Uint32(unlock.ActID),
			Time:   proto.Uint32(unlock.Time),
		})
	}
	return client.SendMessage(28025, &protobuf.SC_28025{List: actList})
}
