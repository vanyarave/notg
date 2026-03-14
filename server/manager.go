package server

import (
	"sync"

	"messenger/storage"
)

// ChatManager owns the lifecycle of all active ChatRooms.
// It is safe for concurrent use; internal access is protected by a mutex.
type ChatManager struct {
	mu    sync.Mutex
	chats map[int64]*ChatRoom
	store *storage.MessageStore
}

func NewChatManager(store *storage.MessageStore) *ChatManager {
	return &ChatManager{
		chats: make(map[int64]*ChatRoom),
		store: store,
	}
}

// GetOrCreateChat returns the existing ChatRoom for chatID, or creates and
// starts a new one. Safe to call from multiple goroutines concurrently.
func (m *ChatManager) GetOrCreateChat(chatID int64) *ChatRoom {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, ok := m.chats[chatID]; ok {
		return room
	}

	room := newChatRoom(chatID, m.store, m)
	m.chats[chatID] = room
	go room.Run()
	return room
}

// RemoveChat deletes the entry for chatID. Called by a ChatRoom when it
// becomes empty and its goroutine is about to exit.
func (m *ChatManager) RemoveChat(chatID int64) {
	m.mu.Lock()
	delete(m.chats, chatID)
	m.mu.Unlock()
}
