package world

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func WorldBossInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	state, err := orm.GetOrCreateCommanderWorldBossState(client.Commander.CommanderID)
	if err != nil {
		return 0, 34502, err
	}

	response := protobuf.SC_34502{
		FightCount:           proto.Uint32(state.FightCount),
		FightCountUpdateTime: proto.Uint32(state.FightCountUpdateTime),
		SelfBoss:             worldBossStateToProto(state.SelfBoss),
		SummonPt:             proto.Uint32(state.SummonPt),
		SummonPtOld:          proto.Uint32(state.SummonPtOld),
		SummonPtDailyAcc:     proto.Uint32(state.SummonPtDailyAcc),
		SummonPtOldDailyAcc:  proto.Uint32(state.SummonPtOldDailyAcc),
		SummonFree:           proto.Uint32(state.SummonFree),
		AutoFightFinishTime:  proto.Uint32(state.AutoFightFinishTime),
		DefaultBossId:        proto.Uint32(state.DefaultBossID),
		AutoFightMaxDamage:   proto.Uint32(state.AutoFightMaxDamage),
		GuildSupport:         proto.Uint32(state.GuildSupport),
		FriendSupport:        proto.Uint32(state.FriendSupport),
		WorldSupport:         proto.Uint32(state.WorldSupport),
		SelfBossLv:           proto.Uint32(state.SelfBossLv),
	}
	return client.SendMessage(34502, &response)
}
