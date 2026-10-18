package game

import (
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
