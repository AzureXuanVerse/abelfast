package answer

import (
	answerfleetmisc "github.com/ggmolly/belfast/internal/answer/fleetmisc"
	"github.com/ggmolly/belfast/internal/connection"
)

func FleetEnergyRecoverTime(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfleetmisc.FleetEnergyRecoverTime(buffer, client)
}
