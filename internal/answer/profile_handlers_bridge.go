package answer

import (
	answerprofile "github.com/ggmolly/belfast/internal/answer/profile"
	"github.com/ggmolly/belfast/internal/connection"
)

func GetPlayerSummaryInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerprofile.GetPlayerSummaryInfo(buffer, client)
}

func GetCommanderHome(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerprofile.GetCommanderHome(buffer, client)
}
