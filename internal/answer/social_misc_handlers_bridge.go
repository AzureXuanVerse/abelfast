package answer

import (
	answersocialmisc "github.com/ggmolly/belfast/internal/answer/socialmisc"
	"github.com/ggmolly/belfast/internal/connection"
)

func SendFriendMessage(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answersocialmisc.SendFriendMessage(buffer, client)
}

func ReportPlayer(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answersocialmisc.ReportPlayer(buffer, client)
}

func GetThemeTemplatePlayerInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answersocialmisc.GetThemeTemplatePlayerInfo(buffer, client)
}
