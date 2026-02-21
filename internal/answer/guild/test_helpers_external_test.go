package guild_test

import (
	"context"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"google.golang.org/protobuf/proto"
)

func execAnswerExternalTestSQLT(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}

func queryAnswerExternalTestInt64(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var value int64
	if err := db.DefaultStore.Pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("query row failed: %v", err)
	}
	return value
}

func execAnswerTestSQLT(t *testing.T, query string, args ...any) {
	execAnswerExternalTestSQLT(t, query, args...)
}

func queryAnswerTestInt64(t *testing.T, query string, args ...any) int64 {
	return queryAnswerExternalTestInt64(t, query, args...)
}

func decodeTestPacket(t *testing.T, client *connection.Client, expectedId int, message proto.Message) int {
	t.Helper()
	buffer := client.Buffer.Bytes()
	if len(buffer) == 0 {
		t.Fatalf("expected response buffer")
	}
	packetID := packets.GetPacketId(0, &buffer)
	if packetID != expectedId {
		t.Fatalf("expected packet %d, got %d", expectedId, packetID)
	}
	packetSize := packets.GetPacketSize(0, &buffer) + 2
	if len(buffer) < packetSize {
		t.Fatalf("expected packet size %d, got %d", packetSize, len(buffer))
	}
	payloadStart := packets.HEADER_SIZE
	payloadEnd := payloadStart + (packetSize - packets.HEADER_SIZE)
	if err := proto.Unmarshal(buffer[payloadStart:payloadEnd], message); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	client.Buffer.Reset()
	return packetID
}
