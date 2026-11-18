// Package sse implements Server-Sent Events for live counter updates.
package sse

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event is a single SSE message.
type Event struct {
	// Name is the event type (e.g., "counter", "status").
	Name string
	// Data is the JSON payload.
	Data []byte
}

// Broker manages a set of subscribers and broadcasts events to them.
type Broker struct {
	mu       sync.RWMutex
	subs     map[chan Event]struct{}
	closed   bool
}

// NewBroker creates a new SSE broker.
func NewBroker() *Broker {
	return &Broker{
		subs: make(map[chan Event]struct{}),
	}
}

// Subscribe returns a channel that receives events.
// The caller must call Unsubscribe when done.
func (b *Broker) Subscribe() chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 16)
	b.subs[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Broker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
}

// Broadcast sends an event to all subscribers.
// If a subscriber's buffer is full, the event is dropped for that subscriber.
func (b *Broker) Broadcast(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Drop if buffer full
		}
	}
}

// SubscriberCount returns the number of active subscribers.
func (b *Broker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// Close shuts down the broker and closes all subscriber channels.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
}

// Handler returns an HTTP handler that streams SSE events.
// The handler runs until the context is cancelled or the client disconnects.
func (b *Broker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := b.Subscribe()
		defer b.Unsubscribe(ch)

		// Send initial keepalive
		fmt.Fprintf(w, ": keepalive\n\n")
		flusher.Flush()

		ctx := r.Context()
		keepalive := time.NewTicker(30 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if ev.Name != "" {
					fmt.Fprintf(w, "event: %s\n", ev.Name)
				}
				fmt.Fprintf(w, "data: %s\n\n", ev.Data)
				flusher.Flush()
			case <-keepalive.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			}
		}
	}
}

// Publisher periodically broadcasts events from a producer function.
type Publisher struct {
	broker   *Broker
	interval time.Duration
	produce  func(ctx context.Context) (Event, error)
}

// NewPublisher creates a publisher that calls produce every interval.
func NewPublisher(broker *Broker, interval time.Duration, produce func(ctx context.Context) (Event, error)) *Publisher {
	return &Publisher{
		broker:   broker,
		interval: interval,
		produce:  produce,
	}
}

// Run starts the publisher loop. It blocks until ctx is cancelled.
func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ev, err := p.produce(ctx)
			if err != nil {
				continue
			}
			p.broker.Broadcast(ev)
		}
	}
}
