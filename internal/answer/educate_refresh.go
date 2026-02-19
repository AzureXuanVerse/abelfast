package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27047
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27048, err
	}

	response := protobuf.SC_27048{Result: proto.Uint32(0)}
	return client.SendMessage(27048, &response)
}
