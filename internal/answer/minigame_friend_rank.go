package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func MiniGameFriendRank(buffer *[]byte, client *connection.Client) (int, int, error) {
	req := &protobuf.CS_26111{}
	if err := proto.Unmarshal(*buffer, req); err != nil {
		return 0, 26112, err
	}

	response := &protobuf.SC_26112{Ranks: []*protobuf.FRIENDSCORE{}}
	if req.GetGameid() == 0 {
		return connection.SendProtoMessage(26112, client, response)
	}
	if _, err := orm.GetMiniGameConfig(req.GetGameid()); err != nil {
		return connection.SendProtoMessage(26112, client, response)
	}

	ranks, err := orm.ListCommanderMiniGameScores(req.GetGameid())
	if err != nil {
		return connection.SendProtoMessage(26112, client, response)
	}
	response.Ranks = make([]*protobuf.FRIENDSCORE, 0, len(ranks))
	for _, rank := range ranks {
		response.Ranks = append(response.Ranks, &protobuf.FRIENDSCORE{
			Id:    proto.Uint32(rank.CommanderID),
			Name:  proto.String(rank.Name),
			Score: proto.Uint32(rank.Score),
			Display: &protobuf.DISPLAYINFO{
				Icon:          proto.Uint32(rank.DisplayIcon),
				Skin:          proto.Uint32(rank.DisplaySkin),
				IconFrame:     proto.Uint32(rank.IconFrame),
				ChatFrame:     proto.Uint32(rank.ChatFrame),
				IconTheme:     proto.Uint32(rank.IconTheme),
				MarryFlag:     proto.Uint32(0),
				TransformFlag: proto.Uint32(0),
			},
			TimeData: proto.Uint32(rank.TimeData),
		})
	}
	return connection.SendProtoMessage(26112, client, response)
}
