package server

// JoinRequest carries everything needed to place a client into a chat room.
// The hub replies on roomCh with a pointer to the assigned ChatRoom.
type JoinRequest struct {
	Client *Client
	ChatID int64
	UserID int64
	roomCh chan *ChatRoom
}

// Hub is a lightweight router. It receives WebSocket join events, delegates
// chat lifecycle to ChatManager, and forwards clients to the right ChatRoom.
// It holds no state of its own — all room state lives in ChatManager.
type Hub struct {
	manager  *ChatManager
	Register chan JoinRequest
}

func NewHub(manager *ChatManager) *Hub {
	return &Hub{
		manager:  manager,
		Register: make(chan JoinRequest),
	}
}

// Run is the Hub event loop. Call it in a goroutine.
func (h *Hub) Run() {
	for req := range h.Register {
		room := h.manager.GetOrCreateChat(req.ChatID)
		room.register <- registerReq{client: req.Client, userID: req.UserID}
		req.roomCh <- room
	}
}
