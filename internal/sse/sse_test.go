package sse

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrokerSubscribeUnsubscribe(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	ch := b.Subscribe()
	if b.SubscriberCount() != 1 {
		t.Errorf("SubscriberCount = %d, want 1", b.SubscriberCount())
	}

	b.Unsubscribe(ch)
	if b.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount = %d after unsubscribe, want 0", b.SubscriberCount())
	}
}

func TestBrokerBroadcast(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	ev := Event{Name: "test", Data: []byte(`{"hello":"world"}`)}
	b.Broadcast(ev)

	// Both subscribers should receive the event
	select {
	case got := <-ch1:
		if got.Name != ev.Name || string(got.Data) != string(ev.Data) {
			t.Errorf("ch1 got wrong event: %+v", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch1 did not receive event")
	}

	select {
	case got := <-ch2:
		if got.Name != ev.Name || string(got.Data) != string(ev.Data) {
			t.Errorf("ch2 got wrong event: %+v", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch2 did not receive event")
	}
}

func TestBrokerBroadcastDropsOnFullBuffer(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	ch := b.Subscribe()
	// Fill the buffer (size 16)
	for i := 0; i < 20; i++ {
		b.Broadcast(Event{Name: "fill", Data: []byte(`{}`)})
	}

	// Drain and count received events
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count > 16 {
		t.Errorf("received %d events, want <= 16 (buffer size)", count)
	}
}

func TestBrokerClose(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()

	b.Close()

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel not closed after Broker.Close()")
	}

	// Double close should be safe
	b.Close()
}

func TestBrokerHandler(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	handler := b.Handler()
	req := httptest.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()

	// Run handler in background
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	go handler(w, req)

	// Give handler time to start
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	ev := Event{Name: "counter", Data: []byte(`{"value":42}`)}
	b.Broadcast(ev)

	// Give event time to be written
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop handler
	cancel()
	time.Sleep(10 * time.Millisecond)

	// Check response
	resp := w.Result()
	body := w.Body.String()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	if !strings.Contains(body, "event: counter") {
		t.Errorf("body does not contain event name: %q", body)
	}
	if !strings.Contains(body, `data: {"value":42}`) {
		t.Errorf("body does not contain event data: %q", body)
	}
}

func TestPublisher(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	callCount := 0
	produce := func(ctx context.Context) (Event, error) {
		callCount++
		data, _ := json.Marshal(map[string]int{"count": callCount})
		return Event{Name: "tick", Data: data}, nil
	}

	pub := NewPublisher(b, 20*time.Millisecond, produce)

	ctx, cancel := context.WithCancel(context.Background())
	go pub.Run(ctx)

	// Subscribe to receive events
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Wait for at least 2 ticks
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should have received at least 2 events
	received := 0
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				goto done
			}
			if ev.Name == "tick" {
				received++
			}
		case <-time.After(100 * time.Millisecond):
			goto done
		}
	}
done:
	if received < 2 {
		t.Errorf("received %d events, want >= 2", received)
	}
}

func TestPublisherErrorHandling(t *testing.T) {
	b := NewBroker()
	defer b.Close()

	produce := func(ctx context.Context) (Event, error) {
		return Event{}, context.DeadlineExceeded
	}

	pub := NewPublisher(b, 10*time.Millisecond, produce)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should not panic or block
	go pub.Run(ctx)
	<-ctx.Done()
}
