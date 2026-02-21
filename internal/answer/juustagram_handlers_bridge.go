package answer

import (
	answerjuustagram "github.com/ggmolly/belfast/internal/answer/juustagram"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type JuustagramDiscussOption = answerjuustagram.JuustagramDiscussOption

func HandleJuustagramAction(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.HandleJuustagramAction(buffer, client)
}

func JuustagramComment(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.JuustagramComment(buffer, client)
}

func JuustagramData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.JuustagramData(buffer, client)
}

func JuustagramMessageRange(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.JuustagramMessageRange(buffer, client)
}

func JuustagramReadTip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.JuustagramReadTip(buffer, client)
}

func ToggleMangaLike(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.ToggleMangaLike(buffer, client)
}

func MarkMangaRead(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerjuustagram.MarkMangaRead(buffer, client)
}

func BuildJuustagramMessage(commanderID uint32, messageID uint32, now uint32) (*protobuf.INS_MESSAGE, error) {
	return answerjuustagram.BuildJuustagramMessage(commanderID, messageID, now)
}

func ListJuustagramDiscussOptions(messageID uint32) ([]JuustagramDiscussOption, error) {
	return answerjuustagram.ListJuustagramDiscussOptions(messageID)
}
