package answer

import (
	"github.com/ggmolly/belfast/internal/answer/technology"
	"github.com/ggmolly/belfast/internal/connection"
)

const technologyOK = uint32(0)

func StartTechnologyResearch(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.StartTechnologyResearch(buffer, client)
}

func FinishTechnologyResearch(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.FinishTechnologyResearch(buffer, client)
}

func StopTechnologyResearch(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.StopTechnologyResearch(buffer, client)
}

func RefreshTechnologyProjects(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.RefreshTechnologyProjects(buffer, client)
}

func ChangeRefreshTechnologyTendency(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.ChangeRefreshTechnologyTendency(buffer, client)
}

func SelectTechnologyCatchupTarget(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.SelectTechnologyCatchupTarget(buffer, client)
}

func JoinTechnologyQueue(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.JoinTechnologyQueue(buffer, client)
}

func FinishQueueTechnology(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.FinishQueueTechnology(buffer, client)
}

func TechnologyRefreshList(buffer *[]byte, client *connection.Client) (int, int, error) {
	return technology.TechnologyRefreshList(buffer, client)
}

func TechnologyResearchDebugSummary(commanderID uint32) string {
	return technology.TechnologyResearchDebugSummary(commanderID)
}
