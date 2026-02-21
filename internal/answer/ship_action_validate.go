package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func HandleShipActionValidate(buffer *[]byte, client *connection.Client) (int, int, error) {
	var data protobuf.CS_12020
	if err := proto.Unmarshal(*buffer, &data); err != nil {
		return 0, 12021, err
	}

	response := protobuf.SC_12021{Result: proto.Uint32(1)}
	ship, ok := client.Commander.OwnedShipsMap[data.GetShipId()]
	if ok {
		response.Result = proto.Uint32(0)
	}

	if _, _, err := client.SendMessage(12021, &response); err != nil {
		return 0, 12021, err
	}

	if ok {
		push := protobuf.SC_12019{Intimacy: proto.Uint32(ship.Intimacy)}
		if _, _, err := client.SendMessage(12019, &push); err != nil {
			return 0, 12019, err
		}
	}

	return 0, 12021, nil
}
