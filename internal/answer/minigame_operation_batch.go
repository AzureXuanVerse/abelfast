package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func MiniGameOperationBatch(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26105{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26104, err
	}

	if len(req.GetCombine()) == 0 {
		resp := &protobuf.SC_26104{Result: proto.Uint32(miniGameOpResultFailure), AwardList: []*protobuf.DROPINFO{}}
		return connection.SendProtoMessage(26104, client, resp)
	}

	var totalWritten int
	for _, operation := range req.GetCombine() {
		resp := runMiniGameOperation(client, operation)
		n, _, err := connection.SendProtoMessage(26104, client, resp)
		totalWritten += n
		if err != nil {
			return totalWritten, 26104, err
		}
	}
	return totalWritten, 26104, nil
}
