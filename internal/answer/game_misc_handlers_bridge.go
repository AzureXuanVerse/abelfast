package answer

import (
	answergamemisc "github.com/ggmolly/belfast/internal/answer/gamemisc"
	"github.com/ggmolly/belfast/internal/connection"
)

func GameTracking(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answergamemisc.GameTracking(buffer, client)
}

func GameNotices(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answergamemisc.GameNotices(buffer, client)
}
