package packets

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

const missingPacketResultUnsupportedCommand uint32 = 1

type handler func(client *connection.Client, payload []byte) error

var handlers = map[int][]handler{}

// Handle registers fn for the client message it takes, the packet id is taken
// from the message name (CS_<id>). Handlers registered for the same packet run
// in registration order. A payload that doesn't decode is an error, which
// closes the connection.
//
//	packets.Handle(func(client *connection.Client, req *protobuf.CS_10800) error {
//		return client.Send(&protobuf.SC_10801{...})
//	})
func Handle[Req any, PReq interface {
	*Req
	proto.Message
}](fn func(client *connection.Client, req PReq) error) {
	register(fn, false)
}

// HandleLenient is Handle for a request the client may send empty. The
// client's Packer.Pack (net/packer.lua) frames an empty serialized message
// with a size of 6 (5 + 1) but writes no payload, so the frame carries one
// stray byte the client never wrote (0x01 in the capture), which isn't valid
// protobuf (field number 0). SellItemCommand / FragmentSellCommand send
// CS_15008 with an empty item_list after login and the official server
// answers it (SC_15009 result 0). That lone byte is decoded as the empty
// message; anything else that doesn't decode is still an error.
func HandleLenient[Req any, PReq interface {
	*Req
	proto.Message
}](fn func(client *connection.Client, req PReq) error) {
	register(fn, true)
}

func register[Req any, PReq interface {
	*Req
	proto.Message
}](fn func(client *connection.Client, req PReq) error, lenient bool) {
	id, err := protocol.ClientPacketID(PReq(new(Req)))
	if err != nil {
		panic(err)
	}
	logger.LogEvent("Handler", "Added", fmt.Sprintf("CS_%d", id), logger.LOG_LEVEL_DEBUG)
	handlers[id] = append(handlers[id], func(client *connection.Client, payload []byte) error {
		req := PReq(new(Req))
		if lenient && len(payload) == 1 {
			return fn(client, req)
		}
		if err := proto.Unmarshal(payload, req); err != nil {
			return fmt.Errorf("decoding %x: %w", payload, err)
		}
		return fn(client, req)
	})
}

// Dispatch runs the handlers registered for packetID, unknown packets are
// answered with SC_10998 (unsupported command).
func Dispatch(client *connection.Client, packetID int, payload []byte) error {
	fns, ok := handlers[packetID]
	logger.WithFields(
		"Handler",
		logger.FieldValue("remote", fmt.Sprintf("%s:%d", client.IP, client.Port)),
		logger.FieldValue("packet", packetID),
		logger.FieldValue("size", len(payload)),
		logger.FieldValue("has_handler", ok),
	).Debug("received packet")
	if !ok {
		logger.LogEvent("Handler", "Missing", fmt.Sprintf("CS_%d", packetID), logger.LOG_LEVEL_ERROR)
		return client.Send(&protobuf.SC_10998{
			Cmd:    proto.Uint32(uint32(packetID)),
			Result: proto.Uint32(missingPacketResultUnsupportedCommand),
		})
	}
	for _, fn := range fns {
		start := time.Now()
		if err := fn(client, payload); err != nil {
			return fmt.Errorf("CS_%d: %w", packetID, err)
		}
		logger.LogEvent("Metrics", "HandlerMs", fmt.Sprintf("CS_%d -> %s", packetID, time.Since(start)), logger.LOG_LEVEL_DEBUG)
	}
	return nil
}
