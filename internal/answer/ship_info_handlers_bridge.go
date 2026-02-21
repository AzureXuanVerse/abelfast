package answer

import (
	answershipinfo "github.com/ggmolly/belfast/internal/answer/shipinfo"
	"github.com/ggmolly/belfast/internal/connection"
)

func GetShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershipinfo.GetShip(buffer, client)
}

func GetShipCount(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershipinfo.GetShipCount(buffer, client)
}

func GetShipDiscuss(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershipinfo.GetShipDiscuss(buffer, client)
}

func SendPlayerShipCount(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershipinfo.SendPlayerShipCount(buffer, client)
}
