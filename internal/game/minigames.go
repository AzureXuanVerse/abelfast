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
