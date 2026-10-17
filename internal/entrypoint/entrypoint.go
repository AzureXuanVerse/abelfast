package entrypoint

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
)

const (
	bindAddress = "0.0.0.0"
	port        = 80
)

func Run() {
	server := connection.NewServer(bindAddress, port, packets.Dispatch)

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
