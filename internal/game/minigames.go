package game

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleAppreciationVersion takes the gallery / music version the client
// reports while building the player (Player.Ctor); it expects no reply.
func (game *Game) handleAppreciationVersion(client *connection.Client, req *protobuf.CS_15300) error {
	return nil
}

// handleEmojiInfo answers EmojiInfoRequestCommand: no extra emojis unlocked.
func (game *Game) handleEmojiInfo(client *connection.Client, req *protobuf.CS_11601) error {
	return client.Send(&protobuf.SC_11602{})
}
