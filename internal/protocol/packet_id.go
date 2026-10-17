package protocol

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
)

var packetIDs sync.Map // message full name -> packet id

// ClientPacketID returns the packet id of a client message (CS_<id>).
func ClientPacketID(msg proto.Message) (int, error) {
	return packetID(msg, "CS_")
}

// ServerPacketID returns the packet id of a server message (SC_<id>).
func ServerPacketID(msg proto.Message) (int, error) {
	return packetID(msg, "SC_")
}

// The packet id is encoded in the generated message name, optionally followed
// by a region suffix (e.g. SC_27011_KR).
func packetID(msg proto.Message, prefix string) (int, error) {
	descriptor := msg.ProtoReflect().Descriptor()
	name := string(descriptor.Name())
	if !strings.HasPrefix(name, prefix) {
		return 0, fmt.Errorf("message %s is not a %s message", name, prefix)
	}
	if id, ok := packetIDs.Load(descriptor.FullName()); ok {
		return id.(int), nil
	}
	digits, _, _ := strings.Cut(strings.TrimPrefix(name, prefix), "_")
	id, err := strconv.ParseUint(digits, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("message %s has no packet id: %w", name, err)
	}
	packetIDs.Store(descriptor.FullName(), int(id))
	return int(id), nil
}
