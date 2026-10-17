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
// in registration order.
//
//	packets.Handle(func(client *connection.Client, req *protobuf.CS_10800) error {
//		return client.Send(&protobuf.SC_10801{...})
//	})
func Handle[Req any, PReq interface {
	*Req
	proto.Message
}](fn func(client *connection.Client, req PReq) error) {
	id, err := protocol.ClientPacketID(PReq(new(Req)))
	if err != nil {
		panic(err)
	}
	logger.LogEvent("Handler", "Added", fmt.Sprintf("CS_%d", id), logger.LOG_LEVEL_DEBUG)
	handlers[id] = append(handlers[id], func(client *connection.Client, payload []byte) error {
		req := PReq(new(Req))
		if err := proto.Unmarshal(payload, req); err != nil {
			return err
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
