package answer

import (
	answershopstreet "github.com/ggmolly/belfast/internal/answer/shopstreet"
	"github.com/ggmolly/belfast/internal/connection"
)

func GetShopStreet(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answershopstreet.GetShopStreet(buffer, client)
}
