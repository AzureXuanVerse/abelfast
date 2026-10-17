package entrypoint

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
)

func Run() {
	if len(os.Args) > 1 && os.Args[1] == "mail" {
		runMail(os.Args[2:])
		return
	}
	listen := flag.String("listen", "0.0.0.0:80", "listen address (gateway and game server share it)")
	serverHost := flag.String("server-host", "127.0.0.1", "game server host handed to clients in the server list")
	serverPort := flag.Uint("server-port", 80, "game server port handed to clients in the server list")
	upstream := flag.String("upstream-gateway", "blhxusgate.yo-star.com:80", "official gateway, CS_10800 is relayed there for the asset hashes")
	dbPath := flag.String("db", ".cache/belfast.db", "SQLite database path")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		fatal(err)
	}
	store, err := db.Open(*dbPath)
	if err != nil {
		fatal(fmt.Errorf("database %s: %w", *dbPath, err))
	}
	ticketKey, err := gateway.LoadTicketKey(context.Background(), store.Queries)
	if err != nil {
		fatal(err)
	}

	gw := &gateway.Gateway{
		Upstream:  *upstream,
		DB:        store,
		TicketKey: ticketKey,
		Servers:   []gateway.Server{{ID: 1, Name: "Belfast", Host: *serverHost, Port: uint32(*serverPort)}},
	}
	gw.Register()
	(&game.Game{DB: store}).Register()

	server := connection.NewServer(*listen, packets.Dispatch)
	server.HTTPResponse = gateway.ProbeResponse

	// wait for SIGINT
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt)
	go func() {
		<-sigChannel
		fmt.Printf("\r")
		server.DisconnectAll(consts.DR_CONNECTION_TO_SERVER_LOST)
		os.Exit(0)
	}()
	if err := server.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	logger.LogEvent("Server", "Run", fmt.Sprintf("%v", err), logger.LOG_LEVEL_ERROR)
	os.Exit(1)
}
