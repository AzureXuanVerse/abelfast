package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandSetCommanderDressRead(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21621
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21622, err
	}

	response := &protobuf.SC_21622{Result: proto.Uint32(1)}
	if client.Commander == nil {
		return client.SendMessage(21622, response)
	}

	unique := make(map[uint32]struct{}, len(payload.GetDressId()))
	ids := make([]uint32, 0, len(payload.GetDressId()))
	for _, dressID := range payload.GetDressId() {
		if dressID == 0 {
			continue
		}
		if _, ok := unique[dressID]; ok {
			continue
		}
		unique[dressID] = struct{}{}
		ids = append(ids, dressID)
	}

	if err := orm.MarkCommanderIslandDressRead(client.Commander.CommanderID, ids); err != nil {
		return client.SendMessage(21622, response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21622, response)
}
