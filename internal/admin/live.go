package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ggmolly/belfast/internal/live"
	"github.com/ggmolly/belfast/internal/logger"
)

// logsEvery is the most often the log's "logs" event is sent: a busy log
// would otherwise send one per line.
const logsEvery = 300 * time.Millisecond

// watchLogsLive publishes a "logs" event (at most every logsEvery) when
// lines were logged since the last one.
func (server *Server) watchLogsLive() {
	var logged atomic.Bool
	logger.Watch(func(logger.Event) { logged.Store(true) })
	go func() {
		for range time.Tick(logsEvery) {
			if logged.Swap(false) {
				server.Game.Live.Publish(live.Event{Kind: live.Area, Name: "logs"})
			}
		}
	}()
}

// liveHeartbeat is how often an idle event stream gets a comment, so
// proxies and the browser keep it open.
const liveHeartbeat = 25 * time.Second

// handleLive streams what changes (live.Event, as JSON) to an open page,
// as Server-Sent Events, until it goes away.
func (server *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	if !server.loggedIn(w, r) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	events, stop := server.Game.Live.Subscribe()
	defer stop()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": open\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(liveHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		flusher.Flush()
	}
}

// quiet are the writes that change nothing the pages show (a preview, a
// check whose page refetches itself): no event follows them.
var quiet = map[string]bool{
	"POST /api/broadcast/audience":   true,
	"POST /api/server/version-check": true,
}

// changed is what a back-office write changed, from its path: a guild
// (/api/guilds/{id}...), the server's accounts or settings, else the area it
// is under (/api/notices, /api/codes...). A commander's save tells itself
// (game.AdminWrite), so /api/commanders/{id}/... is no event here.
func changed(r *http.Request) (live.Event, bool) {
	if quiet[r.Pattern] {
		return live.Event{}, false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/"), "/"), "/")
	if len(parts) >= 2 {
		if id, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			switch parts[0] {
			case "commanders":
				return live.Event{}, false
			case "guilds":
				return live.Event{Kind: live.Guild, ID: uint32(id)}, true
			}
		}
		if parts[0] == "server" {
			if parts[1] == "accounts" {
				return live.Event{Kind: live.Area, Name: "accounts"}, true
			}
			return live.Event{Kind: live.Area, Name: "settings"}, true
		}
	}
	return live.Event{Kind: live.Area, Name: parts[0]}, true
}
