package educate

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateTriggerEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27017, err
	}

	response := &protobuf.SC_27017{Result: proto.Uint32(educateResultFailed), Drops: []*protobuf.CHILD_DROP{}}
	if client.Commander == nil || payload.GetEventid() == 0 {
		return client.SendMessage(27017, response)
	}

	events, err := loadEducateEvents()
	if err != nil {
		return 0, 27017, err
	}
	if _, ok := events[payload.GetEventid()]; !ok {
		return client.SendMessage(27017, response)
	}

	if err := setEducateFlag(client.Commander.CommanderID, educateFlagID(educateFlagHomeEventBase, payload.GetEventid())); err != nil {
		return 0, 27017, err
	}

	response.Result = proto.Uint32(educateResultOK)
	return client.SendMessage(27017, response)
}
