package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	serverURL  = "ws://localhost:8080/ws"
	numClients = 50
	room       = "test"
	duration   = 20 * time.Second
	msgInterval = time.Second
)

type message struct {
	Type string `json:"type"`
	Room string `json:"room,omitempty"`
	User string `json:"user,omitempty"`
	Text string `json:"text,omitempty"`
}

func runClient(id int, wg *sync.WaitGroup, connected, errors *atomic.Int64) {
	defer wg.Done()

	username := fmt.Sprintf("bot%d", id)

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		errors.Add(1)
		log.Printf("[client %d] connect error: %v", id, err)
		return
	}
	defer conn.Close()

	// Send join frame.
	join, _ := json.Marshal(message{Type: "join", Room: room, User: username})
	if err := conn.WriteMessage(websocket.TextMessage, join); err != nil {
		errors.Add(1)
		log.Printf("[client %d] join error: %v", id, err)
		return
	}

	connected.Add(1)

	// Drain incoming messages in the background so the server never blocks.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(msgInterval)
	defer ticker.Stop()
	deadline := time.After(duration)

	for {
		select {
		case <-deadline:
			return
		case t := <-ticker.C:
			payload, _ := json.Marshal(message{
				Type: "chat",
				Text: fmt.Sprintf("hello from %s at %s", username, t.Format(time.RFC3339)),
			})
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				errors.Add(1)
				return
			}
		case <-done:
			return
		}
	}
}

func main() {
	var (
		wg        sync.WaitGroup
		connected atomic.Int64
		errors    atomic.Int64
	)

	log.Printf("starting %d clients against %s (room=%q, duration=%s)", numClients, serverURL, room, duration)

	for i := 1; i <= numClients; i++ {
		wg.Add(1)
		go runClient(i, &wg, &connected, &errors)
		// Small stagger to avoid thundering-herd at startup.
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()

	fmt.Printf("\n--- load test complete ---\n")
	fmt.Printf("connected clients : %d\n", connected.Load())
	fmt.Printf("errors            : %d\n", errors.Load())
	fmt.Printf("done\n")
}
