package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"messenger/models"
)

// MessageStore persists chat messages using a single, long-lived *sql.DB
// connection opened once at startup.
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore opens (or creates) the SQLite file at path, verifies the
// connection, and ensures the schema is up to date.
func NewMessageStore(path string) (*MessageStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	store := &MessageStore{db: db}
	if err := store.init(); err != nil {
		return nil, err
	}
	return store, nil
}

// init creates the schema if it does not already exist.
// Each statement is executed separately because the SQLite driver does not
// support multiple statements in a single Exec call.
func (s *MessageStore) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER  PRIMARY KEY AUTOINCREMENT,
			room       TEXT     NOT NULL,
			user       TEXT     NOT NULL,
			text       TEXT     NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_room ON messages (room, created_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

// SaveMessage persists a single chat message using the shared db connection.
func (s *MessageStore) SaveMessage(room, user, text string) error {
	_, err := s.db.Exec(
		`INSERT INTO messages (room, user, text) VALUES (?, ?, ?)`,
		room, user, text,
	)
	if err != nil {
		return fmt.Errorf("SaveMessage: %w", err)
	}
	return nil
}

// GetRecentMessages returns up to limit messages for room, ordered
// oldest-first (chronological) so clients can replay them in order.
//
// The inner query fetches the newest `limit` rows; the outer query
// re-orders them ascending so the caller always receives history in
// the correct chronological sequence.
func (s *MessageStore) GetRecentMessages(room string, limit int) ([]models.Message, error) {
	rows, err := s.db.Query(`
		SELECT user, text FROM (
			SELECT user, text, created_at
			FROM   messages
			WHERE  room = ?
			ORDER  BY created_at DESC
			LIMIT  ?
		)
		ORDER BY created_at ASC
	`, room, limit)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.User, &m.Text); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Type = "chat"
		m.Room = room
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// Close releases the database connection.
func (s *MessageStore) Close() error {
	return s.db.Close()
}
