package friend

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func SearchFriend(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_50001{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 50002, err
	}

	keyword := strings.TrimSpace(request.GetKeyword())
	response := &protobuf.SC_50002{Result: proto.Uint32(1)}

	var (
		profile *orm.CommanderSocialProfile
		err     error
	)

	switch request.GetType() {
	case 0:
		if keyword == "" {
			return client.SendMessage(50002, response)
		}
		commanderID, parseErr := strconv.ParseUint(keyword, 10, 32)
		if parseErr != nil {
			return client.SendMessage(50002, response)
		}
		profile, err = orm.GetCommanderSocialProfileByID(uint32(commanderID))
	case 1:
		if keyword == "" {
			return client.SendMessage(50002, response)
		}
		profile, err = orm.GetCommanderSocialProfileByName(keyword)
	default:
		return client.SendMessage(50002, response)
	}

	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(50002, response)
		}
		return 0, 50002, err
	}

	medalIDs, err := orm.ListCommanderMedalDisplay(profile.CommanderID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return 0, 50002, err
	}
	if errors.Is(err, db.ErrNotFound) {
		medalIDs = []uint32{}
	}

	response.Result = proto.Uint32(0)
	response.Player = buildDetailInfo(*profile, client, medalIDs)
	return client.SendMessage(50002, response)
}
