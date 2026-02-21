package answer

import (
	answerrandomflag "github.com/ggmolly/belfast/internal/answer/randomflag"
	"github.com/ggmolly/belfast/internal/connection"
)

func ToggleRandomFlagShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerrandomflag.ToggleRandomFlagShip(buffer, client)
}

func ChangeRandomFlagShips(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerrandomflag.ChangeRandomFlagShips(buffer, client)
}

func ChangeRandomFlagShipMode(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerrandomflag.ChangeRandomFlagShipMode(buffer, client)
}
