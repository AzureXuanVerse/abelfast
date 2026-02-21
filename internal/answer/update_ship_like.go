package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func UpdateShipLike(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_17107
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 17108, err
	}
	likeErr := client.Commander.Like(payload.GetShipGroupId())
	response := protobuf.SC_17108{
		Result: proto.Uint32(boolToUint32(likeErr != nil)),
	}
	size, packetID, err := client.SendMessage(17108, &response)
	if err != nil {
		return size, packetID, err
	}
	if likeErr != nil {
		return size, packetID, nil
	}
	if _, err := sendCollectionShipGroupUpdate(client, payload.GetShipGroupId()); err != nil {
		return 0, 17004, err
	}
	return size, packetID, nil
}
