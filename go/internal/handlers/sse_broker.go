package handlers

import (
	"net/http"
	"sync"
)

type SSEBroker struct {
	mu          sync.RWMutex
	subscribers map[string]chan struct{}
}

func NewSSEBroker() *SSEBroker {
	return &SSEBroker{subscribers: make(map[string]chan struct{})}
}

func (b *SSEBroker) Broadcast() {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	id := r.RemoteAddr
	ch := make(chan struct{}, 1)

	b.mu.Lock()
	b.subscribers[id] = ch
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		b.mu.Unlock()
		close(ch)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	w.Write([]byte("event: connected\ndata: ok\n\n"))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			w.Write([]byte("data: update\n\n"))
			flusher.Flush()
		}
	}
}
