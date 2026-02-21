package educate

import (
	"sort"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateGetEvents(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27014
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27015, err
	}

	response := &protobuf.SC_27015{Result: proto.Uint32(educateResultOK), Events: []uint32{}}
	if client.Commander == nil || payload.GetType() != 0 {
		response.Result = proto.Uint32(educateResultFailed)
		return client.SendMessage(27015, response)
	}

	specialEvents, err := loadEducateSpecialEvents()
	if err != nil {
		return 0, 27015, err
	}
	events := make([]uint32, 0, len(specialEvents))
	for _, row := range specialEvents {
		if row.ID == 0 || row.Show == 0 {
			continue
		}
		consumed, err := hasEducateFlag(client.Commander.CommanderID, educateFlagID(educateFlagHomeEventBase, row.ID))
		if err != nil {
			return 0, 27015, err
		}
		if consumed {
			continue
		}
		events = append(events, row.ID)
	}
	sort.Slice(events, func(i, j int) bool { return events[i] < events[j] })
	response.Events = events
	return client.SendMessage(27015, response)
}
