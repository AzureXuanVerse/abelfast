package answer

import (
	answermedal "github.com/ggmolly/belfast/internal/answer/medal"
	"github.com/ggmolly/belfast/internal/connection"
)

type honorMedalGoodsListEntry = answermedal.HonorMedalGoodsListEntry

const (
	medalShopCurrencyItemID = answermedal.MedalShopCurrencyItemID
)

func GetMedalShop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermedal.GetMedalShop(buffer, client)
}

func MedalShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermedal.MedalShopPurchase(buffer, client)
}
