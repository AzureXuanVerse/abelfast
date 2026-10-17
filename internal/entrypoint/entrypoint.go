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
	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
)

func Run() {
	listen := flag.String("listen", "0.0.0.0:80", "listen address (gateway and game server share it)")
	serverHost := flag.String("server-host", "127.0.0.1", "game server host handed to clients in the server list")
	serverPort := flag.Uint("server-port", 80, "game server port handed to clients in the server list")
	upstream := flag.String("upstream-gateway", "blhxusgate.yo-star.com:80", "official gateway, CS_10800 is relayed there for the asset hashes")
	dbPath := flag.String("db", ".cache/belfast.db", "SQLite database path")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		fatal(err)
	}
	conn, err := db.Open(*dbPath)
	if err != nil {
		fatal(fmt.Errorf("database %s: %w", *dbPath, err))
	}
	queries := db.New(conn)
	ticketKey, err := gateway.LoadTicketKey(context.Background(), queries)
	if err != nil {
		fatal(err)
	}

	gw := &gateway.Gateway{
		Upstream:  *upstream,
		DB:        queries,
		TicketKey: ticketKey,
		Servers: []gateway.Server{{
			ID: 1, Name: "Belfast", Host: *serverHost, Port: uint32(*serverPort),
			State: gateway.StateNormal, Tag: gateway.TagNone, Sort: 0,
		}},
	}
	gw.Register()

	server := connection.NewServer(*listen, packets.Dispatch)
	server.HTTP = gw.HTTP()

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
