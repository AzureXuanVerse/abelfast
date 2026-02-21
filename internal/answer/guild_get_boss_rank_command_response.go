package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildGetBossRankCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61029
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61030, err
	}

	response := &protobuf.SC_61030{List: []*protobuf.RANK_INFO_P61{}}
	if client.Commander == nil || payload.GetType() != 0 {
		return client.SendMessage(61030, response)
	}

	state, err := orm.GetGuildOperationStateForCommander(client.Commander.CommanderID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61030, response)
		}
		return 0, 61030, err
	}
	if state.EndTime <= nowUnix() {
		return client.SendMessage(61030, response)
	}

	bossState, err := orm.GetGuildOperationBossState(state.GuildID, state.ChapterID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61030, response)
		}
		return 0, 61030, err
	}

	ranks, err := orm.ListGuildOperationBossRanks(state.GuildID, state.ChapterID, bossState.BossID)
	if err != nil {
		return 0, 61030, err
	}

	response.List = make([]*protobuf.RANK_INFO_P61, 0, len(ranks))
	for _, entry := range ranks {
		response.List = append(response.List, &protobuf.RANK_INFO_P61{
			UserId: proto.Uint32(entry.UserID),
			Damage: proto.Uint32(entry.Damage),
		})
	}
	return client.SendMessage(61030, response)
}
