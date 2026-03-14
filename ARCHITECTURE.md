Messenger Architecture

Private realtime messenger backend.

Stack
	•	Go
	•	WebSocket
	•	SQLite

The system is designed as a realtime chat backend capable of handling persistent chat history and multiple concurrent chat rooms.

⸻

Core Architecture

The server uses a layered realtime architecture.

Client
↓
WebSocket
↓
Hub (event router)
↓
ChatManager (chat lifecycle)
↓
ChatRoom goroutines
↓
Clients

Message persistence is handled by SQLite.

⸻

Components

Hub

Hub acts as a lightweight event router.

Responsibilities:
	•	route join requests
	•	route chat messages
	•	forward events to the correct chat
	•	delegate chat lifecycle to ChatManager

Hub does not store chats and does not handle message persistence.

⸻

ChatManager

ChatManager manages active chats in memory.

Responsibilities:
	•	create chat instances
	•	return existing chat instances
	•	remove inactive chats

Structure:

map[chatID]*ChatRoom protected by a mutex.

Chats are created lazily when the first client joins.

Chats are removed automatically when the last client disconnects.

⸻

ChatRoom

Each chat runs as its own goroutine.

Responsibilities:
	•	manage connected clients
	•	broadcast messages
	•	handle join/leave events
	•	persist messages
	•	terminate when empty

Each ChatRoom contains:
	•	clients map
	•	register channel
	•	unregister channel
	•	broadcast channel

Lifecycle:

client join
↓
ChatRoom created (if needed)
↓
clients communicate
↓
last client leaves
↓
ChatRoom goroutine stops

⸻

Client

Client represents a single WebSocket connection.

Responsibilities:
	•	read messages from websocket
	•	send messages to the chat
	•	write outbound messages to websocket

Each client runs two goroutines:

ReadPump
WritePump

⸻

MessageStore

Handles message persistence.

Database:

SQLite (chat.db)

Responsibilities:
	•	save messages
	•	load recent messages
	•	manage users
	•	manage sessions

⸻

Database Schema

users

id
username
created_at

sessions

token
user_id
created_at

chats

id
type
created_at

chat_members

chat_id
user_id
joined_at

messages

id
chat_id
user_id
text
created_at

⸻

Connection Flow

Client login

↓
POST /login
↓
session token created

WebSocket connection

↓
ws://host/ws?token=SESSION_TOKEN

Server validates session.

User identity is attached to the Client object.

⸻

Join Flow

client connect
↓
client sends join
↓
Hub routes to ChatManager
↓
ChatManager returns ChatRoom
↓
client registered in chat
↓
chat history loaded
↓
realtime messaging begins

⸻

Message Flow

client message
↓
Client.ReadPump
↓
Hub
↓
ChatRoom.broadcast
↓
MessageStore.SaveMessage
↓
broadcast to clients

⸻

Presence Events

Realtime events emitted:

user_join
user_leave

These events are not stored in the database.

⸻

Chat Lifecycle

Chats exist only while clients are connected.

first client joins
↓
chat created
↓
clients communicate
↓
last client leaves
↓
chat destroyed

This prevents memory leaks and idle goroutines.

⸻

System Goals

The architecture is designed to support:
	•	realtime messaging
	•	chat rooms
	•	private chats
	•	persistent history
	•	authenticated users
	•	horizontal scalability in the future