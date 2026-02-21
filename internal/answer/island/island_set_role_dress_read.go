package island

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandSetRoleDressRead(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21624
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21625, err
	}

	response := &protobuf.SC_21625{Result: proto.Uint32(1)}
	if client.Commander == nil {
		return client.SendMessage(21625, response)
	}

	unique := make(map[uint32]struct{}, len(payload.GetDressId()))
	ids := make([]uint32, 0, len(payload.GetDressId()))
	for _, dressID := range payload.GetDressId() {
		if dressID == 0 {
			continue
		}
		if _, exists := unique[dressID]; exists {
			continue
		}
		unique[dressID] = struct{}{}
		ids = append(ids, dressID)
	}

	if err := orm.MarkRoleIslandDressRead(client.Commander.CommanderID, ids); err != nil {
		return client.SendMessage(21625, response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21625, response)
}
