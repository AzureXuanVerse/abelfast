package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Tx is a request's unit of work: its queries run in one transaction, and
// what they cause besides storage is collected on it. The inventory
// primitives (SpendResource, AddItem, AddShip, grantDrops...) record the task
// events and pushes they cause themselves; a handler only adds the events
// about the action itself (a build, a win...).
type Tx struct {
	*db.Queries
	ctx         context.Context
	commanderID uint32

	events   []TaskEvent
	after    []func() error
	pushes   []proto.Message      // SC_17002 archive updates... in the order they happened
	newShips []*protobuf.SHIPINFO // ships obtained outside of a reply, pushed in SC_12042

	// newShipsAfterReply sends the SC_12042 after the reply instead of
	// before it (the battle result, as captured).
	newShipsAfterReply bool
}

// tx runs fn in a transaction. Once fn succeeds the task events it recorded
// are applied in the same transaction, then its afterEvents functions run.
// When it commits, the task progress pushes (SC_20002, SC_20102, SC_20103,
// SC_20202) are queued on the client, then the other pushes, then the new
// ships (SC_12042), all ahead of the reply. Nothing is queued when it fails.
func (game *Game) tx(ctx context.Context, client *connection.Client, fn func(tx *Tx) error) error {
	var tx *Tx
	var progress []proto.Message
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		tx = newTx(ctx, queries, client.CommanderID)
		if err := fn(tx); err != nil {
			return err
		}
		var err error
		if progress, err = game.applyTaskEvents(ctx, queries, client.CommanderID, tx.events); err != nil {
			return err
		}
		for _, fn := range tx.after {
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, push := range append(progress, tx.pushes...) {
		if err := client.Push(push); err != nil {
			return err
		}
	}
	if len(tx.newShips) == 0 {
		return nil
	}
	if tx.newShipsAfterReply {
		return client.PushAfterReply(NewShipsPush(tx.newShips...))
	}
	return client.Push(NewShipsPush(tx.newShips...))
}

// newTx is a unit of work on queries for commanderID; what it records is only
// sent when it runs through Game.tx.
func newTx(ctx context.Context, queries *db.Queries, commanderID uint32) *Tx {
	return &Tx{Queries: queries, ctx: ctx, commanderID: commanderID}
}

// event records what the commander did, for the tasks that count it.
func (tx *Tx) event(events ...TaskEvent) {
	tx.events = append(tx.events, events...)
}

// afterEvents runs fn in the transaction once the task events are applied,
// for what must not count them.
func (tx *Tx) afterEvents(fn func() error) {
	tx.after = append(tx.after, fn)
}

// push queues a message to send ahead of the reply once tx commits.
func (tx *Tx) push(msg proto.Message) {
	tx.pushes = append(tx.pushes, msg)
}
