package server

import "messenger/models"

// ChatRoom holds the clients that belong to a single room and its
// dedicated broadcast channel.
type ChatRoom struct {
	id        string
	clients   map[*Client]bool
	broadcast chan models.Message
}

func newChatRoom(id string) *ChatRoom {
	return &ChatRoom{
		id:        id,
		clients:   make(map[*Client]bool),
		broadcast: make(chan models.Message, 256),
	}
}

// deliver fans a message out to every client in the room.
// Must be called from the hub's goroutine so map access is safe.
func (r *ChatRoom) deliver(msg models.Message, hub *Hub) {
	for client := range r.clients {
		select {
		case client.send <- msg:
		default:
			// Slow client: drop and clean up.
			close(client.send)
			delete(r.clients, client)
		}
	}
}
