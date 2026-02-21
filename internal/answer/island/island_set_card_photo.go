package island

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSetCardPhoto(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21328
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21329, err
	}

	response := &protobuf.SC_21329{Result: proto.Uint32(1)}
	if payload.GetType() != islandCardPhotoTypeID {
		return client.SendMessage(21329, response)
	}

	validIDs, err := loadIslandCardDIYIDs()
	if err != nil {
		return client.SendMessage(21329, response)
	}
	if _, ok := validIDs[parsePictureID(payload.GetPicture())]; !ok {
		return client.SendMessage(21329, response)
	}

	state, err := orm.GetIslandCardState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21329, response)
		}
		state = orm.NewIslandCardState(client.Commander.CommanderID)
	}
	state.Picture = payload.GetPicture()
	if err := orm.UpsertIslandCardState(state); err != nil {
		return client.SendMessage(21329, response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21329, response)
}
