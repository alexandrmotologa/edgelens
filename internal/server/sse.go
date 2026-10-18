package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/alexandrmotologa/edgelens/pkg/explainer"
)

// SSEBroker coordinates real-time streaming of proxy transactions to web clients.
type SSEBroker struct {
	mu      sync.Mutex
	clients map[chan []byte]bool
}

// NewSSEBroker initializes the broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan []byte]bool),
	}
}

// Subscribe adds an SSE client.
func (b *SSEBroker) Subscribe() chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan []byte, 100)
	b.clients[ch] = true
	return ch
}

// Unsubscribe removes an SSE client.
func (b *SSEBroker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.clients[ch] {
		delete(b.clients, ch)
		close(ch)
	}
}

// Broadcast sends an event to all connected clients.
func (b *SSEBroker) Broadcast(eventType string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data))

	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			// Non-blocking drop if client is saturated
		}
	}
}

// BroadcastExplanation sends a transaction update.
func (b *SSEBroker) BroadcastExplanation(exp *explainer.Explanation) {
	b.Broadcast("traffic", exp)
}

// HandleSSE serves the persistent HTTP SSE stream.
func (b *SSEBroker) HandleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Send initial connection event
	_, _ = w.Write([]byte("event: connected\ndata: {\"status\":\"connected\"}\n\n"))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		}
	}
}
