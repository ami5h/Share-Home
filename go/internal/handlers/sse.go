package handlers

import (
	"log"
)

// DefaultBroker is the global SSE broker. Set it in main.go before starting the server.
var DefaultBroker *SSEBroker

// Broadcast triggers an SSE event to all connected clients.
func Broadcast() {
	if DefaultBroker == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("SSE broadcast: %v", r)
			}
		}()
		DefaultBroker.Broadcast()
	}()
}
