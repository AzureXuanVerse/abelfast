package answer

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
)

func ActivityStoreData(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26160
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26161, err
	}

	response := &protobuf.SC_26161{Result: proto.Uint32(1)}
	if client.Commander == nil || payload.GetActId() == 0 {
		return client.SendMessage(26161, response)
	}

	template, err := loadActivityTemplate(payload.GetActId())
	if err != nil {
		return client.SendMessage(26161, response)
	}
	_, _, active, err := parseActivityTimeWindow(template.Time, time.Now().UTC())
	if err != nil || !active {
		return client.SendMessage(26161, response)
	}

	if err := orm.UpsertActivityStoreState(&orm.ActivityStoreState{
		CommanderID: client.Commander.CommanderID,
		ActivityID:  payload.GetActId(),
		Data1:       payload.GetIntValue(),
		StrData1:    payload.GetStrValue(),
	}); err != nil {
		return 0, 26161, err
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(26161, response)
}
