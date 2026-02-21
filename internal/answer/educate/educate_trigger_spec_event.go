package educate

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateTriggerSpecEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27027
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27028, err
	}

	response := &protobuf.SC_27028{Result: proto.Uint32(educateResultFailed), Drops: []*protobuf.CHILD_DROP{}}
	if client.Commander == nil || payload.GetSpecEventsId() == 0 {
		return client.SendMessage(27028, response)
	}

	events, err := loadEducateSpecialEvents()
	if err != nil {
		return 0, 27028, err
	}
	event, ok := events[payload.GetSpecEventsId()]
	if !ok {
		return client.SendMessage(27028, response)
	}

	finishFlag := educateFlagID(educateFlagSpecialEventBase, payload.GetSpecEventsId())
	alreadyDone, err := hasEducateFlag(client.Commander.CommanderID, finishFlag)
	if err != nil {
		return 0, 27028, err
	}
	if alreadyDone {
		return client.SendMessage(27028, response)
	}

	if drop := toChildDrop(event.DropDisplay); drop != nil {
		if err := applyEducateChildDrop(client, drop); err != nil {
			return 0, 27028, err
		}
		response.Drops = append(response.Drops, drop)
	}
	if err := setEducateFlag(client.Commander.CommanderID, finishFlag); err != nil {
		return 0, 27028, err
	}
	if event.Type == 3 {
		if err := setEducateFlag(client.Commander.CommanderID, educateFlagID(educateFlagDiscountBase, payload.GetSpecEventsId())); err != nil {
			return 0, 27028, err
		}
	}

	response.Result = proto.Uint32(educateResultOK)
	return client.SendMessage(27028, response)
}
