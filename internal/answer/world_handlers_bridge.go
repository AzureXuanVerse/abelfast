package answer

import (
	answerworld "github.com/ggmolly/belfast/internal/answer/world"
	"github.com/ggmolly/belfast/internal/connection"
)

func WorldBaseInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerworld.WorldBaseInfo(buffer, client)
}

func WorldBossInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerworld.WorldBossInfo(buffer, client)
}

func WorldCheckInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerworld.WorldCheckInfo(buffer, client)
}
