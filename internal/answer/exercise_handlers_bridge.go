package answer

import (
	answerexercise "github.com/ggmolly/belfast/internal/answer/exercise"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const exerciseRivalCount = answerexercise.ExerciseRivalCount

func ExercisePowerRankList(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerexercise.ExercisePowerRankList(buffer, client)
}

func ExerciseReplaceRivals(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerexercise.ExerciseReplaceRivals(buffer, client)
}

func UpdateExerciseFleet(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerexercise.UpdateExerciseFleet(buffer, client)
}

func buildExerciseRivalTargetList() []*protobuf.TARGETINFO {
	return answerexercise.BuildExerciseRivalTargetList()
}

func buildExerciseSeasonPushUpdate(targetList []*protobuf.TARGETINFO) *protobuf.SC_18005 {
	return answerexercise.BuildExerciseSeasonPushUpdate(targetList)
}

func currentExerciseSeasonScoreAndRank() (uint32, uint32) {
	return answerexercise.CurrentExerciseSeasonScoreAndRank()
}
