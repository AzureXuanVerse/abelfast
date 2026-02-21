package island

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandNodeResultSuccess = uint32(0)
	islandNodeResultFailure = uint32(1)
)

func IslandRequestNodeList(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26108{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26109, err
	}

	response := &protobuf.SC_26109{Ret: proto.Uint32(islandNodeResultFailure), NodeList: []*protobuf.PB_ISLAND_NODE{}}
	if _, err := loadActivityTemplate(req.GetActId()); err != nil {
		return connection.SendProtoMessage(26109, client, response)
	}

	nodes, err := orm.GetOrCreateIslandNodeState(client.Commander.CommanderID, req.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26109, client, response)
	}
	response.Ret = proto.Uint32(islandNodeResultSuccess)
	response.NodeList = make([]*protobuf.PB_ISLAND_NODE, 0, len(nodes))
	for _, node := range nodes {
		response.NodeList = append(response.NodeList, &protobuf.PB_ISLAND_NODE{
			Id:      proto.Uint32(node.ID),
			EventId: proto.Uint32(node.EventID),
			IsNew:   proto.Uint32(node.IsNew),
		})
	}
	return connection.SendProtoMessage(26109, client, response)
}
