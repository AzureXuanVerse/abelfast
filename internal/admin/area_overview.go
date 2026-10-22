package admin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/region"
)

// processStart is about when belfast started: the package's variables are
// set before main runs.
var processStart = time.Now()

// registerOverview registers the API of the dashboard and online now.
func (server *Server) registerOverview(api apiFunc) {
	api("GET /api/overview", server.handleOverview)
	api("GET /api/overview/online", server.handleOnlinePlayers)
	api("GET /api/overview/commanders", server.handleRoster)
	api("POST /api/overview/kick-all", server.handleKickAll)
}

type serverStatus struct {
	// the game server address the gateway's server list hands out
	Address   string `json:"address"`
	Region    string `json:"region"`
	StartedAt int64  `json:"started_at"`
	// the client version and asset hash of the latest SC_10801 relayed from
	// the official gateway, empty before the first one
	ClientVersion string `json:"client_version"`
	AssetHash     string `json:"asset_hash"`
	AssetsAt      int64  `json:"assets_at"`
	AssetsError   string `json:"assets_error,omitempty"`
	// the database file and its size with its WAL, empty when not found
	Database      string `json:"database"`
	DatabaseBytes int64  `json:"database_bytes"`
}

type overviewCounts struct {
	Online     int   `json:"online"`
	Commanders int   `json:"commanders"`
	NewToday   int   `json:"new_today"`
	MailsToday int64 `json:"mails_today"`
}

// activity is something that happened today, read back from the tables'
// times: a registration, a login or logout (last_online is set at both), a
// mail sent.
type activity struct {
	Kind    string `json:"kind"`
	Time    int64  `json:"time"`
	Actor   string `json:"actor"`
	ActorID uint32 `json:"actor_id,omitempty"`
	What    string `json:"what"`
}

type overview struct {
	Server    serverStatus   `json:"server"`
	Counts    overviewCounts `json:"counts"`
	DayStart  int64          `json:"day_start"`
	NextReset int64          `json:"next_reset"`
	Activity  []activity     `json:"activity"`
}

// activityLimit is how many of today's events the dashboard lists.
const activityLimit = 12

// dayStart is the midnight, server time, that began now's day: the daily
// reset (see game.startOfDay).
func dayStart(now time.Time) time.Time {
	local := now.In(gamedata.Zone())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, gamedata.Zone())
}

// handleOverview is the dashboard: the server's state, today's numbers and
// what happened today.
func (server *Server) handleOverview(r *http.Request) (any, error) {
	ctx, now := r.Context(), time.Now()
	start := dayStart(now)
	result := overview{
		Server:    server.status(),
		DayStart:  start.Unix(),
		NextReset: start.AddDate(0, 0, 1).Unix(),
		Activity:  []activity{},
	}
	online := server.Game.Online()
	ips := make(map[uint32]string, len(online))
	for _, player := range online {
		ips[player.ID] = player.IP
	}
	result.Counts.Online = len(online)
	commanders, err := server.Store.ListCommanders(ctx)
	if err != nil {
		return nil, err
	}
	result.Counts.Commanders = len(commanders)
	for _, c := range commanders {
		if c.RegisteredAt >= start.Unix() {
			result.Counts.NewToday++
			result.Activity = append(result.Activity, activity{Kind: "new", Time: c.RegisteredAt, Actor: c.Name, ActorID: c.ID, What: "Registered"})
		}
		if c.LastOnline >= start.Unix() {
			if ip, ok := ips[c.ID]; ok {
				result.Activity = append(result.Activity, activity{Kind: "login", Time: c.LastOnline, Actor: c.Name, ActorID: c.ID, What: "Logged in from " + ip})
			} else {
				result.Activity = append(result.Activity, activity{Kind: "logout", Time: c.LastOnline, Actor: c.Name, ActorID: c.ID, What: "Logged out"})
			}
		}
	}
	if result.Counts.MailsToday, err = server.Store.CountMailsSince(ctx, start.Unix()); err != nil {
		return nil, err
	}
	batches, err := server.Store.ListMailBatchesSince(ctx, db.ListMailBatchesSinceParams{Date: start.Unix(), Limit: activityLimit})
	if err != nil {
		return nil, err
	}
	noSender, err := mailSenderDefault()
	if err != nil {
		return nil, err
	}
	for _, batch := range batches {
		if batch.Sender == "" {
			batch.Sender = noSender
		}
		what := fmt.Sprintf("Mail %q", batch.Title)
		if batch.Recipients > 1 {
			what += fmt.Sprintf(" to %d commanders", batch.Recipients)
		}
		result.Activity = append(result.Activity, activity{Kind: "mail", Time: batch.Date, Actor: batch.Sender, What: what})
	}
	sort.SliceStable(result.Activity, func(i, j int) bool { return result.Activity[i].Time > result.Activity[j].Time })
	if len(result.Activity) > activityLimit {
		result.Activity = result.Activity[:activityLimit]
	}
	return result, nil
}

// mailSenderDefault is what the client shows for a mail without a sender
// (gametip mail_sender_default), read once.
var mailSenderDefault = sync.OnceValues(func() (string, error) {
	tips, err := gamedata.Gametip()
	if err != nil {
		return "", err
	}
	return tips["mail_sender_default"], nil
})

// assetWait is how long the dashboard waits for the official gateway when
// no SC_10801 was relayed yet.
const assetWait = 2 * time.Second

