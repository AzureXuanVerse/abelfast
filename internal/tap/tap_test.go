package tap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// TestTapRelaysAndCaptures: a client sends a framed CS_10800 through the tap;
// the upstream must receive the bytes verbatim and the capture must contain
// the decoded packet, then a junk HTTP probe must be captured as hex.
func TestTapRelaysAndCaptures(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	upstreamBytes := make(chan []byte, 1)
	go func() {
		conn, err := upstream.Accept()
		if err != nil {
			upstreamBytes <- nil
			return
		}
		data, _ := io.ReadAll(conn)
		upstreamBytes <- data
	}()

	capture := &bytes.Buffer{}
	sink := NewSink(capture)
	tapInstance := &Tap{Listen: "127.0.0.1:0", Upstream: upstream.Addr().String(), Sink: sink, Ready: make(chan struct{})}
	go tapInstance.Run()
	select {
	case <-tapInstance.Ready:
	case <-time.After(2 * time.Second):
		t.Fatal("tap never started listening")
	}

	client, err := net.Dial("tcp", tapInstance.Address())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := proto.Marshal(&protobuf.CS_10800{State: proto.Uint32(1), Platform: proto.String("android")})
	if err != nil {
		t.Fatal(err)
	}
	frame := protocol.AppendFrame(nil, 10800, 0, payload)
	if _, err := client.Write(frame); err != nil {
		t.Fatal(err)
	}
	// give the tap time to relay before the junk kills the session
	time.Sleep(200 * time.Millisecond)
	client.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	client.Close()

	select {
	case got := <-upstreamBytes:
		if !bytes.Equal(got, frame) {
			t.Fatalf("upstream got %v, want verbatim %v", got, frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upstream saw nothing")
	}

	// wait for the session to fully close before reading the capture
	for i := 0; i < 100; i++ {
		if strings.Contains(sink.Lines(), `"event":"close"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	text := sink.Lines()
	if !strings.Contains(text, `"name":"CS_10800"`) || !strings.Contains(text, "android") {
		t.Fatalf("capture missing decoded CS_10800:\n%s", text)
	}
	if !strings.Contains(text, `"event":"junk"`) {
		t.Fatalf("capture missing junk event:\n%s", text)
	}
	// every line must be valid JSON
	scanned := strings.NewReader(text)
	scanner := bufio.NewScanner(scanned)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			t.Fatalf("invalid JSON line: %s", line)
		}
	}
}
