package answer

import (
	answersupportship "github.com/ggmolly/belfast/internal/answer/supportship"
	"github.com/ggmolly/belfast/internal/connection"
)

const (
	supportRequisitionItemID = 15001

	supportRequisitionResultOK              = 0
	supportRequisitionResultFailed          = 1
	supportRequisitionResultNotEnoughMedals = 2
	supportRequisitionResultLimitReached    = 30
)

func SupportShipRequisition(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answersupportship.SupportShipRequisition(buffer, client)
}

func RequestPlayerAssistShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answersupportship.RequestPlayerAssistShip(buffer, client)
}
