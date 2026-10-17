package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// handleAuctionGameInit answers the auction minigame state the client asks
// for right after loading (AuctionGameInitCommand): nothing played yet.
func (game *Game) handleAuctionGameInit(client *connection.Client, req *protobuf.CS_23430) error {
	return client.Send(protocol.FillRequired(&protobuf.SC_23431{}))
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

// limitChallengeStages are constellation_challenge_month[month].stage, the
// limit challenges of each month (index 0 is January).
var limitChallengeStages = [12][3]uint32{
	{10022, 10023, 10024}, {10025, 10026, 10027}, {10028, 10029, 10030}, {10031, 10032, 10033},
	{10034, 10035, 10036}, {10001, 10002, 10003}, {10004, 10005, 10006}, {10007, 10008, 10009},
	{10010, 10011, 10012}, {10013, 10014, 10015}, {10016, 10017, 10018}, {10019, 10020, 10021},
}

// handleLimitChallenge answers LimitChallengeReqCommand: this month's
// challenges, none cleared or awarded.
func (game *Game) handleLimitChallenge(client *connection.Client, req *protobuf.CS_24020) error {
	reply := &protobuf.SC_24021{Result: proto.Uint32(0)}
	for _, stage := range limitChallengeStages[serverNow().Month()-1] {
		reply.Awards = append(reply.Awards, &protobuf.KVDATA{Key: proto.Uint32(stage), Value: proto.Uint32(0)})
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
