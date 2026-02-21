package answer

import (
	answeremoji "github.com/ggmolly/belfast/internal/answer/emoji"
	"github.com/ggmolly/belfast/internal/connection"
)

func EmojiInfoRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answeremoji.EmojiInfoRequest(buffer, client)
}
