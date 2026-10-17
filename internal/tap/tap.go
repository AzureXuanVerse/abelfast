// Package tap is a transparent framing-aware proxy for the game protocol:
// every client connection is relayed verbatim to a fixed upstream while each
// framed packet is decoded into a JSONL capture for later disassembly.
// Non-protocol traffic on the same port (HTTP probes) is relayed too and
// captured raw.
package tap

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protocol"
)

// Sink writes capture lines as JSONL; safe for concurrent use.
type Sink struct {
	mu sync.Mutex
	w  io.Writer
}

func NewSink(w io.Writer) *Sink { return &Sink{w: w} }

func (s *Sink) line(fields map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(fields)
	if err != nil {
		return
	}
	fmt.Fprintf(s.w, "%s\n", data)
}

// Lines returns the sink contents so far (for buffered sinks).
func (s *Sink) Lines() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.w.(interface{ String() string }); ok {
		return b.String()
	}
	return ""
}

// Tap relays connections from Listen to Upstream, capturing every frame.
type Tap struct {
	Listen   string
	Upstream string
	Sink     *Sink

	// Ready is closed once the listener is bound.
	Ready chan struct{}
	addr  atomic.Value // string
}

// Address returns the bound listener address, or "" before Ready.
func (t *Tap) Address() string {
	v, _ := t.addr.Load().(string)
	return v
}

func (t *Tap) Run() error {
	listener, err := net.Listen("tcp", t.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	if t.Ready == nil {
		t.Ready = make(chan struct{})
	}
	t.addr.Store(listener.Addr().String())
	close(t.Ready)
	logger.LogEvent("Tap", "Run", fmt.Sprintf("listening on %s -> %s", t.Listen, t.Upstream), logger.LOG_LEVEL_INFO)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go t.handle(conn)
	}
}

func (t *Tap) handle(client net.Conn) {
	upstream, err := net.Dial("tcp", t.Upstream)
	if err != nil {
		logger.LogEvent("Tap", "Dial", fmt.Sprintf("upstream %s: %v", t.Upstream, err), logger.LOG_LEVEL_ERROR)
		client.Close()
		return
	}
	s := &session{tap: t, client: client, upstream: upstream}
	t.Sink.line(map[string]any{
		"ts": time.Now().UTC().Format(time.RFC3339Nano), "event": "open",
		"client": client.RemoteAddr().String(), "upstream": t.Upstream,
	})
	done := make(chan struct{}, 2)
	go s.pump(s.upstream, s.client, "CS", done)
	go s.pump(s.client, s.upstream, "SC", done)
	<-done
	s.close()
	<-done
	t.Sink.line(map[string]any{
		"ts": time.Now().UTC().Format(time.RFC3339Nano), "event": "close",
		"client": client.RemoteAddr().String(),
	})
}

type session struct {
	tap       *Tap
	client    net.Conn
	upstream  net.Conn
	closeOnce sync.Once
}

func (s *session) close() {
	s.closeOnce.Do(func() {
		s.client.Close()
		s.upstream.Close()
	})
}

// pump reads framed packets from src, captures them, and forwards the bytes
// verbatim to dst. Once a direction carries non-protocol traffic (e.g. the
// SDK's HTTP probe on the gateway port and its response), the rest of it is
// relayed raw and captured chunk by chunk.
func (s *session) pump(dst io.Writer, src io.Reader, dir string, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	prototype := protocol.ClientMessage
	if dir == "SC" {
		prototype = protocol.ServerMessage
	}
	var buf []byte
	chunk := make([]byte, 32<<10)
	for {
		n, err := src.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		for len(buf) > 0 {
			if len(buf) >= 3 && buf[2] != 0x00 || len(buf) >= 2 && int(buf[0])<<8|int(buf[1]) < protocol.MinFrameSize {
				s.relayRaw(dst, src, dir, buf, err == nil)
				s.close()
				return
			}
			if len(buf) < 2 {
				break
			}
			size := int(buf[0])<<8 | int(buf[1])
			total := 2 + size
			if len(buf) < total {
				break
			}
			frame := buf[:total]
			buf = buf[total:]
			s.capturePacket(dir, prototype, frame)
			if _, err := dst.Write(frame); err != nil {
				return
			}
		}
		if err != nil {
			s.close()
			return
		}
	}
}

func (s *session) capturePacket(dir string, prototype func(int) proto.Message, frame []byte) {
	header := protocol.ParseHeader(frame)
	payload := frame[protocol.HeaderSize:]
	entry := map[string]any{
		"ts":  time.Now().UTC().Format(time.RFC3339Nano),
		"dir": dir, "id": header.ID, "len": len(payload),
	}
	if msg := prototype(header.ID); msg != nil {
		msg = msg.ProtoReflect().New().Interface()
		if err := proto.Unmarshal(payload, msg); err == nil {
			entry["name"] = string(msg.ProtoReflect().Descriptor().Name())
			entry["text"] = prototext.Format(msg)
		} else {
			entry["hex"] = hex.EncodeToString(payload)
		}
	} else {
		entry["hex"] = hex.EncodeToString(payload)
	}
	s.tap.Sink.line(entry)
}

// relayRaw forwards non-protocol bytes verbatim: first pending, then (if more)
// everything else src sends until it closes, capturing each chunk.
func (s *session) relayRaw(dst io.Writer, src io.Reader, dir string, pending []byte, more bool) {
	s.captureRaw(dir, pending)
	if _, err := dst.Write(pending); err != nil || !more {
		return
	}
	chunk := make([]byte, 32<<10)
	for {
		n, err := src.Read(chunk)
		if n > 0 {
			s.captureRaw(dir, chunk[:n])
			if _, werr := dst.Write(chunk[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// captureRaw records a chunk of non-protocol traffic, as text when it is
// printable UTF-8 (HTTP), hex otherwise.
func (s *session) captureRaw(dir string, data []byte) {
	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"event": "raw", "dir": dir, "len": len(data),
	}
	if utf8.Valid(data) && !bytes.ContainsFunc(data, func(r rune) bool {
		return unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t'
	}) {
		entry["text"] = string(data)
	} else {
		entry["hex"] = hex.EncodeToString(data)
	}
	s.tap.Sink.line(entry)
}
