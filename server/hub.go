package server

import (
	"log"

	"messenger/models"
	"messenger/storage"
)

// JoinRequest pairs a client with the room it wants to join.
type JoinRequest struct {
	Client *Client
	RoomID string
}

// Hub manages chat rooms and routes messages to the correct room.
type Hub struct {
	rooms      map[string]*ChatRoom
	store      *storage.MessageStore
	Register   chan JoinRequest
	Unregister chan *Client
	Broadcast  chan models.Message
}

func NewHub(store *storage.MessageStore) *Hub {
	return &Hub{
		rooms:      make(map[string]*ChatRoom),
		store:      store,
		Register:   make(chan JoinRequest),
		Unregister: make(chan *Client),
		Broadcast:  make(chan models.Message),
	}
}

// Run starts the hub event loop. Call it in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case req := <-h.Register:
			room, ok := h.rooms[req.RoomID]
			if !ok {
				room = newChatRoom(req.RoomID)
				h.rooms[req.RoomID] = room
			}
			room.clients[req.Client] = true
			req.Client.roomID = req.RoomID
			// Notify existing members that a new user has joined.
			room.deliver(models.Message{
				Type: "user_join",
				User: req.Client.username,
				Room: req.RoomID,
			}, h)

		case client := <-h.Unregister:
			room, ok := h.rooms[client.roomID]
			if !ok {
				break
			}
			if _, member := room.clients[client]; member {
				delete(room.clients, client)
				close(client.send)
				if len(room.clients) == 0 {
					delete(h.rooms, client.roomID)
				} else {
					// Notify remaining members that the user has left.
					room.deliver(models.Message{
						Type: "user_leave",
						User: client.username,
						Room: client.roomID,
					}, h)
				}
			}

		case msg := <-h.Broadcast:
			if room, ok := h.rooms[msg.Room]; ok {
				room.deliver(msg, h)

				if h.store != nil {
					if err := h.store.SaveMessage(msg.Room, msg.User, msg.Text); err != nil {
						log.Println("failed to save message:", err)
					}
				}
			}
		}
	}
}
