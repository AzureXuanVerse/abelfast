package exercise

import "github.com/ggmolly/belfast/internal/protobuf"

func BuildExerciseRivalTargetList() []*protobuf.TARGETINFO {
	return buildExerciseRivalTargetList()
}

func BuildExerciseSeasonPushUpdate(targetList []*protobuf.TARGETINFO) *protobuf.SC_18005 {
	return buildExerciseSeasonPushUpdate(targetList)
}

func CurrentExerciseSeasonScoreAndRank() (uint32, uint32) {
	return currentExerciseSeasonScoreAndRank()
}

const ExerciseRivalCount = exerciseRivalCount
