package answer

import (
	"github.com/ggmolly/belfast/internal/answer/gameroom"
	"github.com/ggmolly/belfast/internal/connection"
)

func GameRoomWeeklyCoinClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	return gameroom.GameRoomWeeklyCoinClaim(buffer, client)
}

func GameRoomSuccessSettlement(buffer *[]byte, client *connection.Client) (int, int, error) {
	return gameroom.GameRoomSuccessSettlement(buffer, client)
}

func GameRoomFirstEnterCoinClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	return gameroom.GameRoomFirstEnterCoinClaim(buffer, client)
}

func GameRoomExchangeCoin(buffer *[]byte, client *connection.Client) (int, int, error) {
	return gameroom.GameRoomExchangeCoin(buffer, client)
}
