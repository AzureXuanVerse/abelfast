package exercise

import (
	"fmt"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const exerciseRivalCount = 5

func buildExerciseRivalTargetList() []*protobuf.TARGETINFO {
	targets := make([]*protobuf.TARGETINFO, 0, exerciseRivalCount)
	for i := 0; i < exerciseRivalCount; i++ {
		id := uint32(90000000 + i + 1)
		level := uint32(1)
		name := fmt.Sprintf("Rival #%d", i+1)
		score := uint32(0)
		rank := uint32(i + 1)
		targets = append(targets, &protobuf.TARGETINFO{
			Id:    proto.Uint32(id),
			Level: proto.Uint32(level),
			Name:  proto.String(name),
			Score: proto.Uint32(score),
			Rank:  proto.Uint32(rank),
		})
	}
	return targets
}

func buildExerciseSeasonPushUpdate(targetList []*protobuf.TARGETINFO) *protobuf.SC_18005 {
	score, rank := currentExerciseSeasonScoreAndRank()
	return &protobuf.SC_18005{
		Score:      proto.Uint32(score),
		Rank:       proto.Uint32(rank),
		TargetList: targetList,
	}
}

func currentExerciseSeasonScoreAndRank() (uint32, uint32) {
	return 0, 0
}
