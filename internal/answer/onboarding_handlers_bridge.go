package answer

import (
	answeronboarding "github.com/ggmolly/belfast/internal/answer/onboarding"
	"github.com/ggmolly/belfast/internal/connection"
)

const (
	createPlayerNameMin = 4
	createPlayerNameMax = 14
)

var starterShipIDs = map[uint32]struct{}{
	101171: {},
	201211: {},
	401231: {},
}

func CreateNewPlayer(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answeronboarding.CreateNewPlayer(buffer, client)
}
