package logger

import (
	"sync"
	"time"
)

// recentSize bounds the events kept in memory for the back-office.
const recentSize = 2000

// Event is a logged line, kept in memory (see Recent).
type Event struct {
	Seq     uint64
	Time    time.Time
	Level   int
	Scope   string
	Fields  []Field
	Message string
}

// LevelName is the level's label, as printed.
func LevelName(level int) string { return levelName(level) }

// recent is a ring of the last recentSize events logged, and the functions
// watching every event.
var recent struct {
	mu       sync.Mutex
	events   [recentSize]Event
	next     uint64 // Seq of the next event, events[(Seq-1) % recentSize]
	watchers []func(Event)
}

func remember(scope, message string, level int, fields []Field) {
	recent.mu.Lock()
	recent.next++
	event := Event{Seq: recent.next, Time: time.Now(), Level: level, Scope: scope, Fields: fields, Message: message}
	recent.events[(event.Seq-1)%recentSize] = event
	watchers := recent.watchers
	recent.mu.Unlock()
	for _, fn := range watchers {
		fn(event)
	}
}

// Recent returns the events kept that were logged after the one numbered
// after (0 for all of them), oldest first.
func Recent(after uint64) []Event {
	recent.mu.Lock()
	defer recent.mu.Unlock()
	first := uint64(1)
	if recent.next > recentSize {
		first = recent.next - recentSize + 1
	}
	if after+1 > first {
		first = after + 1
	}
	var events []Event
	if recent.next >= first {
		events = make([]Event, 0, recent.next-first+1)
	}
	for seq := first; seq <= recent.next; seq++ {
		events = append(events, recent.events[(seq-1)%recentSize])
	}
	return events
}

// Watch calls fn with every event logged from now on, after it is kept. fn
// runs on the logging goroutine and must not log.
func Watch(fn func(Event)) {
	recent.mu.Lock()
	defer recent.mu.Unlock()
	recent.watchers = append(recent.watchers[:len(recent.watchers):len(recent.watchers)], fn)
}
