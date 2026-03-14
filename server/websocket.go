package server

import (
	"database/sql"
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

// ServeWs upgrades the HTTP connection to WebSocket.
//
// Authentication:
//
//	ws://localhost:8080/ws?token=SESSION_TOKEN
//
// The token must have been obtained via POST /login. The connection is
// rejected with 401 if the token is missing or unknown.
//
// Join frame (client → server, sent after upgrade):
//
//	{"type":"join","room":"general"}
//
// The "user" field in the join frame is ignored; identity comes from the
// session resolved by the token.
func ServeWs(hub *Hub, store *storage.MessageStore, w http.ResponseWriter, r *http.Request) {
	// --- Authenticate before upgrading ---
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	sess, err := store.GetSession(token)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "invalid token", http.StatusUnauthorized)
		} else {
			log.Printf("GetSession error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	// --- Upgrade ---
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	// --- Read join frame ---
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		log.Printf("join read error: %v", err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})

	var join models.Message
	if err := json.Unmarshal(raw, &join); err != nil || join.Type != "join" || join.Room == "" {
		log.Printf("invalid join message from %s", r.RemoteAddr)
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "first message must be {type:join, room:...}"))
		conn.Close()
		return
	}

	// --- Resolve chat ---
	chatID, err := store.CreateChat(join.Room, "group")
	if err != nil {
		log.Printf("CreateChat error: %v", err)
		conn.Close()
		return
	}

	if err := store.AddUserToChat(chatID, sess.UserID); err != nil {
		log.Printf("AddUserToChat error: %v", err)
		conn.Close()
		return
	}

	// --- Replay history ---
	history, err := store.GetRecentMessages(chatID, 50)
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

	// --- Register client ---
	// Identity comes exclusively from the validated session; the join frame's
	// "user" field is deliberately ignored.
	client := &Client{
		username: sess.Username,
		conn:     conn,
		send:     make(chan models.Message, 256),
	}

	roomCh := make(chan *ChatRoom, 1)
	hub.Register <- JoinRequest{Client: client, ChatID: chatID, UserID: sess.UserID, roomCh: roomCh}
	client.room = <-roomCh

	go client.WritePump()
	go client.ReadPump()
}
