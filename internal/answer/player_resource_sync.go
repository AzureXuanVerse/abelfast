package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func buildResourceSnapshot(resources []orm.OwnedResource) []*protobuf.RESOURCE {
	snapshot := make([]*protobuf.RESOURCE, 0, len(resources))
	for _, resource := range resources {
		snapshot = append(snapshot, &protobuf.RESOURCE{
			Type: proto.Uint32(resource.ResourceID),
			Num:  proto.Uint32(resource.Amount),
		})
	}
	return snapshot
}

func SendPlayerResourceSync(client *connection.Client) (int, int, error) {
	if client == nil || client.Commander == nil {
		return 0, 11004, errors.New("missing commander")
	}
	response := protobuf.SC_11004{ResourceList: buildResourceSnapshot(client.Commander.OwnedResources)}
	return client.SendMessage(11004, &response)
}
