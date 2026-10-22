package entrypoint

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ggmolly/belfast/internal/admin"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gateway"
	"github.com/ggmolly/belfast/internal/live"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/region"
)

// serveOptions are the server's flags (belfast without a subcommand).
type serveOptions struct {
	listen        string
	serverHost    string
	serverPort    uint
	upstream      string
	admin         string
	adminPassword string
}

func Run() {
	if err := rootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCommand is `belfast [flags]`, the server, with the database tools as
// subcommands. --region and --db apply to every command.
func rootCommand() *cobra.Command {
	var regionName, dbPath string
	var opts serveOptions
	root := &cobra.Command{
		Use:          "belfast",
		Short:        "Azur Lane private server: gateway and game server on one port",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return region.Set(regionName)
		},
		Run: func(cmd *cobra.Command, args []string) {
			serve(dbPath, opts)
		},
	}
	persistent := root.PersistentFlags()
	persistent.StringVar(&regionName, "region", region.Default, "client region ("+strings.Join(region.All, ", ")+"), the game data is read from $BELFAST_DATA/<region>")
	persistent.StringVar(&dbPath, "db", ".cache/belfast.db", "SQLite database path")
	root.RegisterFlagCompletionFunc("region", cobra.FixedCompletions(region.All, cobra.ShellCompDirectiveNoFileComp))

	flags := root.Flags()
	flags.StringVar(&opts.listen, "listen", "0.0.0.0:80", "listen address (gateway and game server share it)")
	flags.StringVar(&opts.serverHost, "server-host", "127.0.0.1", "game server host handed to clients in the server list")
	flags.UintVar(&opts.serverPort, "server-port", 80, "game server port handed to clients in the server list")
	flags.StringVar(&opts.upstream, "upstream-gateway", "", "official gateway, CS_10800 is relayed there for the asset hashes (default: the region's official gateway)")
	flags.StringVar(&opts.admin, "admin", "127.0.0.1:8081", "back-office listen address, empty to disable")
	flags.StringVar(&opts.adminPassword, "admin-password", os.Getenv("BELFAST_ADMIN_PASSWORD"), "back-office password (default $BELFAST_ADMIN_PASSWORD)")

	root.AddCommand(mailCommand(&dbPath), noticeCommand(&dbPath))
	return root
}

func serve(dbPath string, opts serveOptions) {
	upstream := opts.upstream
	if upstream == "" {
		upstream = gateway.Official[region.Current()]
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		fatal(err)
	}
	store, err := db.Open(dbPath)
	if err != nil {
		fatal(fmt.Errorf("database %s: %w", dbPath, err))
	}
	ticketKey, err := gateway.LoadTicketKey(context.Background(), store.Queries)
	if err != nil {
		fatal(err)
	}

	gw := &gateway.Gateway{
		Upstream:  upstream,
		DB:        store,
		TicketKey: ticketKey,
		Servers:   []gateway.Server{{ID: 1, Name: "Belfast", Host: opts.serverHost, Port: uint32(opts.serverPort)}},
	}
	gw.Register()
	gameServer := &game.Game{DB: store, Live: &live.Broker{}}
	gameServer.Register()
	if opts.admin != "" {
		backOffice := &admin.Server{
			Addr:       opts.admin,
			Password:   opts.adminPassword,
			Store:      store,
			Game:       gameServer,
			Gateway:    gw,
			AssetDir:   filepath.Join(filepath.Dir(dbPath), "assets"),
			ListenAddr: opts.listen,
			DBPath:     dbPath,
		}
		go func() {
			if err := backOffice.Run(); err != nil {
				logger.LogEvent("Admin", "Run", err.Error(), logger.LOG_LEVEL_ERROR)
			}
		}()
	}

	server := connection.NewServer(opts.listen, packets.Dispatch)
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
