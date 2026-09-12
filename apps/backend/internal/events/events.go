// Package events is the change notification bus. The store publishes what it
// changed; transports that hold a connection open — SSE today — forward it.
//
// It knows nothing about HTTP or the store's types: an event names what moved,
// and the client re-reads what it cares about. Shipping the changed memory
// itself would mean every subscriber pays for content it may not have open.
package events

import (
	"sync"
	"time"
)

// Kind is what happened.
type Kind string

const (
	MemoryWritten   Kind = "memory.written"
	MemoryDeleted   Kind = "memory.deleted"
	ProjectWritten  Kind = "project.written"
	ProjectDeleted  Kind = "project.deleted"
	IndexReconciled Kind = "index.reconciled"
	SettingsChanged Kind = "settings.changed"
)

// Event is one change. Project and Memory are empty when the kind does not
// scope to one.
type Event struct {
	Kind    Kind      `json:"kind"`
	Project string    `json:"project,omitempty"`
	Memory  string    `json:"memory,omitempty"`
	Count   int       `json:"count,omitempty"`
	At      time.Time `json:"at"`
}

// buffer is how far behind a subscriber may fall before events are dropped.
//
// ponytail: a slow subscriber loses events rather than blocking the writer. A
// dropped event costs a stale row until the next one arrives; blocking a write
// on a wedged GUI socket would cost the write. If lossless delivery is ever
// needed, add a sequence number and let the client ask for a resync.
const buffer = 64

// Bus fans one publisher out to many subscribers.
type Bus struct {
	mu   sync.Mutex
	next int
	subs map[int]chan Event
}

// NewBus returns an empty bus. Publishing with no subscribers is a no-op, so
// the store never has to check whether anything is listening.
func NewBus() *Bus {
	return &Bus{subs: map[int]chan Event{}}
}

// Subscribe returns a channel of events and a function that stops the
// subscription and closes the channel. Cancel is safe to call twice.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	if b == nil {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}

	b.mu.Lock()
	id := b.next
	b.next++
	ch := make(chan Event, buffer)
	b.subs[id] = ch
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, id)
			b.mu.Unlock()
			close(ch)
		})
	}
}

// Publish delivers e to every subscriber. It never blocks.
func (b *Bus) Publish(e Event) {
	if b == nil {
		return
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
