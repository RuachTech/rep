// Package hotreload provides the /rep/changes SSE endpoint.
//
// Per REP-RFC-0001 §4.6, this endpoint streams configuration deltas
// to connected clients when environment variables change.
package hotreload

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Event represents a configuration change event.
type Event struct {
	Type  string // "rep:config:update" or "rep:config:delete"
	Key   string
	Tier  string
	Value string // Empty for delete events.
}

// Hub manages SSE client connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
	logger  *slog.Logger
}

// NewHub creates a new hot reload hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[chan Event]struct{}),
		logger:  logger,
	}
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			// Client is slow — drop the event to avoid blocking.
			h.logger.Warn("rep.hotreload.client_slow", "event_key", event.Key)
		}
	}

	h.logger.Info("rep.config.changed",
		"key", event.Key,
		"tier", event.Tier,
		"action", event.Type,
		"clients_notified", len(h.clients),
	)
}

// ClientCount returns the number of connected SSE clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// subscribe registers a new client channel and returns an unsubscribe function.
func (h *Hub) subscribe() (chan Event, func()) {
	ch := make(chan Event, 16) // Buffered to handle bursts.

	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}

	return ch, unsub
}

// Handler serves the GET /rep/changes SSE endpoint.
type Handler struct {
	hub *Hub
}

// NewHandler creates a new SSE handler backed by the given hub.
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// ServeHTTP handles SSE connections.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify the client supports SSE.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering.

	// Subscribe to events.
	ch, unsub := h.hub.subscribe()
	defer unsub()

	// Send initial ping to confirm connection.
	fmt.Fprintf(w, ": connected to REP hot reload\n\n")
	flusher.Flush()

	// Keep-alive ticker.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return // Channel closed.
			}

			data, _ := json.Marshal(map[string]string{
				"key":   event.Key,
				"tier":  event.Tier,
				"value": event.Value,
			})

			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n", string(data))
			fmt.Fprintf(w, "id: %d\n\n", time.Now().UnixMilli())
			flusher.Flush()

		case <-ticker.C:
			// Keep-alive comment to prevent proxy timeouts.
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return // Client disconnected.
		}
	}
}
