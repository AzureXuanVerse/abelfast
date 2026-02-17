package answer

import (
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandEnter(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21202
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21203, err
	}

	targetIslandID := request.GetIslandId()
	inviteCode := strings.TrimSpace(request.GetCode())
	if inviteCode != "" {
		resolvedCommanderID, err := orm.GetCommanderIDByIslandInviteCode(inviteCode)
		if err != nil {
			if db.IsNotFound(err) {
				response := buildIslandEnterResponse(9, 0, 0, 0)
				return client.SendMessage(21203, response)
			}
			response := buildIslandEnterResponse(1, 0, 0, 0)
			_, _, sendErr := client.SendMessage(21203, response)
			return 0, 21203, sendErr
		}
		targetIslandID = resolvedCommanderID
	}

	if targetIslandID == 0 {
		response := buildIslandEnterResponse(1, 0, 0, 0)
		return client.SendMessage(21203, response)
	}

	result, pos, cd := globalIslandRuntimeState.enter(
		client.Commander.CommanderID,
		client.Commander.Name,
		targetIslandID,
		uint32(time.Now().UTC().Unix()),
	)
	response := buildIslandEnterResponse(result, targetIslandID, pos, cd)
	return client.SendMessage(21203, response)
}

func buildIslandEnterResponse(result uint32, islandID uint32, pos uint32, cd uint32) *protobuf.SC_21203 {
	return &protobuf.SC_21203{
		Result:     proto.Uint32(result),
		PlayerList: []*protobuf.PB_PLAYER{},
		IslandId:   proto.Uint32(islandID),
		Pos:        proto.Uint32(pos),
		Cd:         proto.Uint32(cd),
	}
}
