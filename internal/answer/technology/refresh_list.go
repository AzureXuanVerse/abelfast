package technology

import (
	"github.com/ggmolly/belfast/internal/connection"
)

func TechnologyRefreshList(buffer *[]byte, client *connection.Client) (int, int, error) {
	response, err := buildTechnologyRefreshSyncResponse(client.Commander.CommanderID)
	if err != nil {
		return 0, 63000, err
	}
	return client.SendMessage(63000, response)
}
