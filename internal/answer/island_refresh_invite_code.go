package answer

import (
	"fmt"
	"hash/crc32"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandRefreshInviteCode(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21008
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21009, err
	}

	state, err := orm.GetOrCreateCommanderIslandSocialState(client.Commander.CommanderID)
	if err != nil {
		response := protobuf.SC_21009{Result: proto.Uint32(1), InviteCode: proto.String("")}
		return client.SendMessage(21009, &response)
	}

	if request.GetType() != 0 {
		response := protobuf.SC_21009{Result: proto.Uint32(1), InviteCode: proto.String(state.InviteCode)}
		return client.SendMessage(21009, &response)
	}

	today := uint32(time.Now().UTC().Unix() / 86400)
	if state.InviteCodeRefreshDay == today {
		response := protobuf.SC_21009{Result: proto.Uint32(2), InviteCode: proto.String(state.InviteCode)}
		return client.SendMessage(21009, &response)
	}

	inviteCode, err := generateUniqueIslandInviteCode(client.Commander.CommanderID)
	if err != nil {
		response := protobuf.SC_21009{Result: proto.Uint32(1), InviteCode: proto.String(state.InviteCode)}
		return client.SendMessage(21009, &response)
	}

	state.InviteCode = inviteCode
	state.InviteCodeRefreshDay = today
	if err := orm.SaveCommanderIslandSocialState(state); err != nil {
		response := protobuf.SC_21009{Result: proto.Uint32(1), InviteCode: proto.String("")}
		return client.SendMessage(21009, &response)
	}

	response := protobuf.SC_21009{Result: proto.Uint32(0), InviteCode: proto.String(inviteCode)}
	return client.SendMessage(21009, &response)
}

func generateUniqueIslandInviteCode(commanderID uint32) (string, error) {
	seed := uint32(time.Now().UTC().UnixNano())
	for attempt := uint32(0); attempt < 16; attempt++ {
		value := crc32.ChecksumIEEE([]byte(fmt.Sprintf("%d:%d:%d", commanderID, seed, attempt)))
		code := fmt.Sprintf("%08X", value)
		taken, err := orm.IsIslandInviteCodeTaken(code, commanderID)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, nil
		}
	}
	return "", fmt.Errorf("unable to generate unique invite code")
}
