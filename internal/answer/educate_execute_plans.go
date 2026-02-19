package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	educateUnsupportedTypeResult = 1
)

func EducateExecutePlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27002
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27003, err
	}

	response := protobuf.SC_27003{
		Result:      proto.Uint32(0),
		PlanResults: []*protobuf.CHILD_PLAN_RESULT{},
		Events:      []*protobuf.CHILD_PLAN_CELL{},
	}
	if payload.GetType() != 1 {
		response.Result = proto.Uint32(educateUnsupportedTypeResult)
	}

	return client.SendMessage(27003, &response)
}
