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

// Client is a middleman between the WebSocket connection and its ChatRoom.
type Client struct {
	room     *ChatRoom // set after the JoinRequest is processed
	chatID   int64
	userID   int64
	username string
	conn     *websocket.Conn

	// Buffered channel of outbound messages.
	send chan models.Message
}

// ReadPump pumps messages from the WebSocket connection to the room.
func (c *Client) ReadPump() {
	defer func() {
		// Route unregister to the room, not the hub.
		if c.room != nil {
			c.room.unregister <- c
		}
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

		// Enforce identity fields so clients cannot spoof them.
		msg.ChatID = c.chatID
		msg.UserID = c.userID
		msg.User = c.username

		if c.room != nil {
			c.room.broadcast <- msg
		}
	}
}

// WritePump pumps messages from the room to the WebSocket connection.
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
