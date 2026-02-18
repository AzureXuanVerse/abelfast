package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ColoringClear(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_26006{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 26007, err
	}
	response := &protobuf.SC_26007{Result: proto.Uint32(coloringResultFailure)}

	pages, err := loadColoringActivityPages(payload.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26007, client, response)
	}

	pageExists := false
	for i := range pages {
		if pages[i].PageID == payload.GetId() {
			pageExists = true
			break
		}
	}
	if !pageExists {
		return connection.SendProtoMessage(26007, client, response)
	}

	template, err := loadColoringTemplate(payload.GetId())
	if err != nil || template.Blank != 1 {
		return connection.SendProtoMessage(26007, client, response)
	}

	state, err := getOrCreateColoringState(client.Commander.CommanderID, payload.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26007, client, response)
	}
	coloringClearPage(state, payload.GetId())
	if err := orm.SaveCommanderColoringState(state); err != nil {
		return connection.SendProtoMessage(26007, client, response)
	}

	response.Result = proto.Uint32(coloringResultSuccess)
	return connection.SendProtoMessage(26007, client, response)
}
