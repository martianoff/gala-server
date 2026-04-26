package tui

import (
	"sync"
	"time"
)

// Event captures the outcome of a single HTTP request for dashboard display.
type Event struct {
	Method    string
	Path      string
	Status    int
	LatencyMs int64
	At        time.Time
}

// NewEvent constructs an Event timestamped now.
func NewEvent(method, path string, status int, latencyMs int64) Event {
	return Event{Method: method, Path: path, Status: status, LatencyMs: latencyMs, At: time.Now()}
}

// Bus is a thread-safe queue carrying request events from the HTTP server's
// goroutines to the TUI's main goroutine.
type Bus struct {
	mu     sync.Mutex
	events []Event
}

// NewBus returns an empty bus.
func NewBus() *Bus {
	return &Bus{events: make([]Event, 0, 256)}
}

// Push enqueues an event. Safe from any goroutine.
func (b *Bus) Push(e Event) {
	b.mu.Lock()
	b.events = append(b.events, e)
	b.mu.Unlock()
}

// Drain atomically removes and returns all queued events.
func (b *Bus) Drain() []Event {
	b.mu.Lock()
	out := b.events
	b.events = make([]Event, 0, 256)
	b.mu.Unlock()
	return out
}
