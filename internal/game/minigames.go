package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleAuctionGameInit answers the auction minigame state the client asks
// for right after loading (AuctionGameInitCommand): nothing played yet.
func (game *Game) handleAuctionGameInit(client *connection.Client, req *protobuf.CS_23430) error {
	return client.Send(&protobuf.SC_23431{
		Gold:          proto.Uint32(0),
		BuyNum:        proto.Uint32(0),
		MaxProfit:     proto.Uint32(0),
		AccProfit:     proto.Int32(0),
		AccBuyPrice:   proto.Uint32(0),
		PreBuyState:   proto.Uint32(0),
		PreTimestamp:  proto.Uint32(0),
		MatchTime:     proto.Uint32(0),
		IsForbidden:   proto.Uint32(0),
		GameNum:       proto.Uint32(0),
		InactiveNum:   proto.Uint32(0),
		InactiveState: proto.Uint32(0),
		BackForbidden: proto.Uint32(0),
		AccItemPrice:  proto.Uint32(0),
		GetReliefNum:  proto.Uint32(0),
	})
}

// handleAppreciationVersion acknowledges the gallery / music version the
// client reports while building the player (Player.Ctor); it expects no reply.
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

// handleLimitChallenge answers LimitChallengeReqCommand: no limit challenge
// cleared or awarded this month.
func (game *Game) handleLimitChallenge(client *connection.Client, req *protobuf.CS_24020) error {
	return client.Send(&protobuf.SC_24021{Result: proto.Uint32(0)})
}

// handleJuusGroups answers InstagramChatCommand: no chat groups in the
// in-game social feed yet.
func (game *Game) handleJuusGroups(client *connection.Client, req *protobuf.CS_11710) error {
	return client.Send(&protobuf.SC_11711{})
}
