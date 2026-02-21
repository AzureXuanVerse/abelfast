package answer

import (
	"time"

	"github.com/ggmolly/belfast/internal/answer/gameroom"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func EventData(buffer *[]byte, client *connection.Client) (int, int, error) {
	roomIDs, err := gameroom.LoadGameRoomTemplateIDs()
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
		Rooms:         make([]*protobuf.GAMEROOM, 0, len(roomIDs)),
	}
	for _, roomID := range roomIDs {
		response.Rooms = append(response.Rooms, &protobuf.GAMEROOM{
			Roomid:   proto.Uint32(roomID),
			MaxScore: proto.Uint32(scoreByRoom[roomID]),
		})
	}
	return client.SendMessage(26120, &response)
}
