package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleSecondaryPassword answers FetchSecondaryPasswordCommand, which keeps
// a loading overlay up until it gets a reply: no secondary password set.
func (game *Game) handleSecondaryPassword(client *connection.Client, req *protobuf.CS_11603) error {
	return client.Send(&protobuf.SC_11604{State: proto.Uint32(0)})
}

// handleChangeChatRoom answers ChangeChatRoomCommand, joining the world chat
// room asked for. There is no chat yet, so any room is accepted.
func (game *Game) handleChangeChatRoom(client *connection.Client, req *protobuf.CS_11401) error {
	return client.Send(&protobuf.SC_11402{Result: proto.Uint32(0), RoomId: proto.Uint32(req.GetRoomId())})
}

// handleClientReport takes CS_12299, sent by the C# side of the client after
// loading; there is no reply.
func (game *Game) handleClientReport(client *connection.Client, req *protobuf.CS_12299) error {
	return nil
}
