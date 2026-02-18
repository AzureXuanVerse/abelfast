package answer

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EventData(buffer *[]byte, client *connection.Client) (int, int, error) {
	templates, err := loadGameRoomTemplates()
	if err != nil {
		return 0, 26120, err
	}
	state, err := orm.LoadGameRoomState(client.Commander.CommanderID, time.Now().UTC())
	if err != nil {
		return 0, 26120, err
	}
	scores, err := orm.ListGameRoomScores(client.Commander.CommanderID)
	if err != nil {
		return 0, 26120, err
	}
	scoreByRoom := make(map[uint32]uint32, len(scores))
	for _, score := range scores {
		scoreByRoom[score.RoomID] = score.MaxScore
	}

	response := protobuf.SC_26120{
		WeeklyFree:    proto.Uint32(boolToUint32(state.WeeklyClaimed)),
		MonthlyTicket: proto.Uint32(state.MonthlyTicket),
		PayCoinCount:  proto.Uint32(state.PayCoinCount),
		FirstEnter:    proto.Uint32(boolToUint32(state.FirstEnterClaimed)),
		Rooms:         make([]*protobuf.GAMEROOM, 0, len(templates)),
	}
	for _, room := range templates {
		response.Rooms = append(response.Rooms, &protobuf.GAMEROOM{
			Roomid:   proto.Uint32(room.ID),
			MaxScore: proto.Uint32(scoreByRoom[room.ID]),
		})
	}
	return client.SendMessage(26120, &response)
}
