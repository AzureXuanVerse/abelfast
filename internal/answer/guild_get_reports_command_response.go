package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetReportsCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61017
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61018, err
	}
	if client.Commander == nil {
		return client.SendMessage(61018, &protobuf.SC_61018{Reports: []*protobuf.REPORT{}})
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61018, &protobuf.SC_61018{Reports: []*protobuf.REPORT{}})
		}
		return 0, 61018, err
	}
	reports, err := orm.ListGuildReportsSince(guild.ID, payload.GetIndex())
	if err != nil {
		return 0, 61018, err
	}
	protoReports := make([]*protobuf.REPORT, 0, len(reports))
	for _, report := range reports {
		nodes := make([]*protobuf.REPORT_NODE, 0, len(report.Nodes))
		for _, node := range report.Nodes {
			nodes = append(nodes, &protobuf.REPORT_NODE{Id: proto.Uint32(node.NodeID), Status: proto.Uint32(node.Status)})
		}
		protoReports = append(protoReports, &protobuf.REPORT{
			Id:        proto.Uint32(report.ID),
			EventId:   proto.Uint32(report.EventID),
			EventType: proto.Uint32(report.EventType),
			Score:     proto.Uint32(report.Score),
			Nodes:     nodes,
			Status:    proto.Uint32(report.Status),
		})
	}
	return client.SendMessage(61018, &protobuf.SC_61018{Reports: protoReports})
}
