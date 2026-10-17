package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// taskPushes are TaskProxy's initial state: no tasks, and the weekly mission
// progress at 0 (WeekTaskProgress compares against it from the home screen).
func taskPushes() []proto.Message {
	return []proto.Message{
		&protobuf.SC_20001{},
		&protobuf.SC_20101{Info: &protobuf.WEEKLY_INFO{Pt: proto.Uint32(0), RewardLv: proto.Uint32(0)}},
	}
}

// handleMetaPoints answers the META ship points request
// (MetaPTDataRequestCommand): no META ship progress yet.
func (game *Game) handleMetaPoints(client *connection.Client, req *protobuf.CS_34001) error {
	return client.Send(&protobuf.SC_34002{})
}
