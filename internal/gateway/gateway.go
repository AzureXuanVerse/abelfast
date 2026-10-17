// Package gateway answers the packets the client sends to the gateway before
// it picks a game server: the version / asset hash check (CS_10800), the
// server list state check (CS_10018) and the account login (CS_10020), plus
// the SDK's HTTP reachability probe on the same port.
package gateway

import (
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Server is one entry of the server list.
type Server struct {
	ID   uint32
	Name string
	Host string
	Port uint32
}

// info is the server's SERVERINFO: state 0 is Server.STATUS's "Online"
// (model/vo/server.lua), tag_state 0 no badge. Servers with sort 0 are the
// candidates for the "Featured" slot, which is also the default server of a
// new device (ServerProxy:setServers). The proxy address is where the client
// retries when Host:Port can't be reached (ConnectionMgr:ConnectByProxy).
func (server Server) info() *protobuf.SERVERINFO {
	return &protobuf.SERVERINFO{
		Ids:       []uint32{server.ID},
		Ip:        proto.String(server.Host),
		Port:      proto.Uint32(server.Port),
		State:     proto.Uint32(0),
		Name:      proto.String(server.Name),
		TagState:  proto.Uint32(0),
		Sort:      proto.Uint32(0),
		ProxyIp:   proto.String(server.Host),
		ProxyPort: proto.Uint32(server.Port),
	}
}

// ProbeResponse answers the SDK's reachability probe (GET /?cmd=test?) as
// the official gateway does (captured).
var ProbeResponse = []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain;charset=utf-8\r\nContent-Length: 8\r\nConnection: keep-alive\r\n\r\nbad args")

// versionTTL is how long an upstream SC_10801 is reused.
const versionTTL = 10 * time.Minute

type Gateway struct {
	Servers []Server
	// Upstream is the official gateway (host:port). CS_10800 is relayed there
	// so the asset bundle hashes in SC_10801 always match the CDN the client
	// downloads from; the reply is passed through unchanged.
	Upstream string
	DB       *db.Store
	// TicketKey signs the server tickets, see LoadTicketKey.
	TicketKey []byte

	// the upstream SC_10801 cache, see version
	mu       sync.Mutex
	versions map[string]cachedVersion
	fetching map[string]*versionFetch
}

type cachedVersion struct {
	msg *protobuf.SC_10801
	at  time.Time
}

// versionFetch is an upstream request in flight, done is closed once msg or
// err is set.
type versionFetch struct {
	done chan struct{}
	msg  *protobuf.SC_10801
	err  error
}

// Register installs the gateway packet handlers.
func (gateway *Gateway) Register() {
	packets.Handle(gateway.handleVersionCheck)
	packets.Handle(gateway.handleServerState)
	packets.Handle(gateway.handleLogin)
	packets.Handle(gateway.handleServerLogin)
	packets.Handle(gateway.handleHeartbeat)
	packets.Handle(gateway.handleCreatePlayer)
}

func (gateway *Gateway) serverList() []*protobuf.SERVERINFO {
	list := make([]*protobuf.SERVERINFO, len(gateway.Servers))
	for i, server := range gateway.Servers {
		list[i] = server.info()
	}
	return list
}

//belfast:static
func (gateway *Gateway) handleVersionCheck(client *connection.Client, req *protobuf.CS_10800) error {
	msg, err := gateway.version(req)
	if err != nil {
		return err
	}
	return client.Send(msg)
}

//belfast:static
func (gateway *Gateway) handleServerState(client *connection.Client, req *protobuf.CS_10018) error {
	return client.Send(&protobuf.SC_10019{Serverlist: gateway.serverList()})
}

// handleLogin accepts any SDK login: arg2 is the SDK account uid, the account
// id is derived from it so it stays the same across logins and restarts.
//
//belfast:static
func (gateway *Gateway) handleLogin(client *connection.Client, req *protobuf.CS_10020) error {
	account := accountID(req.GetArg2())
	ticket := gateway.issueTicket(account)
	logger.LogEvent("Gateway", "Login", fmt.Sprintf("account %d (login type %d) from %s", account, req.GetLoginType(), client.IP), logger.LOG_LEVEL_INFO)
	return client.Send(&protobuf.SC_10021{
		Result:       proto.Uint32(0),
		Serverlist:   gateway.serverList(),
		AccountId:    proto.Uint32(account),
		ServerTicket: proto.String(ticket),
		Device:       proto.Uint32(req.GetDevice()),
	})
}

func accountID(uid string) uint32 {
	hash := fnv.New32a()
	hash.Write([]byte(uid))
	if id := hash.Sum32(); id != 0 {
		return id
	}
	return 1
}