func (server *Server) status() serverStatus {
	status := serverStatus{Region: region.Current(), StartedAt: processStart.Unix()}
	status.Address = server.ListenAddr
	// AssetHash refreshes the gateway's cache in the background (or uses
	// it); the dashboard only waits for it, and shows its error, when
	// nothing was ever relayed
	fetched := make(chan error, 1)
	go func() {
		_, err := server.Gateway.AssetHash(gateway.AssetsMain)
		fetched <- err
	}()
	msg, at := server.Gateway.LatestVersion()
	if msg == nil {
		select {
		case err := <-fetched:
			if err != nil {
				status.AssetsError = err.Error()
			}
		case <-time.After(assetWait):
		}
		msg, at = server.Gateway.LatestVersion()
	}
	if msg != nil {
		status.AssetsAt = at.Unix()
		for _, version := range msg.GetVersion() {
			// $azhash$<major>$<minor>$<build>$<hash> (the captures' SC_10801)
			if fields := strings.Split(version, "$"); len(fields) == 6 && fields[1] == "azhash" {
				status.ClientVersion = strings.Join(fields[2:5], ".")
				status.AssetHash = fields[5]
			}
		}
	} else if status.AssetsError == "" {
		status.AssetsError = "no reply from the official gateway yet"
	}
	status.Database, status.DatabaseBytes = server.databaseFile()
	return status
}

// databaseFile is the database's path (-db) and its size with the WAL.
func (server *Server) databaseFile() (string, int64) {
	var size int64
	for _, file := range []string{server.DBPath, server.DBPath + "-wal"} {
		if info, err := os.Stat(file); err == nil {
			size += info.Size()
		}
	}
	return server.DBPath, size
}

// rosterRow is a commander row with its avatar and level progress.
type rosterRow struct {
	commanderRow
	// the squareicon of its flagship (first secretary), its skin's when it
	// wears one
	Icon string `json:"icon,omitempty"`
	// the flagship's rarity (ship_data_statistics), for the icon's frame
	Rarity uint32 `json:"rarity,omitempty"`
	// online now only: exp into the level, and the level's exp to the next
	// (user_level exp)
	Exp     uint32 `json:"exp,omitempty"`
	ExpNext uint32 `json:"exp_next,omitempty"`
}

// userLevels is user_level's exp to the next level, by level, loaded once.
var userLevels = sync.OnceValues(func() (map[uint32]struct {
	Exp uint32 `json:"exp"`
}, error) {
	return gamedata.Table[struct {
		Exp uint32 `json:"exp"`
	}]("user_level")
})

// avatar is a commander's flagship as its icon shows it.
type avatar struct {
	Icon   string
	Rarity uint32
}

// avatars is every commander's flagship icon, by commander id.
func (server *Server) avatars(ctx context.Context) (map[uint32]avatar, error) {
	rows, err := server.Store.ListFlagshipDisplays(ctx)
	if err != nil {
		return nil, err
	}
	icons := make(map[uint32]avatar, len(rows))
	for _, row := range rows {
		icons[row.CommanderID] = avatar{Icon: server.names.shipIcon(row.TemplateID, row.SkinID), Rarity: server.names[dropShip][row.TemplateID].Rarity}
	}
	return icons, nil
}

// handleOnlinePlayers lists the commanders playing with their avatar, IP
// and level progress.
func (server *Server) handleOnlinePlayers(r *http.Request) (any, error) {
	ctx := r.Context()
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	levels, err := userLevels()
	if err != nil {
		return nil, err
	}
	commanders, ips, err := server.onlineCommanders(ctx)
	if err != nil {
		return nil, err
	}
	rows := []rosterRow{}
	for _, c := range commanders {
		row := rosterRow{commanderRow: toRow(c.ID, c.AccountID, c.Name, c.Level, c.RegisteredAt, c.LastOnline), Icon: icons[c.ID].Icon, Rarity: icons[c.ID].Rarity, Exp: c.Exp, ExpNext: levels[c.Level].Exp}
		row.Online, row.IP = true, ips[c.ID]
		rows = append(rows, row)
	}
	return rows, nil
}

// handleRoster lists every commander with its avatar.
func (server *Server) handleRoster(r *http.Request) (any, error) {
	ctx := r.Context()
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	commanders, err := server.Store.ListCommanders(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]rosterRow, 0, len(commanders))
	for _, c := range commanders {
		row := rosterRow{commanderRow: toRow(c.ID, c.AccountID, c.Name, c.Level, c.RegisteredAt, c.LastOnline), Icon: icons[c.ID].Icon, Rarity: icons[c.ID].Rarity}
		row.Online = server.Game.IsOnline(c.ID)
		rows = append(rows, row)
	}
	return rows, nil
}

// handleKickAll disconnects every commander playing.
func (server *Server) handleKickAll(r *http.Request) (any, error) {
	kicked := 0
	var failed []string
	for _, online := range server.Game.Online() {
		was, err := server.Game.Kick(online.ID)
		if err != nil {
			failed = append(failed, err.Error())
		}
		if was {
			kicked++
		}
	}
	if len(failed) > 0 {
		return nil, fmt.Errorf("kicked %d, %s", kicked, strings.Join(failed, "; "))
	}
	return map[string]int{"kicked": kicked}, nil
}
