package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"messenger/models"
	"messenger/storage"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now; tighten this in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWs upgrades the HTTP connection to WebSocket, performs the join
// handshake, replays recent history, then hands the connection off to the hub.
//
// The client must send a join message as its very first frame:
//
//	{"type":"join","room":"room_name","user":"username"}
func ServeWs(hub *Hub, store *storage.MessageStore, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	// Give the client a short window to send the join frame.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	_, raw, err := conn.ReadMessage()
	if err != nil {
		log.Printf("join read error: %v", err)
		conn.Close()
		return
	}

	// Reset the deadline; ReadPump will set its own.
	conn.SetReadDeadline(time.Time{})

	var join models.Message
	if err := json.Unmarshal(raw, &join); err != nil || join.Type != "join" || join.Room == "" || join.User == "" {
		log.Printf("invalid join message from %s", r.RemoteAddr)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "first message must be a valid join"))
		conn.Close()
		return
	}

	// Replay the last 50 messages before the client enters live message flow.
	history, err := store.GetRecentMessages(join.Room, 50)
	if err != nil {
		log.Printf("history load error: %v", err)
	}
	for _, msg := range history {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("history send error: %v", err)
			conn.Close()
			return
		}
	}

	client := &Client{
		hub:      hub,
		username: join.User,
		conn:     conn,
		send:     make(chan models.Message, 256),
	}
	hub.Register <- JoinRequest{Client: client, RoomID: join.Room}

	go client.WritePump()
	go client.ReadPump()
}
