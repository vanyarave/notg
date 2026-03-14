package models

type Message struct {
	Type   string   `json:"type"`
	ChatID int64    `json:"chat_id,omitempty"`
	Room   string   `json:"room,omitempty"` // kept for client compatibility
	User   string   `json:"user,omitempty"`
	Users  []string `json:"users,omitempty"` // used by users_list event
	Text   string   `json:"text,omitempty"`

	// UserID is used internally for persistence; it is not sent to clients.
	UserID int64 `json:"-"`
}
