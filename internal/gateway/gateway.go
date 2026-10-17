// Package gateway answers the packets the client sends to the gateway before
// it picks a game server: the version / asset hash check (CS_10800), the
// server list state check (CS_10018) and the account login (CS_10020), plus
// the SDK's HTTP reachability probe on the same port.
package gateway

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// State is SERVERINFO.state, the client's Server.STATUS (model/vo/server.lua).
// The login scene colors the server by it, matching the in-game legend.
type State uint32

const (
	StateNormal       State = 0 // green "Online"
	StateMaintenance  State = 1 // grey "Offline", not selectable (login_loginScene_server_vindicate)
	StateFull         State = 2 // red "Full", not selectable (login_loginScene_server_full)
	StateRegisterFull State = 3 // orange "Busy", selectable but closed to new commanders
)

// Tag is SERVERINFO.tag_state, the badge drawn on the server.
type Tag uint32

const (
	TagNone Tag = 0
	TagHot  Tag = 1
	TagNew  Tag = 2
)

// Server is one entry of the server list.
type Server struct {
	ID   uint32
	Name string
	Host string
	Port uint32
	// ProxyHost / ProxyPort is where the client retries when Host:Port can't
	// be reached (ConnectionMgr:ConnectByProxy), empty means Host:Port.
	ProxyHost string
	ProxyPort uint32
	State     State
	Tag       Tag
	// Sort orders the list. Servers with Sort 0 are the candidates for the
	// "Featured" slot, which is also the default server of a new device
	// (ServerProxy:setServers); without one it falls back to the last server.
	Sort uint32
}

func (server Server) info() *protobuf.SERVERINFO {
	proxyHost, proxyPort := server.ProxyHost, server.ProxyPort
	if proxyHost == "" {
		proxyHost, proxyPort = server.Host, server.Port
	}
	return &protobuf.SERVERINFO{
		Ids:       []uint32{server.ID},
		Ip:        proto.String(server.Host),
		Port:      proto.Uint32(server.Port),
		State:     proto.Uint32(uint32(server.State)),
		Name:      proto.String(server.Name),
		TagState:  proto.Uint32(uint32(server.Tag)),
		Sort:      proto.Uint32(server.Sort),
		ProxyIp:   proto.String(proxyHost),
		ProxyPort: proto.Uint32(proxyPort),
	}
}

// DefaultVersionTTL is how long an upstream SC_10801 is reused.
const DefaultVersionTTL = 10 * time.Minute

type Gateway struct {
	Servers []Server
	// Upstream is the official gateway (host:port). CS_10800 is relayed there
	// so the asset bundle hashes in SC_10801 always match the CDN the client
	// downloads from; the reply is passed through unchanged.
	Upstream   string
	VersionTTL time.Duration
	DB         *db.Store
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

// HTTP answers the SDK's reachability probe (GET /?cmd=test?) the way the
// official gateway does.
func (gateway *Gateway) HTTP() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain;charset=utf-8")
		w.Write([]byte("bad args"))
	})
}

func (gateway *Gateway) serverList() []*protobuf.SERVERINFO {
	list := make([]*protobuf.SERVERINFO, len(gateway.Servers))
	for i, server := range gateway.Servers {
		list[i] = server.info()
	}
	return list
}

func (gateway *Gateway) handleVersionCheck(client *connection.Client, req *protobuf.CS_10800) error {
	msg, err := gateway.version(req)
	if err != nil {
		return err
	}
	return client.Send(msg)
}

func (gateway *Gateway) handleServerState(client *connection.Client, req *protobuf.CS_10018) error {
	return client.Send(&protobuf.SC_10019{Serverlist: gateway.serverList()})
}

// handleLogin accepts any SDK login: arg2 is the SDK account uid, the account
// id is derived from it so it stays the same across logins and restarts.
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
