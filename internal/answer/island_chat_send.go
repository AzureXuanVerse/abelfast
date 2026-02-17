package answer

import (
	"strings"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandSendChat(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21323
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21324, err
	}

	trimmedContent := strings.TrimSpace(request.GetContent())
	if trimmedContent == "" {
		response := protobuf.SC_21324{Result: proto.Uint32(1), Tip: proto.String("content is empty")}
		return client.SendMessage(21324, &response)
	}
	if !globalIslandRuntimeState.hasMatchingSession(client.Commander.CommanderID, request.GetIslandId()) {
		response := protobuf.SC_21324{Result: proto.Uint32(1), Tip: proto.String("not in island session")}
		return client.SendMessage(21324, &response)
	}

	response := protobuf.SC_21324{Result: proto.Uint32(0), Tip: proto.String("")}
	if _, _, err := client.SendMessage(21324, &response); err != nil {
		return 0, 21324, err
	}

	push := &protobuf.SC_21325{
		IslandId: proto.Uint32(request.GetIslandId()),
		UserId:   proto.Uint32(client.Commander.CommanderID),
		Content:  proto.String(trimmedContent),
	}
	broadcastIslandPacket(client.Server, request.GetIslandId(), 21325, push)
	return 0, 21324, nil
}

func broadcastIslandPacket(server *connection.Server, islandID uint32, packetID int, message proto.Message) {
	if server == nil {
		return
	}
	for _, candidate := range server.ListClients() {
		if candidate == nil || candidate.Commander == nil {
			continue
		}
		if globalIslandRuntimeState.hasMatchingSession(candidate.Commander.CommanderID, islandID) {
			candidate.SendMessage(packetID, message)
		}
	}
}
