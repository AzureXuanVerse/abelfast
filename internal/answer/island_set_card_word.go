package answer

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSetCardWord(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21330
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21331, err
	}

	response := &protobuf.SC_21331{Result: proto.Uint32(1)}
	word := normalizeCardWord(payload.GetVisitWord())
	if !isCardWordValid(word) {
		return client.SendMessage(21331, response)
	}

	state, err := orm.GetIslandCardState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21331, response)
		}
		state = orm.NewIslandCardState(client.Commander.CommanderID)
	}
	state.VisitWord = word
	if err := orm.UpsertIslandCardState(state); err != nil {
		return client.SendMessage(21331, response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21331, response)
}
