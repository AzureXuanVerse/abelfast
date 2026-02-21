package answer

import (
	answermailbasic "github.com/ggmolly/belfast/internal/answer/mailbasic"
	"github.com/ggmolly/belfast/internal/connection"
)

func AskMailBody(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermailbasic.AskMailBody(buffer, client)
}

func DeleteArchivedMail(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermailbasic.DeleteArchivedMail(buffer, client)
}
