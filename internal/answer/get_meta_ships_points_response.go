package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GetMetaShipsPointsResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	response := &protobuf.SC_34002{MetaShipList: []*protobuf.META_SHIP_INFO{}}
	progressList, err := orm.ListCommanderMetaPtProgress(client.Commander.CommanderID)
	if err != nil {
		return connection.SendProtoMessage(34002, client, response)
	}
	response.MetaShipList = make([]*protobuf.META_SHIP_INFO, 0, len(progressList))
	for i := range progressList {
		progress := progressList[i]
		response.MetaShipList = append(response.MetaShipList, &protobuf.META_SHIP_INFO{
			GroupId:   proto.Uint32(progress.GroupID),
			Pt:        proto.Uint32(progress.Pt),
			FetchList: append([]uint32{}, progress.FetchList...),
		})
	}
	return connection.SendProtoMessage(34002, client, response)
}
