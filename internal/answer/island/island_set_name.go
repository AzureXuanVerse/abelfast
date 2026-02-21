package island

import (
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSetName(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21004
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21005, err
	}

	response := &protobuf.SC_21005{Ret: proto.Uint32(1)}
	name := normalizeIslandName(payload.GetName())
	if payload.GetType() > 1 || utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 18 {
		return client.SendMessage(21005, response)
	}

	snapshot, err := orm.GetIslandSnapshot(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21005, response)
		}
		snapshot = defaultIslandSnapshot(client.Commander.CommanderID)
	}
	snapshot.Name = name
	if err := orm.UpsertIslandSnapshot(snapshot); err != nil {
		return client.SendMessage(21005, response)
	}

	response.Ret = proto.Uint32(0)
	return client.SendMessage(21005, response)
}
