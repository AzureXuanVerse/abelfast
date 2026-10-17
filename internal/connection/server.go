package connection

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/smallnest/ringbuffer"

	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protocol"
)

const (
	readBufferSize = 32 << 10
)

// Dispatcher handles one packet. The payload is only valid during the call;
// returning an error closes the connection.
type Dispatcher func(client *Client, packetID int, payload []byte) error

type Server struct {
	address    string
	dispatcher Dispatcher

	// HTTPResponse, when set, answers every request of a connection that
	// opens with an HTTP request instead of a frame (the SDK probes the
	// gateway port with GET /?cmd=test?).
	HTTPResponse []byte

	clientsMu sync.Mutex
	clients   map[uint64]*Client
}

func NewServer(address string, dispatcher Dispatcher) *Server {
	return &Server{
		address:    address,
		dispatcher: dispatcher,
		clients:    make(map[uint64]*Client),
	}
}

func (server *Server) Run() error {
	listener, err := net.Listen("tcp", server.address)
	if err != nil {
		logger.LogEvent("Server", "Run", fmt.Sprintf("error listening: %v", err), logger.LOG_LEVEL_ERROR)
		return err
	}
	defer listener.Close()
	logger.LogEvent("Server", "Run", fmt.Sprintf("listening on %s", server.address), logger.LOG_LEVEL_INFO)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.LogEvent("Server", "Run", fmt.Sprintf("error accepting: %v", err), logger.LOG_LEVEL_ERROR)
			continue
		}
		go server.handleConnection(conn)
	}
}

// DisconnectAll sends SC_10999 (disconnected from server) to every connected client and closes them,
// reasons are defined in consts/disconnect_reasons.go
func (server *Server) DisconnectAll(reason uint8) {
	server.clientsMu.Lock()
	defer server.clientsMu.Unlock()
	for _, client := range server.clients {
		logger.LogEvent("Server", "Disconnect", fmt.Sprintf("disconnecting %s:%d -> %s", client.IP, client.Port, consts.ResolveReason(reason)), logger.LOG_LEVEL_DEBUG)
		if err := client.Disconnect(reason); err == nil {
			if err := client.flush(); err != nil {
				logger.LogEvent("Server", "Disconnect", fmt.Sprintf("failed to flush %s:%d -> %v", client.IP, client.Port, err), logger.LOG_LEVEL_ERROR)
			}
		}
		client.Close()
		delete(server.clients, client.ID)
	}
}

func (server *Server) addClient(client *Client) {
	logger.LogEvent("Server", "Hello", fmt.Sprintf("new connection from %s:%d", client.IP, client.Port), logger.LOG_LEVEL_DEBUG)
	server.clientsMu.Lock()
	defer server.clientsMu.Unlock()
	server.clients[client.ID] = client
}

func (server *Server) removeClient(client *Client) {
	server.clientsMu.Lock()
	defer server.clientsMu.Unlock()
	logger.LogEvent("Server", "Goodbye", fmt.Sprintf("%s:%d", client.IP, client.Port), logger.LOG_LEVEL_DEBUG)
	client.Close()
	delete(server.clients, client.ID)
}

func (server *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return
	}
	if server.HTTPResponse != nil && isHTTPRequest(head[:]) {
		server.serveHTTP(conn, head[:])
		return
	}
	client := newClient(conn, server)
	logger.WithFields("Server", logger.FieldValue("remote", conn.RemoteAddr().String()), logger.FieldValue("local", conn.LocalAddr().String())).Info("connection accepted")

	server.addClient(client)
	client.startDispatcher()

	ring := ringbuffer.New(readBufferSize).SetBlocking(true)
	if _, err := ring.Write(head[:]); err != nil {
		server.removeClient(client)
		return
	}
	go func() {
		_, err := ring.ReadFrom(conn)
		if err != nil && !isClosedConnError(err) {
			logger.LogEvent("Server", "Read", fmt.Sprintf("%s:%d -> %v", client.IP, client.Port, err), logger.LOG_LEVEL_ERROR)
		}
		ring.CloseWriter()
	}()

	for {
		if client.isClosed() {
			server.removeClient(client)
			return
		}
		sizeHeader, size, err := readPacketSize(ring)
		if err != nil {
			if !isClosedConnError(err) {
				logger.LogEvent("Server", "Read", fmt.Sprintf("%s:%d -> %v", client.IP, client.Port, err), logger.LOG_LEVEL_ERROR)
			}
			server.removeClient(client)
			return
		}
		logger.WithFields("Server", logger.FieldValue("remote", conn.RemoteAddr().String()), logger.FieldValue("size", size)).Debug("reading packet body")
		packet := client.acquirePacketBuffer(size)
		copy(packet[:2], sizeHeader[:])
		if _, err := io.ReadFull(ring, packet[2:]); err != nil {
			client.releasePacketBuffer(packet)
			if !isClosedConnError(err) {
				logger.LogEvent("Server", "Read", fmt.Sprintf("%s:%d -> %v", client.IP, client.Port, err), logger.LOG_LEVEL_ERROR)
			}
			server.removeClient(client)
			return
		}
		if err := client.enqueuePacket(packet); err != nil {
			client.releasePacketBuffer(packet)
			server.removeClient(client)
			return
		}
	}
}

// readPacketSize reads the size field of the next frame and returns the full frame size.
func readPacketSize(ring *ringbuffer.RingBuffer) ([2]byte, int, error) {
	var header [2]byte
	if _, err := io.ReadFull(ring, header[:]); err != nil {
		return header, 0, err
	}
	size := int(header[0])<<8 | int(header[1])
	if size < protocol.MinFrameSize {
		return header, 0, fmt.Errorf("invalid packet size %d", size)
	}
	return header, size + 2, nil
}

// isClosedConnError reports errors caused by the peer or the server closing the connection.
func isClosedConnError(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, ringbuffer.ErrWriteOnClosed) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)
}

var httpMethods = [][]byte{[]byte("GET "), []byte("POST"), []byte("HEAD"), []byte("PUT "), []byte("OPTI"), []byte("DELE"), []byte("PATC")}

// isHTTPRequest reports whether the first bytes of a connection start an HTTP
// request. A frame's third byte is always 0x00, so the two never overlap.
func isHTTPRequest(head []byte) bool {
	for _, method := range httpMethods {
		if bytes.Equal(head, method) {
			return true
		}
	}
	return false
}

// httpIdleTimeout is how long an HTTP connection may wait for its next request.
const httpIdleTimeout = 30 * time.Second

// serveHTTP answers each request of an HTTP connection (head being its
// already read first bytes) with server.HTTPResponse until the client closes
// it: only the request head is read, the probe has no body.
func (server *Server) serveHTTP(conn net.Conn, head []byte) {
	logger.WithFields("Server", logger.FieldValue("remote", conn.RemoteAddr().String())).Debug("http connection")
	reader := bufio.NewReader(io.MultiReader(bytes.NewReader(head), conn))
	for {
		conn.SetReadDeadline(time.Now().Add(httpIdleTimeout))
		for {
			line, err := reader.ReadSlice('\n')
			if err != nil {
				return
			}
			if len(bytes.TrimRight(line, "\r\n")) == 0 {
				break
			}
		}
		if _, err := conn.Write(server.HTTPResponse); err != nil {
			return
		}
	}
}
