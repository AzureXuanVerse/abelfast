package island

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSettingFlag(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21332
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21333, err
	}

	response := &protobuf.SC_21333{Result: proto.Uint32(1)}
	state, err := orm.GetIslandCardState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21333, response)
		}
		state = orm.NewIslandCardState(client.Commander.CommanderID)
	}

	for _, flag := range payload.GetFlagList() {
		if flag == nil {
			return client.SendMessage(21333, response)
		}
		if flag.GetFlag() > 1 {
			return client.SendMessage(21333, response)
		}
		switch flag.GetType() {
		case islandCardFlagSocial:
			state.SocialFlag = flag.GetFlag()
		case islandCardFlagLabel:
			state.LabelViewFlag = flag.GetFlag()
		default:
			return client.SendMessage(21333, response)
		}
	}

	if err := orm.UpsertIslandCardState(state); err != nil {
		return client.SendMessage(21333, response)
	}
	response.Result = proto.Uint32(0)
	return client.SendMessage(21333, response)
}
