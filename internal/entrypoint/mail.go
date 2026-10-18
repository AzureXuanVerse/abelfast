package entrypoint

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// runMail is `belfast mail [flags]`: it stores a mail for a commander in the
// database, next to a running server or not. The client sees it from the
// next SC_30001: its next login, or the end of its next battle. With
// -compensate it is a compensation instead (SC_30101, at the next login).
func runMail(args []string) {
	flags := flag.NewFlagSet("mail", flag.ExitOnError)
	dbPath := flags.String("db", ".cache/belfast.db", "SQLite database path")
	commanderID := flags.Uint("commander", 0, "commander id, 0 for the only commander")
	title := flags.String("title", "", "mail title")
	sender := flags.String("sender", "", "sender, empty for the client's default (Headquarters)")
	body := flags.String("body", "", `mail text, "\n" for a new line`)
	attach := flags.String("attach", "", "attachments, type:id:count[,type:id:count...] (DROP_TYPE_*: 1 resource, 2 item, 3 equipment, 4 ship, 5 furniture, 7 skin, 8 virtual item, 25 meowfficer)")
	compensate := flags.Duration("compensate", 0, "send a compensation expiring after this long (e.g. 72h) instead of a mail; the client sees it from its next login")
	flags.Parse(args)

	attachments, err := parseDrops(*attach)
	if err != nil {
		fatal(err)
	}
	if err := game.LoadGameData(); err != nil {
		fatal(fmt.Errorf("game data: %w", err))
	}
	store, err := db.Open(*dbPath)
	if err != nil {
		fatal(fmt.Errorf("database %s: %w", *dbPath, err))
	}
	ctx := context.Background()
	commander := uint32(*commanderID)
	if commander == 0 {
		ids, err := store.ListCommanderIDs(ctx)
		if err != nil {
			fatal(err)
		}
		if len(ids) != 1 {
			fatal(fmt.Errorf("%d commanders in %s, pick one with -commander", len(ids), *dbPath))
		}
		commander = ids[0]
	} else if _, err := store.GetCommander(ctx, commander); err != nil {
		fatal(fmt.Errorf("commander %d: %w", commander, err))
	}
	mail := game.Mail{Title: *title, Sender: *sender, Content: *body, Attachments: attachments}
	now := time.Now()
	if *compensate != 0 {
		id, err := game.SendCompensation(ctx, store.Queries, commander, mail, now, now.Add(*compensate))
		if err != nil {
			fatal(err)
		}
		fmt.Fprintf(os.Stdout, "compensation %d sent to commander %d\n", id, commander)
		return
	}
	id, err := game.SendMail(ctx, store.Queries, commander, mail, now)
	if err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stdout, "mail %d sent to commander %d\n", id, commander)
}

// parseDrops reads type:id:count[,...].
func parseDrops(list string) ([]*protobuf.DROPINFO, error) {
	var drops []*protobuf.DROPINFO
	for _, entry := range strings.Split(list, ",") {
		if entry = strings.TrimSpace(entry); entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("attachment %q: want type:id:count", entry)
		}
		var values [3]uint32
		for i, part := range parts {
			value, err := strconv.ParseUint(part, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("attachment %q: %w", entry, err)
			}
			values[i] = uint32(value)
		}
		drops = append(drops, &protobuf.DROPINFO{Type: proto.Uint32(values[0]), Id: proto.Uint32(values[1]), Number: proto.Uint32(values[2])})
	}
	return drops, nil
}
