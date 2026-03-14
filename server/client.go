package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"messenger/models"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client is a middleman between the WebSocket connection and the hub.
type Client struct {
	hub      *Hub
	roomID   string
	username string
	conn     *websocket.Conn

	// Buffered channel of outbound messages.
	send chan models.Message
}

// ReadPump pumps messages from the WebSocket connection to the hub.
// Each connection runs its own ReadPump in a goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("read error: %v", err)
			}
			break
		}

		var msg models.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("invalid message: %v", err)
			continue
		}

		// Enforce room and username so clients cannot spoof either field.
		msg.Room = c.roomID
		msg.User = c.username
		c.hub.Broadcast <- msg
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
// Each connection runs its own WritePump in a goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
