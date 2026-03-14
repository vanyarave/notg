package server

import (
	"log"

	"messenger/models"
	"messenger/storage"
)

// registerReq carries a client joining this room along with their DB user ID.
type registerReq struct {
	client *Client
	userID int64
}

// ChatRoom owns its client set and runs independently in its own goroutine.
// When the last client leaves it calls manager.RemoveChat and exits.
type ChatRoom struct {
	chatID  int64
	store   *storage.MessageStore
	manager *ChatManager

	register   chan registerReq
	unregister chan *Client
	broadcast  chan models.Message
}

func newChatRoom(chatID int64, store *storage.MessageStore, manager *ChatManager) *ChatRoom {
	return &ChatRoom{
		chatID:     chatID,
		store:      store,
		manager:    manager,
		register:   make(chan registerReq, 8),
		unregister: make(chan *Client, 8),
		broadcast:  make(chan models.Message, 256),
	}
}

// Run is the ChatRoom event loop. It is started by ChatManager.GetOrCreateChat.
func (r *ChatRoom) Run() {
	clients := make(map[*Client]bool)

	for {
		select {
		case req := <-r.register:
			clients[req.client] = true
			req.client.chatID = r.chatID
			req.client.userID = req.userID

			// Build the current user list (before the joiner is counted, they
			// will see themselves arrive via the user_join that follows).
			usernames := make([]string, 0, len(clients)-1)
			for c := range clients {
				if c != req.client {
					usernames = append(usernames, c.username)
				}
			}

			// Send the snapshot only to the newly joined client.
			select {
			case req.client.send <- models.Message{
				Type:   "users_list",
				ChatID: r.chatID,
				Users:  usernames,
			}:
			default:
			}

			// Announce the arrival to everyone in the room (including the joiner).
			r.fan(clients, models.Message{
				Type:   "user_join",
				ChatID: r.chatID,
				User:   req.client.username,
			})

		case client := <-r.unregister:
			if !clients[client] {
				break
			}
			delete(clients, client)
			close(client.send)
			if len(clients) == 0 {
				r.manager.RemoveChat(r.chatID)
				log.Println("chat closed:", r.chatID)
				return
			}
			r.fan(clients, models.Message{
				Type:   "user_leave",
				ChatID: r.chatID,
				User:   client.username,
			})

		case msg := <-r.broadcast:
			if r.store != nil {
				if err := r.store.SaveMessage(msg.ChatID, msg.UserID, msg.Text); err != nil {
					log.Println("failed to save message:", err)
				}
			}
			r.fan(clients, msg)
		}
	}
}

// fan delivers msg to every client, dropping slow ones.
func (r *ChatRoom) fan(clients map[*Client]bool, msg models.Message) {
	for client := range clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(clients, client)
		}
	}
}
