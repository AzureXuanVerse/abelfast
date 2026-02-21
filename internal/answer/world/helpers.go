package world

import (
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func buildWorldCountInfo(runtime *orm.WorldRuntime) *protobuf.COUNTINFO {
	activateCount := uint32(0)
	if runtime.MapID > 0 {
		activateCount = 1
	}
	return &protobuf.COUNTINFO{
		StepCount:      proto.Uint32(0),
		TreasureCount:  proto.Uint32(0),
		TaskProgress:   proto.Uint32(runtime.Progress),
		ActivateCount:  proto.Uint32(activateCount),
		CollectionList: []uint32{},
	}
}

func worldBossStateToProto(boss *orm.WorldBossBossState) *protobuf.WORLDBOSS_INFO_P34 {
	if boss == nil {
		return &protobuf.WORLDBOSS_INFO_P34{
			Id:         proto.Uint32(0),
			TemplateId: proto.Uint32(0),
			Lv:         proto.Uint32(0),
			Hp:         proto.Uint32(0),
			Owner:      proto.Uint32(0),
			LastTime:   proto.Uint32(0),
			KillTime:   proto.Uint32(0),
			FightCount: proto.Uint32(0),
			RankCount:  proto.Uint32(0),
		}
	}
	return &protobuf.WORLDBOSS_INFO_P34{
		Id:         proto.Uint32(boss.ID),
		TemplateId: proto.Uint32(boss.TemplateID),
		Lv:         proto.Uint32(boss.Lv),
		Hp:         proto.Uint32(boss.Hp),
		Owner:      proto.Uint32(boss.Owner),
		LastTime:   proto.Uint32(boss.LastTime),
		KillTime:   proto.Uint32(boss.KillTime),
		FightCount: proto.Uint32(boss.FightCount),
		RankCount:  proto.Uint32(boss.RankCount),
	}
}
