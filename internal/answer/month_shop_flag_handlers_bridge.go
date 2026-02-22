package answer

import (
	answermonthshopflag "github.com/ggmolly/belfast/internal/answer/monthshopflag"
	"github.com/ggmolly/belfast/internal/connection"
)

func MonthShopFlag(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermonthshopflag.MonthShopFlag(buffer, client)
}
