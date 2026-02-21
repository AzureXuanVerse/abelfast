package answer

import (
	answershipaction "github.com/ggmolly/belfast/internal/answer/shipaction"
	"github.com/ggmolly/belfast/internal/connection"
)

func HandleShipActionList(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershipaction.HandleShipActionList(buffer, client)
}

func HandleShipActionValidate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershipaction.HandleShipActionValidate(buffer, client)
}
