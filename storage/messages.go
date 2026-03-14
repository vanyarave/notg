package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"messenger/models"
)

// MessageStore persists users, chats, and messages in a single SQLite database.
// The *sql.DB connection is opened once at startup and reused for every call.
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore opens (or creates) the SQLite file at path and ensures the
// schema is up to date.
func NewMessageStore(path string) (*MessageStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &MessageStore{db: db}
	return s, s.init()
}

// init creates all tables and indexes. Each statement is executed separately
// because the SQLite driver does not support multi-statement Exec calls.
func (s *MessageStore) init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id         INTEGER  PRIMARY KEY AUTOINCREMENT,
			username   TEXT     NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS chats (
			id         INTEGER  PRIMARY KEY AUTOINCREMENT,
			type       TEXT     NOT NULL DEFAULT 'group',
			name       TEXT     NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS chat_members (
			chat_id   INTEGER  NOT NULL,
			user_id   INTEGER  NOT NULL,
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (chat_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER  PRIMARY KEY AUTOINCREMENT,
			chat_id    INTEGER  NOT NULL,
			user_id    INTEGER  NOT NULL,
			text       TEXT     NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages (chat_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT     PRIMARY KEY,
			user_id    INTEGER  NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

// --- Users ---

// CreateUser inserts a new user and returns its ID.
// If the username already exists the existing row's ID is returned instead.
func (s *MessageStore) CreateUser(username string) (int64, error) {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO users (username) VALUES (?)`, username,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateUser: %w", err)
	}
	return s.GetUserByUsername(username)
}

// GetUserByUsername returns the ID of the user with the given username.
func (s *MessageStore) GetUserByUsername(username string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("GetUserByUsername %q: %w", username, err)
	}
	return id, nil
}

// --- Chats ---

// CreateChat inserts a new chat (identified by name) and returns its ID.
// If a chat with that name already exists its ID is returned.
func (s *MessageStore) CreateChat(name, chatType string) (int64, error) {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO chats (name, type) VALUES (?, ?)`, name, chatType,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateChat: %w", err)
	}
	var id int64
	err = s.db.QueryRow(`SELECT id FROM chats WHERE name = ?`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("GetChat %q: %w", name, err)
	}
	return id, nil
}

// AddUserToChat records that a user is a member of a chat.
// Duplicate memberships are silently ignored.
func (s *MessageStore) AddUserToChat(chatID, userID int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO chat_members (chat_id, user_id) VALUES (?, ?)`,
		chatID, userID,
	)
	if err != nil {
		return fmt.Errorf("AddUserToChat: %w", err)
	}
	return nil
}

// --- Messages ---

// SaveMessage persists a chat message and returns any database error.
func (s *MessageStore) SaveMessage(chatID, userID int64, text string) error {
	_, err := s.db.Exec(
		`INSERT INTO messages (chat_id, user_id, text) VALUES (?, ?, ?)`,
		chatID, userID, text,
	)
	if err != nil {
		return fmt.Errorf("SaveMessage: %w", err)
	}
	return nil
}

// GetRecentMessages returns up to limit messages for the given chat, ordered
// oldest-first so clients can replay them in chronological order.
func (s *MessageStore) GetRecentMessages(chatID int64, limit int) ([]models.Message, error) {
	rows, err := s.db.Query(`
		SELECT u.username, m.text FROM (
			SELECT user_id, text, created_at
			FROM   messages
			WHERE  chat_id = ?
			ORDER  BY created_at DESC
			LIMIT  ?
		) m
		JOIN users u ON u.id = m.user_id
		ORDER BY m.created_at ASC
	`, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetRecentMessages: %w", err)
	}
	defer rows.Close()

	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.User, &m.Text); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Type = "chat"
		m.ChatID = chatID
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// --- Sessions ---

// Session holds the data resolved from a valid session token.
type Session struct {
	Token    string
	UserID   int64
	Username string
}

// CreateSession generates a cryptographically random token, persists it, and
// returns the token string.
func (s *MessageStore) CreateSession(userID int64) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("CreateSession rand: %w", err)
	}
	token := hex.EncodeToString(buf)

	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id) VALUES (?, ?)`, token, userID,
	)
	if err != nil {
		return "", fmt.Errorf("CreateSession insert: %w", err)
	}
	return token, nil
}

// GetSession looks up a token and returns the associated Session (including
// the username). Returns sql.ErrNoRows if the token does not exist.
func (s *MessageStore) GetSession(token string) (*Session, error) {
	row := s.db.QueryRow(`
		SELECT s.token, s.user_id, u.username
		FROM   sessions s
		JOIN   users u ON u.id = s.user_id
		WHERE  s.token = ?
	`, token)

	var sess Session
	if err := row.Scan(&sess.Token, &sess.UserID, &sess.Username); err != nil {
		return nil, err // callers check for sql.ErrNoRows
	}
	return &sess, nil
}

// Close releases the database connection.
func (s *MessageStore) Close() error {
	return s.db.Close()
}
