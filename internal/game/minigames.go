package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleAuctionGameInit answers the auction minigame state the client asks
// for right after loading (AuctionGameInitCommand): nothing played yet.
func (game *Game) handleAuctionGameInit(client *connection.Client, req *protobuf.CS_23430) error {
	return client.Send(&protobuf.SC_23431{})
}

// handleAppreciationVersion takes the gallery / music version the client
// reports while building the player (Player.Ctor); it expects no reply.
func (game *Game) handleAppreciationVersion(client *connection.Client, req *protobuf.CS_15300) error {
	return nil
}

// handleMiniGameHubs answers MiniGameRequestCommand: no minigame played.
func (game *Game) handleMiniGameHubs(client *connection.Client, req *protobuf.CS_26101) error {
	return client.Send(&protobuf.SC_26102{})
}

// handleEmojiInfo answers EmojiInfoRequestCommand: no extra emojis unlocked.
func (game *Game) handleEmojiInfo(client *connection.Client, req *protobuf.CS_11601) error {
	return client.Send(&protobuf.SC_11602{})
}

// constellation_challenge_month row: stage lists the limit challenges of a
// month
type limitChallengeMonthConfig struct {
	Stage []uint32 `json:"stage"`
}

var limitChallengeMonths map[uint32]limitChallengeMonthConfig

// handleLimitChallenge answers LimitChallengeReqCommand: this month's
// challenges, none cleared or awarded.
func (game *Game) handleLimitChallenge(client *connection.Client, req *protobuf.CS_24020) error {
	reply := &protobuf.SC_24021{Result: result(nil)}
	for _, stage := range limitChallengeMonths[uint32(serverNow().Month())].Stage {
		reply.Awards = append(reply.Awards, &protobuf.KVDATA{Key: proto.Uint32(stage)})
	}
	return client.Send(reply)
}

// handleJuusGroups answers InstagramChatCommand: no chat groups in the
// in-game social feed yet.
func (game *Game) handleJuusGroups(client *connection.Client, req *protobuf.CS_11710) error {
	groups, err := game.juusGroups(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	return client.Send(groups)
}
