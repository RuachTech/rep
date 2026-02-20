package hotreload

import (
	"bufio"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHub_BroadcastToClients(t *testing.T) {
	hub := NewHub(slog.Default())

	ch1, unsub1 := hub.subscribe()
	defer unsub1()
	ch2, unsub2 := hub.subscribe()
	defer unsub2()

	event := Event{Type: "rep:config:update", Key: "API_URL", Tier: "public", Value: "new-value"}
	hub.Broadcast(event)

	select {
	case e := <-ch1:
		if e.Key != "API_URL" {
			t.Errorf("client 1: expected key API_URL, got %s", e.Key)
		}
	case <-time.After(time.Second):
		t.Fatal("client 1: timed out waiting for event")
	}

	select {
	case e := <-ch2:
		if e.Key != "API_URL" {
			t.Errorf("client 2: expected key API_URL, got %s", e.Key)
		}
	case <-time.After(time.Second):
		t.Fatal("client 2: timed out waiting for event")
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub(slog.Default())

	ch, unsub := hub.subscribe()
	unsub()

	// Channel should be closed after unsubscribe.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestHub_ClientCount(t *testing.T) {
	hub := NewHub(slog.Default())

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}

	_, unsub1 := hub.subscribe()
	_, unsub2 := hub.subscribe()
	_, unsub3 := hub.subscribe()

	if hub.ClientCount() != 3 {
		t.Errorf("expected 3 clients, got %d", hub.ClientCount())
	}

	unsub1()
	if hub.ClientCount() != 2 {
		t.Errorf("expected 2 clients after unsubscribe, got %d", hub.ClientCount())
	}

	unsub2()
	unsub3()
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after all unsubscribed, got %d", hub.ClientCount())
	}
}

func TestHub_SlowClient(t *testing.T) {
	hub := NewHub(slog.Default())

	ch, unsub := hub.subscribe()
	defer unsub()

	// Fill the buffer (16 events).
	for i := 0; i < 16; i++ {
		hub.Broadcast(Event{Type: "rep:config:update", Key: "KEY"})
	}

	// 17th should not block (dropped for slow client).
	done := make(chan struct{})
	go func() {
		hub.Broadcast(Event{Type: "rep:config:update", Key: "OVERFLOW"})
		close(done)
	}()

	select {
	case <-done:
		// OK — broadcast did not block.
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked on slow client")
	}

	// Drain the buffer.
	for range ch {
		if len(ch) == 0 {
			break
		}
	}
}

func TestSSEHandler_Headers(t *testing.T) {
	hub := NewHub(slog.Default())
	h := NewHandler(hub)

	// Use a server so we get proper flushing support.
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/rep/changes")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected no-cache, got %q", cc)
	}
}

func TestSSEHandler_ReceivesEvent(t *testing.T) {
	hub := NewHub(slog.Default())
	h := NewHandler(hub)

	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/rep/changes")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	// Read the initial comment (": connected to REP hot reload").
	if !scanner.Scan() {
		t.Fatal("expected initial comment line")
	}
	initial := scanner.Text()
	if !strings.HasPrefix(initial, ": connected") {
		t.Errorf("expected connection comment, got %q", initial)
	}

	// Broadcast an event.
	hub.Broadcast(Event{
		Type:  "rep:config:update",
		Key:   "TEST_KEY",
		Tier:  "public",
		Value: "new_value",
	})

	// Read the event lines.
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Blank line separates initial comment from events, or events from each other.
			if len(lines) > 0 {
				break
			}
			continue
		}
		lines = append(lines, line)
	}

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 SSE lines (event + data), got %d: %v", len(lines), lines)
	}

	foundEvent := false
	foundData := false
	for _, line := range lines {
		if strings.HasPrefix(line, "event: rep:config:update") {
			foundEvent = true
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, "TEST_KEY") {
			foundData = true
		}
	}

	if !foundEvent {
		t.Error("expected event: line in SSE output")
	}
	if !foundData {
		t.Error("expected data: line with TEST_KEY in SSE output")
	}
}
