package entrypoint

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
)

func Run() {
	listen := flag.String("listen", "0.0.0.0:80", "listen address (gateway and game server share it)")
	serverHost := flag.String("server-host", "127.0.0.1", "game server host handed to clients in the server list")
	serverPort := flag.Uint("server-port", 80, "game server port handed to clients in the server list")
	upstream := flag.String("upstream-gateway", "blhxusgate.yo-star.com:80", "official gateway, CS_10800 is relayed there for the asset hashes")
	flag.Parse()

	gw := &gateway.Gateway{
		Upstream: *upstream,
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
		logger.LogEvent("Server", "Run", fmt.Sprintf("%v", err), logger.LOG_LEVEL_ERROR)
		os.Exit(1)
	}
}
