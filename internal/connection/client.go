package connection

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"
	queue "gopkg.in/eapache/queue.v1"

	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

const (
	packetQueueSize = 512
	packetPoolSize  = 128
)

var (
	errClientClosed = errors.New("client is closed")

	nextClientID atomic.Uint64
)

type clientMetrics struct {
	queueMax      int // guarded by queueMu
	queueBlocks   uint64
	handlerErrors atomic.Uint64
	writeErrors   atomic.Uint64
	packets       atomic.Uint64
}

type Client struct {
	ID          uint64
	IP          net.IP
	Port        int
	ConnectedAt time.Time

	conn   net.Conn
	server *Server

	// Replies are buffered until the current packet has been handled, then flushed.
	writeMu     sync.Mutex
	writeBuffer []byte
	packetIndex int // index of the packet being handled, echoed in replies

	packetQueue  *queue.Queue
	queueMu      sync.Mutex
	queueCond    *sync.Cond
	packetPool   chan []byte
	closed       bool
	closeOnce    sync.Once
	dispatchOnce sync.Once
	metrics      clientMetrics
}

func newClient(conn net.Conn, server *Server) *Client {
	addr := conn.RemoteAddr().(*net.TCPAddr)
	client := &Client{
		ID:          nextClientID.Add(1),
		IP:          addr.IP,
		Port:        addr.Port,
		ConnectedAt: time.Now().UTC(),
		conn:        conn,
		server:      server,
		packetQueue: queue.New(),
		packetPool:  make(chan []byte, packetPoolSize),
	}
	client.queueCond = sync.NewCond(&client.queueMu)
	return client
}

// Send buffers a server message, its packet id is taken from the message name (SC_<id>).
func (client *Client) Send(msg proto.Message) error {
	id, err := protocol.ServerPacketID(msg)
	if err != nil {
		return err
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("SC_%d: %w", id, err)
	}
	client.writeMu.Lock()
	client.writeBuffer = protocol.AppendFrame(client.writeBuffer, id, client.packetIndex, payload)
	client.writeMu.Unlock()
	logger.LogEvent("Connection", "Send", fmt.Sprintf("SC_%d - %d bytes buffered", id, len(payload)), logger.LOG_LEVEL_DEBUG)
	return nil
}

// Disconnect buffers SC_10999 (disconnected from server), reasons are defined in consts/disconnect_reasons.go
func (client *Client) Disconnect(reason uint8) error {
	return client.Send(&protobuf.SC_10999{
		Reason: proto.Uint32(uint32(reason)),
	})
}

// flush writes the buffered replies to the socket.
func (client *Client) flush() error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	if len(client.writeBuffer) == 0 {
		return nil
	}
	_, err := client.conn.Write(client.writeBuffer)
	client.writeBuffer = client.writeBuffer[:0]
	if err != nil {
		client.metrics.writeErrors.Add(1)
		return err
	}
	return nil
}

func (client *Client) Close() {
	client.closeWithError(nil)
}

func (client *Client) closeWithError(err error) {
	client.closeOnce.Do(func() {
		client.queueMu.Lock()
		client.closed = true
		client.queueCond.Broadcast()
		client.queueMu.Unlock()
		if err != nil && !isClosedConnError(err) {
			logger.LogEvent("Client", "Close", fmt.Sprintf("%s:%d -> %v", client.IP, client.Port, err), logger.LOG_LEVEL_ERROR)
		}
		_ = client.conn.Close()
		client.logMetrics()
	})
}

func (client *Client) isClosed() bool {
	client.queueMu.Lock()
	defer client.queueMu.Unlock()
	return client.closed
}

func (client *Client) logMetrics() {
	client.queueMu.Lock()
	queueMax := client.metrics.queueMax
	queueBlocks := client.metrics.queueBlocks
	client.queueMu.Unlock()
	logger.LogEvent("Metrics", "ClientStats", fmt.Sprintf("%s:%d queueMax=%d queueBlocks=%d handlerErrors=%d writeErrors=%d packets=%d",
		client.IP, client.Port, queueMax, queueBlocks,
		client.metrics.handlerErrors.Load(), client.metrics.writeErrors.Load(), client.metrics.packets.Load()), logger.LOG_LEVEL_INFO)
}

func (client *Client) startDispatcher() {
	client.dispatchOnce.Do(func() {
		go client.dispatchLoop()
	})
}

func (client *Client) enqueuePacket(packet []byte) error {
	client.queueMu.Lock()
	defer client.queueMu.Unlock()
	for client.packetQueue.Length() >= packetQueueSize && !client.closed {
		client.metrics.queueBlocks++
		logger.LogEvent("Metrics", "QueueBlock", fmt.Sprintf("%s:%d depth=%d", client.IP, client.Port, client.packetQueue.Length()), logger.LOG_LEVEL_DEBUG)
		client.queueCond.Wait()
	}
	if client.closed {
		return errClientClosed
	}
	client.packetQueue.Add(packet)
	depth := client.packetQueue.Length()
	if depth > client.metrics.queueMax {
		client.metrics.queueMax = depth
		logger.LogEvent("Metrics", "QueueDepth", fmt.Sprintf("%s:%d depth=%d", client.IP, client.Port, depth), logger.LOG_LEVEL_DEBUG)
	}
	client.queueCond.Signal()
	return nil
}

func (client *Client) dispatchLoop() {
	for {
		client.queueMu.Lock()
		for client.packetQueue.Length() == 0 && !client.closed {
			client.queueCond.Wait()
		}
		if client.closed {
			client.drainQueueLocked()
			client.queueMu.Unlock()
			return
		}
		packet := client.packetQueue.Remove().([]byte)
		client.queueCond.Signal()
		client.queueMu.Unlock()

		client.metrics.packets.Add(1)
		err := client.handlePacket(packet)
		client.releasePacketBuffer(packet)
		if err != nil {
			client.closeWithError(err)
			return
		}
	}
}

// handlePacket runs the dispatcher on a complete frame and flushes its replies.
func (client *Client) handlePacket(packet []byte) error {
	header := protocol.ParseHeader(packet)
	client.writeMu.Lock()
	client.packetIndex = header.Index
	client.writeMu.Unlock()
	if err := client.server.dispatcher(client, header.ID, packet[protocol.HeaderSize:]); err != nil {
		client.metrics.handlerErrors.Add(1)
		return err
	}
	return client.flush()
}

func (client *Client) acquirePacketBuffer(size int) []byte {
	select {
	case buf := <-client.packetPool:
		if cap(buf) >= size {
			return buf[:size]
		}
	default:
	}
	return make([]byte, size)
}

func (client *Client) releasePacketBuffer(buffer []byte) {
	select {
	case client.packetPool <- buffer[:0]:
	default:
	}
}

func (client *Client) drainQueueLocked() {
	for client.packetQueue.Length() > 0 {
		packet := client.packetQueue.Remove().([]byte)
		client.releasePacketBuffer(packet)
	}
}
