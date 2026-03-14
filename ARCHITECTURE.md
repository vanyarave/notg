# Messenger Architecture

Private realtime messenger backend.

Stack

- Go
- WebSocket
- SQLite

Server Architecture

The system uses a hub-based architecture for realtime communication.

Components

Hub  
Central event router.

Responsibilities:
- manages chat rooms
- routes messages between clients
- registers and unregisters users
- broadcasts presence events

Client  
Represents a websocket connection.

Responsibilities:
- read messages from websocket
- send messages to hub
- write outbound messages to websocket

ChatRoom  
Represents a chat room.

Responsibilities:
- holds connected clients
- broadcasts messages to room members
- handles join/leave events

MessageStore  
Handles message persistence.

Uses SQLite database `chat.db`.

Responsibilities:
- save messages
- load recent messages
- provide message history

Connection Flow

client
↓
WebSocket connection
↓
ServeWs
↓
Client
↓
Hub
↓
Room

Message Flow

client message
↓
Client.ReadPump
↓
Hub.Broadcast
↓
ChatRoom.deliver
↓
other clients

Persistence Flow

message
↓
Hub
↓
MessageStore.SaveMessage
↓
SQLite

Join Protocol

First message from client must be:

{
"type": "join",
"room": "room_name",
"user": "username"
}

Only after join the client is registered in the room.

Presence Events

The server emits realtime events:

user_join  
user_leave

These are not stored in the database.

Stored Message Format

{
"type": "chat",
"room": "family",
"user": "alex",
"text": "hello"
}