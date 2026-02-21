package answer

import (
	answerphantomquest "github.com/ggmolly/belfast/internal/answer/phantomquest"
	"github.com/ggmolly/belfast/internal/connection"
)

func GetPhantomQuestProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerphantomquest.GetPhantomQuestProgress(buffer, client)
}

func FinishPhantomQuest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerphantomquest.FinishPhantomQuest(buffer, client)
}
