package admin

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/region"
)

// registerServer registers the API of settings, logs and login accounts.
func (server *Server) registerServer(api apiFunc) {
	api("GET /api/server/settings", server.handleSettings)
	api("POST /api/server/version-check", server.handleVersionCheck)
	api("POST /api/server/logout-all", server.handleLogoutAll)
	api("GET /api/server/logs", server.handleLogs)
	api("GET /api/server/accounts", server.handleAccounts)
	api("GET /api/server/accounts/{id}", server.handleAccount)
	api("POST /api/server/accounts/{id}/password", server.handleAccountPassword)
	api("POST /api/server/accounts/{id}/mail-box", server.handleAccountMailBox)
	api("POST /api/server/accounts/{id}/unlink", server.handleAccountUnlink)
	api("POST /api/server/accounts/{id}/clear-secondary", server.handleAccountClearSecondary)
	api("DELETE /api/server/accounts/{id}/codes/{code}", server.handleAccountForgetCode)
	api("DELETE /api/server/accounts/{id}", server.handleAccountDelete)
}

type listedServer struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port uint32 `json:"port"`
}

// upstreamVersion is an SC_10801 of the upstream gateway: its version
// strings, the client version and asset hash read from the $azhash$ one.
type upstreamVersion struct {
	At            int64    `json:"at"`
	Versions      []string `json:"versions"`
	ClientVersion string   `json:"client_version"`
	AssetHash     string   `json:"asset_hash"`
}

func toUpstreamVersion(msg *protobuf.SC_10801, at time.Time) *upstreamVersion {
	version := &upstreamVersion{At: at.Unix(), Versions: msg.GetVersion()}
	for _, v := range msg.GetVersion() {
		// $azhash$<major>$<minor>$<build>$<hash>
		if fields := strings.Split(v, "$"); len(fields) == 6 && fields[1] == "azhash" {
			version.ClientVersion = strings.Join(fields[2:5], ".")
			version.AssetHash = fields[5]
		}
	}
	return version
}

type settings struct {
	Region string `json:"region"`
	// the game server's listen address (-listen) and the back-office's
	ListenAddr  string `json:"listen_addr"`
	AdminAddr   string `json:"admin_addr"`
	PasswordSet bool   `json:"password_set"`
	Sessions    int    `json:"sessions"`
	// the gateway's server list and upstream
	Servers  []listedServer   `json:"servers"`
	Upstream string           `json:"upstream"`
	Official string           `json:"official"`
	Version  *upstreamVersion `json:"version"`
	// last failed check, kept until one succeeds
	VersionError string `json:"version_error,omitempty"`
	// the database file (-db) and its size with its WAL
	Database      string `json:"database"`
	DatabaseBytes int64  `json:"database_bytes"`
	// the game data: its dir, the region's, and the table files in it
	DataDir     string `json:"data_dir"`
	RegionDir   string `json:"region_dir"`
	Tables      int    `json:"tables"`
	ServerFiles int    `json:"server_files"`
}

// versionCheck is the error of the latest Check now, cleared by a success.
var versionCheck struct {
	mu  sync.Mutex
	err string
}

func (server *Server) handleSettings(r *http.Request) (any, error) {
	result := settings{
		Region:      region.Current(),
		ListenAddr:  server.ListenAddr,
		AdminAddr:   server.Addr,
		PasswordSet: server.Password != "",
		Servers:     []listedServer{},
		DataDir:     absolute(gamedata.Dir()),
		RegionDir:   absolute(gamedata.RegionDir()),
	}
	server.sessions.mu.Lock()
	now := time.Now()
	for _, expires := range server.sessions.expires {
		if now.Before(expires) {
			result.Sessions++
		}
	}
	server.sessions.mu.Unlock()
	if gw := server.Gateway; gw != nil {
		for _, s := range gw.Servers {
			result.Servers = append(result.Servers, listedServer{ID: s.ID, Name: s.Name, Host: s.Host, Port: s.Port})
		}
		result.Upstream = gw.Upstream
		if msg, at := gw.LatestVersion(); msg != nil {
			result.Version = toUpstreamVersion(msg, at)
		}
	}
	result.Official = gateway.Official[result.Region]
	versionCheck.mu.Lock()
	result.VersionError = versionCheck.err
	versionCheck.mu.Unlock()
	result.Database, result.DatabaseBytes = server.databaseFile()
	for _, sub := range []string{"sharecfgdata", "ShareCfg"} {
		result.Tables += countFiles(filepath.Join(gamedata.RegionDir(), sub), ".json")
	}
	result.ServerFiles = countFiles(filepath.Join(gamedata.RegionDir(), "server"), "")
	return result, nil
}

func absolute(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// countFiles counts the files of dir ending in suffix.
func countFiles(dir, suffix string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			n++
		}
	}
	return n
}

// handleVersionCheck asks the upstream gateway for its SC_10801 now.
func (server *Server) handleVersionCheck(r *http.Request) (any, error) {
	if server.Gateway == nil {
		return nil, badRequest("no gateway")
	}
	msg, err := server.Gateway.CheckUpstream()
	versionCheck.mu.Lock()
	defer versionCheck.mu.Unlock()
	if err != nil {
		versionCheck.err = err.Error()
		return nil, httpError{http.StatusBadGateway, err.Error()}
	}
	versionCheck.err = ""
	return toUpstreamVersion(msg, time.Now()), nil
}

// handleLogoutAll ends every back-office session, this one included.
func (server *Server) handleLogoutAll(r *http.Request) (any, error) {
	if server.Password == "" {
		return nil, badRequest("no password set, there are no sessions")
	}
	server.sessions.mu.Lock()
	ended := len(server.sessions.expires)
	server.sessions.expires = nil
	server.sessions.mu.Unlock()
	return map[string]int{"ended": ended}, nil
}
