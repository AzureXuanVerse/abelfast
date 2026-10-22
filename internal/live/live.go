// Package live tells the back-office what changed as it happens: the game
// and the back-office publish events (a commander's save written, someone
// logging in or out, a guild or a server area edited), the back-office
// streams them to its open pages (GET /api/live), which refetch what
// they show.
package live

import "sync"

// Kinds of events.
const (
	Commander = "commander" // ID's save changed
	Online    = "online"    // someone logged in or out
	Guild     = "guild"     // guild ID changed
	Area      = "area"      // a server-wide area changed, Name says which
)

// Event is one change.
type Event struct {
	Kind string `json:"kind"`
	ID   uint32 `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Broker hands events to its subscribers. A nil Broker drops them.
type Broker struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// subscriberBuffer is how many events a slow subscriber may fall behind by
// before newer ones are dropped for it (its page refetches on the next).
const subscriberBuffer = 64

// Publish sends event to every subscriber, never waiting on one.
func (broker *Broker) Publish(event Event) {
	if broker == nil {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	for sub := range broker.subs {
		select {
		case sub <- event:
		default:
		}
	}
}

// Subscribe returns the events from now on, and the function that stops
// them.
func (broker *Broker) Subscribe() (<-chan Event, func()) {
	sub := make(chan Event, subscriberBuffer)
	broker.mu.Lock()
	if broker.subs == nil {
		broker.subs = make(map[chan Event]struct{})
	}
	broker.subs[sub] = struct{}{}
	broker.mu.Unlock()
	return sub, func() {
		broker.mu.Lock()
		delete(broker.subs, sub)
		broker.mu.Unlock()
	}
}
