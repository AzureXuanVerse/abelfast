package answer

import (
	answerpropose "github.com/ggmolly/belfast/internal/answer/propose"
	"github.com/ggmolly/belfast/internal/connection"
)

func ProposeShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerpropose.ProposeShip(buffer, client)
}

func RenameProposedShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerpropose.RenameProposedShip(buffer, client)
}

func ConfirmShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerpropose.ConfirmShip(buffer, client)
}
