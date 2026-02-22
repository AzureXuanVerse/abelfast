package answer

import (
	answercommandermisc "github.com/ggmolly/belfast/internal/answer/commandermisc"
	"github.com/ggmolly/belfast/internal/connection"
)

func CommanderCommissionsFleet(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderCommissionsFleet(buffer, client)
}

func CommanderDock(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderDock(buffer, client)
}

func CommanderGuildTechnologies(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderGuildTechnologies(buffer, client)
}

func CommanderManualGetPtAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderManualGetPtAward(buffer, client)
}

func CommanderManualGetTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderManualGetTask(buffer, client)
}

func CommanderManualInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderManualInfo(buffer, client)
}

func CommanderMissions(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderMissions(buffer, client)
}

func CommanderOwnedSkins(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answercommandermisc.CommanderOwnedSkins(buffer, client)
}
