package answer

import (
	answerserverlink "github.com/ggmolly/belfast/internal/answer/serverlink"
	"github.com/ggmolly/belfast/internal/connection"
)

func BuildServerInterconnectionResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerserverlink.BuildServerInterconnectionResponse(buffer, client)
}
